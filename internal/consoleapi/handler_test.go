package consoleapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gear/internal/auditprivacy"
)

func TestMandateViewShowsRefusalAndNarrowedMandate(t *testing.T) {
	server := httptest.NewServer(NewHandler(testConfig(t)))
	defer server.Close()

	var view MandateView
	getJSON(t, server.URL+"/api/mandate", &view)
	if view.Refusal == nil || view.Refusal.Criterion != "citizenship" || view.Refusal.Verb != "select" {
		t.Fatalf("expected citizenship/select refusal, got %#v", view.Refusal)
	}
	if view.NarrowedMandate == nil || len(view.ActionGrants) != 4 || view.ActionGrants[2].Class != "CANDIDATE_RANK" || view.ActionGrants[2].Disposition != "forbid" {
		t.Fatalf("expected narrowed mandate with rank forbidden, got %#v", view)
	}
	if len(view.PolicyFields) != 10 || len(view.HiddenInputs) == 0 {
		t.Fatalf("expected policy boundary metadata, got fields=%#v hidden=%#v", view.PolicyFields, view.HiddenInputs)
	}
	data, _ := json.Marshal(view.RefusalAuditEntries)
	if findings := auditprivacy.Scan(string(data)); len(findings) != 0 {
		t.Fatalf("refusal audit entries must be privacy-safe, got %#v", findings)
	}
}

func TestEscalationDecisionFlowUsesReasonRefs(t *testing.T) {
	server := httptest.NewServer(NewHandler(testConfig(t)))
	defer server.Close()

	var queue EscalationView
	getJSON(t, server.URL+"/api/escalations", &queue)
	if len(queue.Items) != 3 || queue.Summary.PendingEscalations != 3 {
		t.Fatalf("expected 3 pending escalations, got %#v", queue)
	}

	badResp := postJSON(t, server.URL+"/api/escalations/decision", []byte(`{"actionRef":"`+queue.Items[0].ActionRef+`","decision":"approve","decidedBy":"hiring-manager-1","reasonRef":"fixture://synthetic-cv-lab/reasons/test","freeText":"not allowed"}`))
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected extra free text to be rejected, got %d", badResp.StatusCode)
	}
	badResp.Body.Close()

	goodResp := postJSON(t, server.URL+"/api/escalations/decision", []byte(`{"actionRef":"`+queue.Items[0].ActionRef+`","decision":"approve","decidedBy":"hiring-manager-1","reasonRef":"fixture://synthetic-cv-lab/reasons/abc"}`))
	if goodResp.StatusCode != http.StatusOK {
		t.Fatalf("expected valid decision 200, got %d", goodResp.StatusCode)
	}
	goodResp.Body.Close()

	getJSON(t, server.URL+"/api/escalations", &queue)
	if queue.Summary.PendingEscalations != 2 || queue.Items[0].Status != "approve" || queue.Items[0].ReasonRef == "" {
		t.Fatalf("expected one approved escalation, got %#v", queue)
	}
}

func TestAuditAndPrivacyViewsAreSafe(t *testing.T) {
	server := httptest.NewServer(NewHandler(testConfig(t)))
	defer server.Close()

	var audit AuditView
	getJSON(t, server.URL+"/api/audit", &audit)
	if !audit.Verification.OK || len(audit.EffectsWithoutDecision) != 0 || len(audit.PrivacyFindings) != 0 {
		t.Fatalf("unexpected audit view %#v", audit)
	}
	data, _ := json.Marshal(audit)
	if findings := auditprivacy.Scan(string(data)); len(findings) != 0 {
		t.Fatalf("audit API response must be privacy-safe, got %#v", findings)
	}

	var scan PrivacyScanView
	getJSON(t, server.URL+"/api/privacy-scan", &scan)
	if !scan.OK || len(scan.Scanned) != 2 {
		t.Fatalf("unexpected privacy scan view %#v", scan)
	}
}

func TestLatencyAndEvidenceEndpoints(t *testing.T) {
	config := testConfig(t)
	write(t, filepath.Join(config.EvidenceRoot, "A8", "2026-08-29T12:00:00Z", "RESULT.md"), "Verdict: PASS\n")
	server := httptest.NewServer(NewHandler(config))
	defer server.Close()

	var latencyBody struct {
		Trials    int   `json:"trials"`
		P95Micros int64 `json:"p95Micros"`
	}
	getJSON(t, server.URL+"/api/latency", &latencyBody)
	if latencyBody.Trials != 20 || latencyBody.P95Micros < 0 {
		t.Fatalf("unexpected latency body %#v", latencyBody)
	}

	var evidence struct {
		Criteria []struct {
			ID      string `json:"id"`
			Verdict string `json:"verdict"`
		} `json:"criteria"`
	}
	getJSON(t, server.URL+"/api/evidence", &evidence)
	if len(evidence.Criteria) != 10 || evidence.Criteria[7].ID != "A8" || evidence.Criteria[7].Verdict != "PASS" {
		t.Fatalf("unexpected evidence status %#v", evidence)
	}
}

func TestServesStaticConsole(t *testing.T) {
	config := testConfig(t)
	server := httptest.NewServer(NewHandler(config))
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected static index 200, got %d", resp.StatusCode)
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	consoleDir := t.TempDir()
	write(t, filepath.Join(consoleDir, "index.html"), "<html><body>GEAR</body></html>")
	return Config{
		ConsoleDir:     consoleDir,
		EvidenceRoot:   t.TempDir(),
		LatencyTrials:  20,
		LatencyWorkers: 1,
		Now:            func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) },
	}
}

func getJSON(t *testing.T, url string, into any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func postJSON(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func write(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
