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

	"gear/internal/policy"
)

func TestEffectsAdjudicatesWithExactTenTrustedPolicyFields(t *testing.T) {
	token := "dev-token.authorised"
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

	pepServer := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Effects:      NewPolicyEffectMediator(NewHTTPPolicyClient(policyServer.URL)),
	}))
	defer pepServer.Close()

	resp := postJSON(t, pepServer.URL+"/v1/effects", EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
		BodyDigest:    "sha256:effect-body",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var decision EffectDecision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "authorise" || decision.AuditRef != "ae-0000001" || decision.EffectRef != "" {
		t.Fatalf("unexpected effect decision %#v", decision)
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
