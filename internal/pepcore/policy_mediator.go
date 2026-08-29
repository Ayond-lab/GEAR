package pepcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gear/internal/chain"
	"gear/internal/policy"
)

var ErrTrustedActionInvalid = fmt.Errorf("%w: trusted active action is invalid", ErrRequestRejected)

type PolicyAdjudicator interface {
	Adjudicate(ctx context.Context, input policy.DecisionInput) (policy.DecisionResult, error)
}

type EffectAuditAppender interface {
	Append(ctx context.Context, entry chain.Entry) (chain.Entry, error)
}

type EffectExecutor interface {
	Execute(ctx context.Context, active ActiveAction, intent EffectIntent) (EffectExecution, error)
}

type EffectExecution struct {
	ConnectorRef string
}

type SyntheticEffectExecutor struct{}

func (SyntheticEffectExecutor) Execute(_ context.Context, _ ActiveAction, intent EffectIntent) (EffectExecution, error) {
	return EffectExecution{ConnectorRef: intent.Connector + ":" + intent.Scope}, nil
}

type PolicyEffectMediator struct {
	Policy        PolicyAdjudicator
	TokenVerifier *TokenVerifier
	Audit         EffectAuditAppender
	Executor      EffectExecutor
	AllowedScopes map[string]bool
}

func NewPolicyEffectMediator(policy PolicyAdjudicator) PolicyEffectMediator {
	return PolicyEffectMediator{
		Policy:        policy,
		TokenVerifier: NewTokenVerifier(),
		Executor:      SyntheticEffectExecutor{},
		AllowedScopes: DefaultAllowedScopes(),
	}
}

func (m PolicyEffectMediator) WithAudit(audit EffectAuditAppender) PolicyEffectMediator {
	m.Audit = audit
	return m
}

func (m PolicyEffectMediator) WithAllowedScopes(scopes map[string]bool) PolicyEffectMediator {
	m.AllowedScopes = scopes
	return m
}

func (m PolicyEffectMediator) WithTokenVerifier(verifier *TokenVerifier) PolicyEffectMediator {
	m.TokenVerifier = verifier
	return m
}

func (m PolicyEffectMediator) WithExecutor(executor EffectExecutor) PolicyEffectMediator {
	m.Executor = executor
	return m
}

func (m PolicyEffectMediator) RequestEffect(ctx context.Context, active ActiveAction, intent EffectIntent) (EffectDecision, error) {
	input, err := DecisionInputFromActiveAction(active)
	if err != nil {
		return EffectDecision{
			ActionRef: active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-TRUSTED-STATE-INVALID", Version: 1},
			Reason:    "trusted active action state is incomplete; fail closed",
		}, nil
	}
	actionRef := policy.ActionRef(input)
	if m.Policy == nil {
		return EffectDecision{
			ActionRef: actionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-UNAVAILABLE", Version: 1},
			Reason:    "policy adjudicator unavailable; fail closed",
		}, nil
	}

	result, err := m.Policy.Adjudicate(ctx, input)
	if err != nil {
		return EffectDecision{
			ActionRef: actionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-UNAVAILABLE", Version: 1},
			Reason:    "policy adjudicator unavailable; fail closed",
		}, nil
	}

	decision := EffectDecision{
		ActionRef: actionRef,
		Decision:  string(result.Decision),
		RuleFired: Rule{ID: result.RuleFired.ID, Version: result.RuleFired.Version},
		Reason:    result.Reason,
		AuditRef:  result.AuditRef,
	}
	if result.EscalationRef != nil {
		decision.EscalationRef = *result.EscalationRef
	}

	switch result.Decision {
	case policy.Deny, policy.Escalate:
		return decision, nil
	case policy.Authorise:
		if result.Token == nil || *result.Token == "" {
			return EffectDecision{
				ActionRef: actionRef,
				Decision:  "deny",
				RuleFired: Rule{ID: "R-PEP-EXECUTION-TOKEN-MISSING", Version: 1},
				Reason:    "policy authorised without execution token; fail closed",
				AuditRef:  result.AuditRef,
			}, nil
		}
		if m.TokenVerifier == nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-TOKEN-VERIFIER-UNAVAILABLE", "execution token verifier unavailable; fail closed")
		}
		allowedScopes := m.AllowedScopes
		if allowedScopes == nil {
			allowedScopes = map[string]bool{}
		}
		request := EffectRequest{
			ActionRef:      actionRef,
			Connector:      intent.Connector,
			Scope:          intent.Scope,
			PayloadDigest:  intent.PayloadDigest,
			MandateVersion: active.MandateVersion,
		}
		if err := m.TokenVerifier.VerifyJWS(*result.Token, request, allowedScopes); err != nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-TOKEN-REJECTED", "execution token rejected; fail closed")
		}
		if err := validateFinalEffectAuthority(active, intent, allowedScopes); err != nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-AUTHORITY-RECHECK", "final effect authority re-check failed; fail closed")
		}
		if m.Executor == nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-EFFECT-EXECUTOR-UNAVAILABLE", "effect executor unavailable; fail closed")
		}
		if m.Audit == nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-EFFECT-AUDIT-UNAVAILABLE", "effect audit unavailable; fail closed")
		}
		stored, err := m.Audit.Append(ctx, effectAuditEntry(input, actionRef, result, intent))
		if err != nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-EFFECT-AUDIT-UNAVAILABLE", "effect audit unavailable; fail closed")
		}
		if _, err := m.Executor.Execute(ctx, active, intent); err != nil {
			return denyAfterPolicy(actionRef, result.AuditRef, "R-PEP-EFFECT-EXECUTION-FAILED", "effect execution failed; fail closed")
		}
		decision.EffectRef = chain.Ref(stored.Seq)
		return decision, nil
	default:
		return EffectDecision{
			ActionRef: actionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-DECISION-INVALID", Version: 1},
			Reason:    "policy returned an unknown decision; fail closed",
			AuditRef:  result.AuditRef,
		}, nil
	}
}

