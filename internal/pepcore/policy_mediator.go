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

	"gear/internal/policy"
)

var ErrTrustedActionInvalid = fmt.Errorf("%w: trusted active action is invalid", ErrRequestRejected)

type PolicyAdjudicator interface {
	Adjudicate(ctx context.Context, input policy.DecisionInput) (policy.DecisionResult, error)
}

type PolicyEffectMediator struct {
	Policy PolicyAdjudicator
}

func NewPolicyEffectMediator(policy PolicyAdjudicator) PolicyEffectMediator {
	return PolicyEffectMediator{Policy: policy}
}

func (m PolicyEffectMediator) RequestEffect(ctx context.Context, active ActiveAction, _ EffectIntent) (EffectDecision, error) {
	input, err := DecisionInputFromActiveAction(active)
	if err != nil {
		return EffectDecision{
			ActionRef: active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-TRUSTED-STATE-INVALID", Version: 1},
			Reason:    "trusted active action state is incomplete; fail closed",
		}, nil
	}
	if m.Policy == nil {
		return EffectDecision{
			ActionRef: active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-UNAVAILABLE", Version: 1},
			Reason:    "policy adjudicator unavailable; fail closed",
		}, nil
	}

	result, err := m.Policy.Adjudicate(ctx, input)
	if err != nil {
		return EffectDecision{
			ActionRef: active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-UNAVAILABLE", Version: 1},
			Reason:    "policy adjudicator unavailable; fail closed",
		}, nil
	}

	decision := EffectDecision{
		ActionRef: active.ActionRef,
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
				ActionRef: active.ActionRef,
				Decision:  "deny",
				RuleFired: Rule{ID: "R-PEP-EXECUTION-TOKEN-MISSING", Version: 1},
				Reason:    "policy authorised without execution token; fail closed",
				AuditRef:  result.AuditRef,
			}, nil
		}
		return decision, nil
	default:
		return EffectDecision{
			ActionRef: active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-POLICY-DECISION-INVALID", Version: 1},
			Reason:    "policy returned an unknown decision; fail closed",
			AuditRef:  result.AuditRef,
		}, nil
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
