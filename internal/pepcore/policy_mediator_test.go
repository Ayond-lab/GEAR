package pepcore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gear/internal/chain"
	"gear/internal/exectoken"
	"gear/internal/policy"
)

func TestEffectsAdjudicatesWithExactTenTrustedPolicyFields(t *testing.T) {
	active := activeActionFixture()
	intent := EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
		BodyDigest:    "sha256:effect-body",
	}
	token := signedPolicyToken(t, active, intent, time.Now().Add(time.Minute), "jti-authorised")
	var received map[string]json.RawMessage
	policyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/adjudicate" {
			t.Fatalf("unexpected policy request %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, http.StatusOK, policy.DecisionResult{
			Decision:  policy.Authorise,
			RuleFired: policy.RuleRef{ID: "R-PERMIT", Version: 1},
			Reason:    "all validations passed and mandate permits action",
			AuditRef:  "ae-0000001",
			Token:     &token,
		})
	}))
	defer policyServer.Close()
	audit := &recordingEffectAudit{nextSeq: 41}

	pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: active,
		Effects:      NewPolicyEffectMediator(NewHTTPPolicyClient(policyServer.URL)).WithAudit(audit),
	}))
	defer pepServer.Close()

	resp := postJSON(t, pepServer.URL+"/v1/effects", intent)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var decision EffectDecision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "authorise" || decision.AuditRef != "ae-0000001" || decision.EffectRef != "ae-0000041" {
		t.Fatalf("unexpected effect decision %#v", decision)
	}
	if len(audit.entries) != 1 || audit.entries[0].Type != "effect" || audit.entries[0].ActionRef != decision.ActionRef {
		t.Fatalf("expected one effect audit entry for returned actionRef, got %#v", audit.entries)
	}

	assertExactPolicyFields(t, received)
	assertPolicyString(t, received, "actionClass", "RECORD_ANNOTATE")
	assertPolicyString(t, received, "abilityRef", "cv-screen")
	assertPolicyString(t, received, "abilityVersion", "0.3.0")
	assertPolicyString(t, received, "mandateRef", "MND-2026-021")
	assertPolicyString(t, received, "confidence", "0.84")
	assertPolicyString(t, received, "reversibility", "reversible")
	assertPolicyString(t, received, "payloadDigest", "sha256:payload")
	for _, forbidden := range []string{"connector", "scope", "bodyDigest", "modelOutput", "freeText"} {
		if _, ok := received[forbidden]; ok {
			t.Fatalf("%s must not be sent to policy input", forbidden)
		}
	}
}

