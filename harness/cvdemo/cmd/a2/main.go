package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	harness "gear/harness/cvdemo"
	"gear/internal/cvdemo"
	"gear/internal/policy"
)

func main() {
	dir := harness.NewEvidenceDir("A2")
	result, err := cvdemo.RunCandidateRankDeny(context.Background(), func() time.Time { return time.Now().UTC() })
	if err != nil {
		writeResult(dir, false, []string{err.Error()})
		panic(err)
	}

	harness.WriteJSON(filepath.Join(dir, "policy-response.json"), result.Decision)
	harness.WriteJSON(filepath.Join(dir, "governedaction-status.json"), result.GovernedStatus)
	harness.WriteJSON(filepath.Join(dir, "audit-entries.json"), result.AuditEntries)
	harness.WriteJSON(filepath.Join(dir, "chain-verification.json"), result.ChainVerification)
	harness.WriteJSON(filepath.Join(dir, "reconciliation.json"), map[string]any{"effectsWithoutDecision": result.EffectsWithoutDecision})
	harness.WriteEnv(dir, "A2")

	var failures []string
	if result.Decision.Decision != string(policy.Deny) {
		failures = append(failures, "CANDIDATE_RANK did not deny")
	}
	if result.Decision.RuleFired.ID != "D1" {
		failures = append(failures, "deny did not cite mandate clause D1")
	}
	if result.Decision.EffectRef != "" || result.GovernedStatus.EffectRef != "" {
		failures = append(failures, "denied rank request produced an effect reference")
	}
	if result.GovernedStatus.ExecutionState != "refused" {
		failures = append(failures, "GovernedAction was not marked refused")
	}
	if !result.ChainVerification.OK || len(result.AuditEntries) != 1 || result.AuditEntries[0].Type != "decision" {
		failures = append(failures, "audit chain did not retain exactly one verified decision entry")
	}

	passed := len(failures) == 0
	writeResult(dir, passed, failures)
	if !passed {
		fmt.Printf("A2 failed; evidence retained at %s\n", dir)
		os.Exit(1)
	}
	fmt.Printf("A2 passed; evidence retained at %s\n", dir)
}

func writeResult(dir string, passed bool, failures []string) {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A2 Candidate Rank Denial\n\n")
	b.WriteString("Hypothesis: under `MND-2026-021` v2, `CANDIDATE_RANK` returns deny, cites D1, produces no effect, and is recorded.\n\n")
	b.WriteString("Method: submit a synthetic `CANDIDATE_RANK` effect intent through the in-process PEP, policy adjudicator, and append-only audit chain.\n\n")
	b.WriteString("Result: " + verdict + "\n\n")
	b.WriteString("Verdict: " + verdict + "\n\n")
	b.WriteString("Falsification condition: A2 fails if ranking authorises or escalates, omits D1, writes an effect, or fails audit-chain verification.\n")
	if len(failures) > 0 {
		b.WriteString("\nFailures:\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}
	harness.WriteText(filepath.Join(dir, "RESULT.md"), b.String())
}
