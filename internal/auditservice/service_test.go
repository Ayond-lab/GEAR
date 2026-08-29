package auditservice

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gear/internal/chain"
)

func TestServiceAppendsAndVerifiesEntries(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	server := httptest.NewServer(NewHandler(store))
	defer server.Close()

	for i := 0; i < 3; i++ {
		response := postEntry(t, server.URL, chain.Entry{
			Type:         "decision",
			ActionRef:    "ga-service",
			Actor:        "gear-policy",
			Mandate:      "MND-2026-021:2",
			Rule:         "R-PERMIT:1",
			Decision:     "authorise",
			InputsDigest: "sha256:inputs",
			Model:        "none",
			DataAccessed: []string{"sha256:payload"},
		})
		if response.AuditRef != chain.Ref(uint64(i+1)) {
			t.Fatalf("unexpected audit ref %#v", response)
		}
	}

	resp, err := http.Get(server.URL + "/v1/verify")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 verify response, got %d", resp.StatusCode)
	}
	var verify VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verify); err != nil {
		t.Fatal(err)
	}
	if !verify.OK || verify.EntryCount != 3 {
		t.Fatalf("expected valid 3-entry chain, got %#v", verify)
	}
}

func TestServiceReportsEffectsWithoutDecisions(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	server := httptest.NewServer(NewHandler(store))
	defer server.Close()

	postEntry(t, server.URL, chain.Entry{Type: "effect", ActionRef: "ga-orphan", Actor: "gear-pep"})
	postEntry(t, server.URL, chain.Entry{Type: "decision", ActionRef: "ga-ok", Actor: "gear-policy"})
	postEntry(t, server.URL, chain.Entry{Type: "effect", ActionRef: "ga-ok", Actor: "gear-pep"})

	resp, err := http.Get(server.URL + "/v1/reconcile/effects-without-decisions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var reconciliation ReconciliationResponse
	if err := json.NewDecoder(resp.Body).Decode(&reconciliation); err != nil {
		t.Fatal(err)
	}
	if len(reconciliation.EffectsWithoutDecisions) != 1 || reconciliation.EffectsWithoutDecisions[0] != "ga-orphan" {
		t.Fatalf("expected ga-orphan reconciliation finding, got %#v", reconciliation)
	}
}

func openTestStore(t *testing.T) *chain.BoltStore {
	t.Helper()
	store, err := chain.OpenBoltStore(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func postEntry(t *testing.T, baseURL string, entry chain.Entry) AppendResponse {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(baseURL+"/v1/entries", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 append response, got %d", resp.StatusCode)
	}

	var response AppendResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}
