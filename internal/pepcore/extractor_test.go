package pepcore

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPExtractorPostsOnlyExtractionRequest(t *testing.T) {
	var received map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/extract" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, http.StatusOK, ExtractResult{
			Fields:       map[string]string{"workAuthorisationStatus": "Holds permit"},
			Confidence:   "0.84",
			EvidenceRefs: []string{"fixture://synthetic-cv-lab/extractions/test"},
		})
	}))
	defer server.Close()

	result, err := NewHTTPExtractor(server.URL).Extract(context.Background(), activeActionFixture(), ExtractRequest{
		SourceRef:     "fixture://synthetic-cv-lab/applications/SYN-CV-0001",
		PayloadDigest: "sha256:payload",
		Profile:       "work-authorisation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Fields["workAuthorisationStatus"] != "Holds permit" || result.Confidence != "0.84" {
		t.Fatalf("unexpected result %#v", result)
	}
	if len(received) != 3 {
		t.Fatalf("extractor request must contain exactly request fields, got %#v", received)
	}
	for _, forbidden := range []string{"actionClass", "abilityRef", "mandateRef", "subjectRef", "freeText", "applicationText"} {
		if _, ok := received[forbidden]; ok {
			t.Fatalf("extractor request leaked %s", forbidden)
		}
	}
}

func TestHTTPExtractorUnavailableWithoutBaseURL(t *testing.T) {
	_, err := NewHTTPExtractor("").Extract(context.Background(), ActiveAction{}, ExtractRequest{})
	if err == nil {
		t.Fatal("expected empty extractor base URL to fail")
	}
}