func denyAfterPolicy(actionRef, auditRef, rule, reason string) (EffectDecision, error) {
	return EffectDecision{
		ActionRef: actionRef,
		Decision:  "deny",
		RuleFired: Rule{ID: rule, Version: 1},
		Reason:    reason,
		AuditRef:  auditRef,
	}, nil
}

func validateFinalEffectAuthority(active ActiveAction, intent EffectIntent, allowedScopes map[string]bool) error {
	if intent.ActionClass != active.ActionClass {
		return fmt.Errorf("%w: actionClass mismatch", ErrRequestRejected)
	}
	if intent.PayloadDigest != active.PayloadDigest {
		return fmt.Errorf("%w: payloadDigest mismatch", ErrRequestRejected)
	}
	if !allowedScopes[intent.Connector+":"+intent.Scope] {
		return fmt.Errorf("%w: unsupported connector scope", ErrRequestRejected)
	}
	return nil
}

func effectAuditEntry(input policy.DecisionInput, actionRef string, result policy.DecisionResult, intent EffectIntent) chain.Entry {
	dataAccessed := []string{intent.PayloadDigest}
	if intent.BodyDigest != "" {
		dataAccessed = append(dataAccessed, intent.BodyDigest)
	}
	return chain.Entry{
		TS:           time.Now().UTC().Format(time.RFC3339Nano),
		Type:         "effect",
		ActionRef:    actionRef,
		Actor:        "gear-pep",
		Mandate:      fmt.Sprintf("%s:%d", input.MandateRef, input.MandateVersion),
		Rule:         fmt.Sprintf("%s:%d", result.RuleFired.ID, result.RuleFired.Version),
		Decision:     string(result.Decision),
		InputsDigest: policy.InputDigest(input),
		Model:        "none",
		DataAccessed: dataAccessed,
	}
}

func DecisionInputFromActiveAction(active ActiveAction) (policy.DecisionInput, error) {
	if !active.Available() || active.AbilityRef == "" || active.AbilityVersion == "" || active.MandateRef == "" || active.MandateVersion <= 0 || active.Confidence == "" || active.Reversibility == "" || len(active.DataClasses) == 0 {
		return policy.DecisionInput{}, ErrTrustedActionInvalid
	}
	if !isSHA256Ref(active.PayloadDigest) {
		return policy.DecisionInput{}, ErrTrustedActionInvalid
	}
	counters := active.Counters
	if counters == nil {
		counters = map[string]int{}
	}
	return policy.DecisionInput{
		ActionClass:    active.ActionClass,
		AbilityRef:     active.AbilityRef,
		AbilityVersion: active.AbilityVersion,
		MandateRef:     active.MandateRef,
		MandateVersion: active.MandateVersion,
		Confidence:     active.Confidence,
		DataClasses:    append([]string(nil), active.DataClasses...),
		Reversibility:  active.Reversibility,
		Counters:       cloneCounters(counters),
		PayloadDigest:  active.PayloadDigest,
	}, nil
}

func cloneCounters(counters map[string]int) map[string]int {
	clone := make(map[string]int, len(counters))
	for key, value := range counters {
		clone[key] = value
	}
	return clone
}

func DefaultAllowedScopes() map[string]bool {
	return map[string]bool{"candidate-record:write": true}
}

func ParseAllowedScopes(value string) map[string]bool {
	scopes := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		scopes[item] = true
	}
	return scopes
}

type HTTPPolicyClient struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPPolicyClient(baseURL string) *HTTPPolicyClient {
	return &HTTPPolicyClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *HTTPPolicyClient) Adjudicate(ctx context.Context, input policy.DecisionInput) (policy.DecisionResult, error) {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	body, err := json.Marshal(input)
	if err != nil {
		return policy.DecisionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/adjudicate", bytes.NewReader(body))
	if err != nil {
		return policy.DecisionResult{}, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return policy.DecisionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return policy.DecisionResult{}, fmt.Errorf("policy adjudication status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result policy.DecisionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return policy.DecisionResult{}, err
	}
	return result, nil
}
