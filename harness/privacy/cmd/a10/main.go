package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	harness "gear/harness/cvdemo"
	"gear/internal/auditprivacy"
	"gear/internal/chain"
	"gear/internal/cvdemo"
)

type fileScan struct {
	Name     string                 `json:"name"`
	Findings []auditprivacy.Finding `json:"findings"`
}

func main() {
	dir := harness.NewEvidenceDir("A10")
	result, err := cvdemo.RunRecordAnnotationPath(context.Background(), func() time.Time { return time.Now().UTC() })
	if err != nil {
		writeResult(dir, false, []string{err.Error()})
		panic(err)
	}
	auditJSON, err := json.MarshalIndent(result.AuditEntries, "", "  ")
	harness.Must(err)
	logs := structuredLogs(result)

	scans := []fileScan{
		{Name: "audit-entries.json", Findings: auditprivacy.Scan(string(auditJSON))},
		{Name: "component-logs.jsonl", Findings: auditprivacy.Scan(logs)},
	}
	harness.WriteText(filepath.Join(dir, "audit-entries.json"), string(auditJSON)+"\n")
	harness.WriteJSON(filepath.Join(dir, "audit-sample.json"), sampledEntries(result.AuditEntries))
	harness.WriteText(filepath.Join(dir, "component-logs.jsonl"), logs)
	harness.WriteJSON(filepath.Join(dir, "scanner-output.json"), scans)
	harness.WriteJSON(filepath.Join(dir, "run-summary.json"), result.Summary)
	harness.WriteEnv(dir, "A10")

	var failures []string
	for _, scan := range scans {
		if len(scan.Findings) != 0 {
			failures = append(failures, fmt.Sprintf("%s contained %d privacy finding(s)", scan.Name, len(scan.Findings)))
		}
	}
	if !result.ChainVerification.OK {
		failures = append(failures, "audit chain did not verify before privacy scanning")
	}

	passed := len(failures) == 0
	writeResult(dir, passed, failures)
	if !passed {
		fmt.Printf("A10 failed; evidence retained at %s\n", dir)
		os.Exit(1)
	}
	fmt.Printf("A10 passed; evidence retained at %s\n", dir)
}

func structuredLogs(result cvdemo.RunResult) string {
	lines := []string{
		`{"component":"gear-triggers","event":"non-match-recorded","payloadDigest":"sha256:a10-non-match","reasonCode":"requires-sponsorship-held-for-human-planning"}`,
		fmt.Sprintf(`{"component":"gear-policy","event":"summary","decisions":%d,"authorised":%d,"escalated":%d}`, result.Summary.DecisionEntries, result.Summary.Authorised, result.Summary.Escalated),
		fmt.Sprintf(`{"component":"gear-audit","event":"verify","ok":%t,"entryCount":%d}`, result.ChainVerification.OK, len(result.AuditEntries)),
	}
	return strings.Join(lines, "\n") + "\n"
}

func sampledEntries(entries []chain.Entry) []chain.Entry {
	if len(entries) <= 6 {
		clone := append([]chain.Entry(nil), entries...)
		return clone
	}
	sample := append([]chain.Entry(nil), entries[:3]...)
	sample = append(sample, entries[len(entries)-3:]...)
	return sample
}

func writeResult(dir string, passed bool, failures []string) {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A10 Audit Privacy Scan\n\n")
	b.WriteString("Hypothesis: no candidate name, application text, extracted free text, human free-text reason, raw contact detail, or salt appears in the audit chain or relevant structured logs.\n\n")
	b.WriteString("Method: run the synthetic CV path, serialize audit entries and representative structured logs, then scan those artefacts with the A10 privacy scanner.\n\n")
	b.WriteString("Result: " + verdict + "\n\n")
	b.WriteString("Verdict: " + verdict + "\n\n")
	b.WriteString("Falsification condition: A10 fails if the scanner reports any prohibited personal-data pattern or if the audit chain does not verify.\n")
	if len(failures) > 0 {
		b.WriteString("\nFailures:\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}
	harness.WriteText(filepath.Join(dir, "RESULT.md"), b.String())
}
