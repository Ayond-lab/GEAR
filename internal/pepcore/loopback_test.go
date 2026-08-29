package pepcore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopbackHealthz(t *testing.T) {
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestExtractValidatesActiveActionAndCallsExtractor(t *testing.T) {
	extractor := &recordingExtractor{result: ExtractResult{
		Fields:     map[string]string{"workAuthorisationStatus": "Holds permit"},
		Confidence: "0.84",
	}}
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Extractor:    extractor,
	}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/extract", ExtractRequest{
		SourceRef:     "fixture://applications/0001",
		PayloadDigest: "sha256:payload",
		Profile:       "work-authorisation",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if extractor.calls != 1 {
		t.Fatalf("expected extractor to be called once, got %d", extractor.calls)
	}

	var body ExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ActionRef != "ga-001" || body.Fields["workAuthorisationStatus"] != "Holds permit" {
		t.Fatalf("unexpected extract response %#v", body)
	}
}

func TestExtractRejectsMismatchedPayloadDigest(t *testing.T) {
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{
		ActiveAction: activeActionFixture(),
		Extractor:    &recordingExtractor{},
	}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/extract", ExtractRequest{
		SourceRef:     "fixture://applications/0001",
		PayloadDigest: "sha256:other",
		Profile:       "work-authorisation",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestEffectsEndpointFailsClosedUntilPolicyMediatorIsConfigured(t *testing.T) {
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{ActiveAction: activeActionFixture()}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/effects", EffectIntent{
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
	var body EffectDecision
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Decision != "deny" || body.RuleFired.ID != "R-PEP-ADJUDICATION-NOT-CONFIGURED" {
		t.Fatalf("expected fail-closed deny, got %#v", body)
	}
}

func TestEffectsRejectsActionClassMismatch(t *testing.T) {
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{ActiveAction: activeActionFixture()}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/effects", EffectIntent{
		ActionClass:   "CANDIDATE_RANK",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestLoopbackHandlerFailsClosedWithoutActiveAction(t *testing.T) {
	server := httptest.NewServer(NewLoopbackHandler(LoopbackConfig{}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/effects", EffectIntent{
		ActionClass:   "RECORD_ANNOTATE",
		Connector:     "candidate-record",
		Scope:         "write",
		PayloadDigest: "sha256:payload",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without active action, got %d", resp.StatusCode)
	}
}

func TestValidateLoopbackListenAddress(t *testing.T) {
	if err := ValidateLoopbackListenAddress("127.0.0.1:9191"); err != nil {
		t.Fatalf("expected loopback listen address to pass, got %v", err)
	}
	for _, addr := range []string{":9191", "0.0.0.0:9191", "127.0.0.1:8080", "localhost:9191"} {
		if err := ValidateLoopbackListenAddress(addr); err == nil {
			t.Fatalf("expected %q to be rejected", addr)
		}
	}
}

func TestActiveActionFromEnv(t *testing.T) {
	env := map[string]string{
		"GEAR_ACTION_REF":      "ga-001",
		"GEAR_ACTION_CLASS":    "RECORD_ANNOTATE",
		"GEAR_ABILITY_REF":     "cv-screen",
		"GEAR_ABILITY_VERSION": "0.3.0",
		"GEAR_MANDATE_REF":     "MND-2026-021",
		"GEAR_MANDATE_VERSION": "2",
		"GEAR_SUBJECT_REF":     "sha256:subject",
		"GEAR_DATA_CLASSES":    "personal, protected-employment",
		"GEAR_CONFIDENCE":      "0.84",
		"GEAR_PAYLOAD_DIGEST":  "sha256:payload",
	}
	action, err := ActiveActionFromEnv(func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if !action.Available() || action.MandateVersion != 2 || len(action.DataClasses) != 2 {
		t.Fatalf("unexpected active action %#v", action)
	}
}

type recordingExtractor struct {
	calls  int
	result ExtractResult
}

func (r *recordingExtractor) Extract(context.Context, ActiveAction, ExtractRequest) (ExtractResult, error) {
	r.calls++
	return r.result, nil
}

func activeActionFixture() ActiveAction {
	return ActiveAction{
		ActionRef:      "ga-001",
		ActionClass:    "RECORD_ANNOTATE",
		AbilityRef:     "cv-screen",
		AbilityVersion: "0.3.0",
		MandateRef:     "MND-2026-021",
		MandateVersion: 2,
		SubjectRef:     "sha256:subject",
		DataClasses:    []string{"personal", "protected-employment"},
		Confidence:     "0.84",
		PayloadDigest:  "sha256:payload",
	}
}

func postJSON(t *testing.T, url string, value any) *http.Response {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
