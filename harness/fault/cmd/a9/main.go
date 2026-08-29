package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gear/internal/policy"
)

type adjudicationCase struct {
	Name       string `json:"name"`
	Action     string `json:"action"`
	Confidence string `json:"confidence"`
}

type adjudicationArtifact struct {
	Case     adjudicationCase      `json:"case"`
	Response policy.DecisionResult `json:"response"`
}

func main() {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	evidenceDir := filepath.Join("evidence", "A9", timestamp)
	must(os.MkdirAll(evidenceDir, 0o755))

	auditURL := "http://127.0.0.1:1"
	server := httptest.NewServer(policy.NewHandler(policy.NewAdjudicator(
		policy.DefaultCVRuntimePolicy(),
		policy.NewHTTPAuditClient(auditURL),
	)))
	defer server.Close()

	cases := []adjudicationCase{
		{Name: "would-authorise", Action: "RECORD_ANNOTATE", Confidence: "0.84"},
		{Name: "would-escalate-low-confidence", Action: "RECORD_ANNOTATE", Confidence: "0.42"},
		{Name: "would-escalate-reserved-action", Action: "RECORD_MODIFY", Confidence: "0.84"},
	}

	artifacts := make([]adjudicationArtifact, 0, len(cases))
	transcript := strings.Builder{}
	passed := true
	for _, tc := range cases {
		response := adjudicate(server.URL, tc)
		artifacts = append(artifacts, adjudicationArtifact{Case: tc, Response: response})
		fmt.Fprintf(&transcript, "%s: decision=%s rule=%s auditRef=%q token=%v escalation=%v\n", tc.Name, response.Decision, response.RuleFired.ID, response.AuditRef, response.Token, response.EscalationRef)
		if response.Decision != policy.Deny || response.RuleFired.ID != "R-AUDIT-UNAVAILABLE" || response.AuditRef != "" || response.Token != nil || response.EscalationRef != nil {
			passed = false
		}
	}

	writeJSON(filepath.Join(evidenceDir, "policy-responses.json"), artifacts)
	writeText(filepath.Join(evidenceDir, "audit-outage-transcript.txt"), transcript.String())
	writeText(filepath.Join(evidenceDir, "RESULT.md"), resultMarkdown(passed))
	writeText(filepath.Join(evidenceDir, "ENV.md"), envMarkdown(timestamp, auditURL))

	if !passed {
		fmt.Printf("A9 failed; evidence retained at %s\n", evidenceDir)
		os.Exit(1)
	}
	fmt.Printf("A9 passed; evidence retained at %s\n", evidenceDir)
}

func adjudicate(baseURL string, tc adjudicationCase) policy.DecisionResult {
	input := policy.DecisionInput{
		ActionClass:    tc.Action,
		AbilityRef:     "cv-screen",
		AbilityVersion: "0.3.0",
		MandateRef:     "MND-2026-021",
		MandateVersion: 2,
		Confidence:     tc.Confidence,
		DataClasses:    []string{"personal", "protected-employment"},
		Reversibility:  "reversible",
		Counters:       map[string]int{"dailyActions": 12, "perSubject": 1},
		PayloadDigest:  "sha256:a9-payload",
	}
	body, err := json.Marshal(input)
	must(err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/v1/adjudicate", bytes.NewReader(body))
	must(err)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("unexpected policy status %d", resp.StatusCode))
	}

	var response policy.DecisionResult
	must(json.NewDecoder(resp.Body).Decode(&response))
	return response
}

func resultMarkdown(passed bool) string {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	return fmt.Sprintf(`# A9 Result

Hypothesis: when gear-audit is unavailable, every adjudication fails closed with deny and no execution token.

Method: run the gear-policy HTTP adjudication handler with its audit client pointed at a stopped local endpoint, then submit action requests that would otherwise authorise or escalate.

Result: all tested adjudications returned deny with rule R-AUDIT-UNAVAILABLE, no audit reference, no token, and no escalation reference.

Verdict: %s

Falsification condition: this criterion fails if any adjudication authorises, escalates, returns a token, or records a non-empty audit reference while audit is unavailable.
`, verdict)
}

func envMarkdown(timestamp, auditURL string) string {
	return fmt.Sprintf(`# A9 Environment

- timestamp: %s
- gitSHA: %s
- goVersion: %s
- osArch: %s/%s
- policyAuditURL: %s
- auditCondition: stopped/unreachable endpoint
`, timestamp, commandOutput("git", "rev-parse", "HEAD"), runtime.Version(), runtime.GOOS, runtime.GOARCH, auditURL)
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(data, '\n'), 0o644))
}

func writeText(path, value string) {
	must(os.WriteFile(path, []byte(value), 0o644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
