package fixturestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gear/internal/cvdemo"
)

func TestRejectsNonSyntheticNamespace(t *testing.T) {
	_, err := New("production-hr", cvdemo.GenerateApplications())
	if !errors.Is(err, ErrNonSyntheticNamespace) {
		t.Fatalf("expected non-synthetic namespace rejection, got %v", err)
	}
}

func TestListGetAndEraseApplication(t *testing.T) {
	store := newTestStore(t)
	applications := store.ListApplications()
	if len(applications) != 60 {
		t.Fatalf("expected 60 applications, got %d", len(applications))
	}
	if applications[0].ApplicationText != "" {
		t.Fatal("list endpoint view must not include application text")
	}

	application, ok := store.GetApplication("SYN-CV-0001")
	if !ok || application.ApplicationText == "" || application.PayloadDigest == "" {
		t.Fatalf("expected retrievable application content, got %#v", application)
	}
	erased, ok := store.EraseApplication("SYN-CV-0001")
	if !ok || !erased.Erased || erased.ApplicationText != "" {
		t.Fatalf("expected erased content view, got %#v", erased)
	}
	after, ok := store.GetApplication("SYN-CV-0001")
	if !ok || !after.Erased || after.ApplicationText != "" || after.PayloadDigest == "" || after.SubjectRef == "" {
		t.Fatalf("erasure must remove content while retaining refs, got %#v", after)
	}
}

func TestStoresExtractionsAndReasonsOutsideAudit(t *testing.T) {
	store := newTestStore(t)
	ctx := WithNow(context.Background(), func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) })
	extraction, err := store.StoreExtraction(ctx, StoreExtractionRequest{
		SourceRef:     cvdemo.SourceRef("SYN-CV-0001"),
		PayloadDigest: "sha256:payload",
		Fields:        map[string]string{"workAuthorisationStatus": "Holds permit"},
		Confidence:    "0.84",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(extraction.Ref, "fixture://synthetic-cv-lab/extractions/") || extraction.CreatedAt == "" {
		t.Fatalf("unexpected extraction record %#v", extraction)
	}
	reason, err := store.StoreReason(ctx, "Human reviewer asked for more evidence")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reason.Ref, "fixture://synthetic-cv-lab/reasons/") || reason.Reason == "" {
		t.Fatalf("unexpected reason record %#v", reason)
	}
}

func TestHTTPHandler(t *testing.T) {
	server := httptest.NewServer(NewHandler(newTestStore(t)))
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/applications")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected list 200, got %d", resp.StatusCode)
	}
	var list ListApplicationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Namespace != cvdemo.SyntheticNamespace || len(list.Applications) != 60 {
		t.Fatalf("unexpected list response %#v", list)
	}

	body := []byte(`{"sourceRef":"fixture://synthetic-cv-lab/applications/SYN-CV-0001","payloadDigest":"sha256:payload","fields":{"workAuthorisationStatus":"EEA national"},"confidence":"0.84"}`)
	resp, err = http.Post(server.URL+"/v1/extractions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected extraction 201, got %d", resp.StatusCode)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(cvdemo.SyntheticNamespace, cvdemo.GenerateApplications())
	if err != nil {
		t.Fatal(err)
	}
	return store
}
