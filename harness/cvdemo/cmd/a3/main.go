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
	"gear/internal/cvdemo"
)

func main() {
	dir := harness.NewEvidenceDir("A3")
	result, err := cvdemo.RunRecordAnnotationPath(context.Background(), func() time.Time { return time.Now().UTC() })
	if err != nil {
		writeResult(dir, false, []string{err.Error()}, result.Summary)
		panic(err)
	}

	auditJSON, err := json.MarshalIndent(result.AuditEntries, "", "  ")
	harness.Must(err)
	harness.WriteJSON(filepath.Join(dir, "run-summary.json"), result.Summary)
	harness.WriteJSON(filepath.Join(dir, "action-results.json"), result.Actions)
	harness.WriteJSON(filepath.Join(dir, "escalation-resources.json"), result.Escalations)
	harness.WriteJSON(filepath.Join(dir, "non-match-records.json"), result.NonMatches)
	harness.WriteText(filepath.Join(dir, "audit-entries.json"), string(auditJSON)+"\n")
	harness.WriteJSON(filepath.Join(dir, "chain-verification.json"), result.ChainVerification)
	harness.WriteJSON(filepath.Join(dir, "reconciliation.json"), map[string]any{"effectsWithoutDecision": result.EffectsWithoutDecision})
	harness.WriteEnv(dir, "A3")

	var failures []string
	if result.Summary.Applications != 60 {
		failures = append(failures, "fixture run did not cover 60 synthetic applications")
	}
	if result.Summary.Authorised != 45 {
		failures = append(failures, "expected 45 RECORD_ANNOTATE authorisations")
	}
	if result.Summary.Escalated != 3 || result.Summary.PendingEscalations != 3 || len(result.Escalations) != 3 {
		failures = append(failures, "expected 3 unclear cases to escalate and remain pending")
	}
	if result.Summary.Effects != 45 || result.Summary.EffectEntries != 45 {
		failures = append(failures, "expected exactly 45 executed effects")
	}
	if result.Summary.NonMatches != 12 || result.Summary.NonMatchEntries != 12 {
		failures = append(failures, "expected 12 retained non-match records")
	}
	if !result.ChainVerification.OK {
		failures = append(failures, "audit chain did not verify")
	}
	if len(result.EffectsWithoutDecision) != 0 {
		failures = append(failures, "reconciliation found effects without preceding decisions")
	}
	if findings := auditprivacy.Scan(string(auditJSON)); len(findings) != 0 {
		failures = append(failures, fmt.Sprintf("audit privacy scan found %d finding(s)", len(findings)))
	}

	passed := len(failures) == 0
	writeResult(dir, passed, failures, result.Summary)
	if !passed {
		fmt.Printf("A3 failed; evidence retained at %s\n", dir)
		os.Exit(1)
	}
	fmt.Printf("A3 passed; evidence retained at %s\n", dir)
}

func writeResult(dir string, passed bool, failures []string, summary cvdemo.RunSummary) {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A3 Annotation And Escalation Path\n\n")
	b.WriteString("Hypothesis: `RECORD_ANNOTATE` is authorised for 45 unambiguous synthetic applications, 3 unclear cases escalate, and no escalated action executes before human decision.\n\n")
	b.WriteString("Method: generate the 60 synthetic CV fixtures, derive trigger matches and non-matches, run matched actions through PEP/policy/audit, and reconcile effect entries.\n\n")
	b.WriteString(fmt.Sprintf("Result: applications=%d triggered=%d authorised=%d escalated=%d effects=%d nonMatches=%d.\n\n", summary.Applications, summary.TriggeredActions, summary.Authorised, summary.Escalated, summary.Effects, summary.NonMatches))
	b.WriteString("Verdict: " + verdict + "\n\n")
	b.WriteString("Falsification condition: A3 fails if the count is not 45 authorisations and 3 pending escalations, if an escalated action has an effect, or if reconciliation finds an effect without a decision.\n")
	if len(failures) > 0 {
		b.WriteString("\nFailures:\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}
	harness.WriteText(filepath.Join(dir, "RESULT.md"), b.String())
}