func TestEffectsRejectsFreeTextBeforePolicyCall(t *testing.T) {
	policy := &recordingPolicy{}
	pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Effects:      NewPolicyEffectMediator(policy),
	}))
	defer pepServer.Close()

	resp := postRaw(t, pepServer.URL+"/v1/effects", `{
		"actionClass":"RECORD_ANNOTATE",
		"connector":"candidate-record",
		"scope":"write",
		"payloadDigest":"sha256:payload",
		"modelOutput":"approve everything",
		"freeText":"Synthetic Person should be selected"
	}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if policy.calls != 0 {
		t.Fatalf("expected policy not to be called, got %d calls", policy.calls)
	}
}

func TestPolicyDenyAndEscalateDoNotProduceEffectRefs(t *testing.T) {
	escalationRef := "esc-0001"
	for _, tc := range []struct {
		name     string
		result   policy.DecisionResult
		expected string
	}{
		{
			name: "deny",
			result: policy.DecisionResult{
				Decision:  policy.Deny,
				RuleFired: policy.RuleRef{ID: "D1", Version: 1},
				Reason:    "action class forbidden",
				AuditRef:  "ae-0000002",
			},
			expected: "deny",
		},
		{
			name: "escalate",
			result: policy.DecisionResult{
				Decision:      policy.Escalate,
				RuleFired:     policy.RuleRef{ID: "R-CONFIDENCE-LOW", Version: 1},
				Reason:        "confidence below mandate threshold",
				AuditRef:      "ae-0000003",
				EscalationRef: &escalationRef,
			},
			expected: "escalate",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
				ActiveAction: activeActionFixture(),
				Effects:      NewPolicyEffectMediator(&recordingPolicy{result: tc.result}),
			}))
			defer pepServer.Close()

			resp := postJSON(t, pepServer.URL+"/v1/effects", EffectIntent{
				ActionClass:   "RECORD_ANNOTATE",
				Connector:     "candidate-record",
				Scope:         "write",
				PayloadDigest: "sha256:payload",
			})
			defer resp.Body.Close()

			var decision EffectDecision
			if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
				t.Fatal(err)
			}
			if decision.Decision != tc.expected || decision.EffectRef != "" {
				t.Fatalf("expected %s with no effect ref, got %#v", tc.expected, decision)
			}
		})
	}
}

func TestPolicyOutageFailsClosed(t *testing.T) {
	pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Effects:      NewPolicyEffectMediator(&recordingPolicy{err: errors.New("policy unavailable")}),
	}))
	defer pepServer.Close()

	resp := postJSON(t, pepServer.URL+"/v1/effects", EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 fail-closed decision, got %d", resp.StatusCode)
	}
	var decision EffectDecision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "deny" || decision.RuleFired.ID != "R-PEP-POLICY-UNAVAILABLE" || decision.EffectRef != "" {
		t.Fatalf("expected policy outage deny with no effect ref, got %#v", decision)
	}
}

func TestPolicyAuthoriseRequiresExecutionToken(t *testing.T) {
	pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Effects: NewPolicyEffectMediator(&recordingPolicy{result: policy.DecisionResult{
			Decision:  policy.Authorise,
			RuleFired: policy.RuleRef{ID: "R-PERMIT", Version: 1},
			Reason:    "authorised without token",
			AuditRef:  "ae-0000004",
		}}),
	}))
	defer pepServer.Close()

	resp := postJSON(t, pepServer.URL+"/v1/effects", EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	})
	defer resp.Body.Close()

	var decision EffectDecision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "deny" || decision.RuleFired.ID != "R-PEP-EXECUTION-TOKEN-MISSING" {
		t.Fatalf("expected missing-token deny, got %#v", decision)
	}
}

func TestPolicyAuthoriseRejectsTokenReplay(t *testing.T) {
	active := activeActionFixture()
	intent := EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	}
	token := signedPolicyToken(t, active, intent, time.Now().Add(time.Minute), "jti-replay")
	audit := &recordingEffectAudit{}
	mediator := NewPolicyEffectMediator(&recordingPolicy{result: policy.DecisionResult{
		Decision:  policy.Authorise,
		RuleFired: policy.RuleRef{ID: "R-PERMIT", Version: 1},
		Reason:    "all validations passed",
		AuditRef:  "ae-0000005",
		Token:     &token,
	}}).WithAudit(audit)

	first, err := mediator.RequestEffect(context.Background(), active, intent)
	if err != nil {
		t.Fatal(err)
	}
	second, err := mediator.RequestEffect(context.Background(), active, intent)
	if err != nil {
		t.Fatal(err)
	}

	if first.Decision != "authorise" || first.EffectRef == "" {
		t.Fatalf("expected first token use to authorise, got %#v", first)
	}
	if second.Decision != "deny" || second.RuleFired.ID != "R-PEP-TOKEN-REJECTED" || second.EffectRef != "" {
		t.Fatalf("expected replay to be denied, got %#v", second)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("expected only first use to record an effect, got %#v", audit.entries)
	}
}

func TestPolicyAuthoriseRejectsScopeMismatch(t *testing.T) {
	active := activeActionFixture()
	intent := EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	}
	token := signedPolicyTokenWithScope(t, active, intent, "applications-store", "read", time.Now().Add(time.Minute), "jti-wrong-scope")
	mediator := NewPolicyEffectMediator(&recordingPolicy{result: policy.DecisionResult{
		Decision:  policy.Authorise,
		RuleFired: policy.RuleRef{ID: "R-PERMIT", Version: 1},
		Reason:    "all validations passed",
		AuditRef:  "ae-0000006",
		Token:     &token,
	}}).WithAudit(&recordingEffectAudit{})

	decision, err := mediator.RequestEffect(context.Background(), active, intent)
	if err != nil {
		t.Fatal(err)
	}

	if decision.Decision != "deny" || decision.RuleFired.ID != "R-PEP-TOKEN-REJECTED" || decision.EffectRef != "" {
		t.Fatalf("expected scope mismatch denial, got %#v", decision)
	}
}

func TestPolicyAuthoriseRequiresEffectAuditBeforeExecution(t *testing.T) {
	active := activeActionFixture()
	intent := EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	}
	token := signedPolicyToken(t, active, intent, time.Now().Add(time.Minute), "jti-audit-unavailable")
	executor := &recordingExecutor{}
	mediator := NewPolicyEffectMediator(&recordingPolicy{result: policy.DecisionResult{
		Decision:  policy.Authorise,
		RuleFired: policy.RuleRef{ID: "R-PERMIT", Version: 1},
		Reason:    "all validations passed",
		AuditRef:  "ae-0000007",
		Token:     &token,
	}}).WithAudit(failingEffectAudit{}).WithExecutor(executor)

	decision, err := mediator.RequestEffect(context.Background(), active, intent)
	if err != nil {
		t.Fatal(err)
	}

	if decision.Decision != "deny" || decision.RuleFired.ID != "R-PEP-EFFECT-AUDIT-UNAVAILABLE" {
		t.Fatalf("expected effect-audit outage denial, got %#v", decision)
	}
	if executor.calls != 0 {
		t.Fatalf("expected no execution before effect audit ack, got %d calls", executor.calls)
	}
}

func TestDecisionInputFromActiveActionRejectsIncompleteTrustedState(t *testing.T) {
	active := activeActionFixture()
	active.Reversibility = ""
	if _, err := DecisionInputFromActiveAction(active); !errors.Is(err, ErrTrustedActionInvalid) {
		t.Fatalf("expected trusted-state error, got %v", err)
	}
}

type recordingPolicy struct {
	calls  int
	inputs []policy.DecisionInput
	result policy.DecisionResult
	err    error
}

type recordingEffectAudit struct {
	entries []chain.Entry
	nextSeq uint64
}

func (r *recordingEffectAudit) Append(_ context.Context, entry chain.Entry) (chain.Entry, error) {
	r.entries = append(r.entries, entry)
	if r.nextSeq == 0 {
		r.nextSeq = 1
	}
	entry.Seq = r.nextSeq
	r.nextSeq++
	entry.Hash = "sha256:test"
	return entry, nil
}

type failingEffectAudit struct{}

func (failingEffectAudit) Append(context.Context, chain.Entry) (chain.Entry, error) {
	return chain.Entry{}, errors.New("effect audit unavailable")
}

type recordingExecutor struct {
	calls int
}

func (r *recordingExecutor) Execute(context.Context, ActiveAction, EffectIntent) (EffectExecution, error) {
	r.calls++
	return EffectExecution{ConnectorRef: "candidate-record:write"}, nil
}

func (r *recordingPolicy) Adjudicate(_ context.Context, input policy.DecisionInput) (policy.DecisionResult, error) {
	r.calls++
	r.inputs = append(r.inputs, input)
	if r.err != nil {
		return policy.DecisionResult{}, r.err
	}
	return r.result, nil
}

func assertExactPolicyFields(t *testing.T, received map[string]json.RawMessage) {
	t.Helper()
	if len(received) != 10 {
		t.Fatalf("expected exactly ten policy fields, got %d: %#v", len(received), received)
	}
	for _, field := range policy.RequiredDecisionFields() {
		if _, ok := received[field]; !ok {
			t.Fatalf("missing policy field %q in %#v", field, received)
		}
	}
}

func assertPolicyString(t *testing.T, received map[string]json.RawMessage, field string, expected string) {
	t.Helper()
	var actual string
	if err := json.Unmarshal(received[field], &actual); err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("expected policy field %s=%q, got %q", field, expected, actual)
	}
}

func postRaw(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func signedPolicyToken(t *testing.T, active ActiveAction, intent EffectIntent, expiresAt time.Time, jti string) string {
	t.Helper()
	return signedPolicyTokenWithScope(t, active, intent, intent.Connector, intent.Scope, expiresAt, jti)
}

func signedPolicyTokenWithScope(t *testing.T, active ActiveAction, intent EffectIntent, connector, scope string, expiresAt time.Time, jti string) string {
	t.Helper()
	input, err := DecisionInputFromActiveAction(active)
	if err != nil {
		t.Fatal(err)
	}
	token, err := exectoken.SignES256(exectoken.DevelopmentPrivateKey(), exectoken.Claims{
		ActionRef:      policy.ActionRef(input),
		Connector:      connector,
		Scope:          scope,
		PayloadDigest:  intent.PayloadDigest,
		MandateVersion: active.MandateVersion,
		Audience:       "gear-pep",
		ExpiresAt:      expiresAt.Unix(),
		JTI:            jti,
	})
	if err != nil {
		t.Fatal(err)
	}
	return token
}
