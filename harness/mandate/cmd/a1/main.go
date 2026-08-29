package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/chain"
	"gear/internal/mandatederive"
)

const unlawfulPurpose = "Check the CVs, select the candidates who are not citizens of the EEA."

func main() {
	evidenceDir := getenv("EVIDENCE_DIR", filepath.Join("evidence", "A1", time.Now().UTC().Format(time.RFC3339)))
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		fatal(err)
	}

	audit := &memoryAudit{}
	deriver := mandatederive.NewDeriver(audit)
	deriver.Now = func() time.Time { return time.Now().UTC() }
	request := mandatederive.Request{
		MandateID:           "MND-A1-REFUSED",
		Version:             1,
		AbilityRef:          "cv-screen",
		Ability:             cvScreenAbilitySpec(),
		PurposeStatement:    unlawfulPurpose,
		OperatorResponseRef: "sha256:operator-response-a1",
	}
	response, err := deriver.Derive(context.Background(), request)
	if err != nil {
		writeResult(evidenceDir, false, []string{err.Error()})
		fatal(err)
	}
	verification := chain.Verify(audit.entries)

	writeJSON(filepath.Join(evidenceDir, "refusal-transcript.json"), map[string]any{
		"purposeStatement": unlawfulPurpose,
		"operatorResponse": "Operator selected the lawful planning alternative for the next request.",
		"response":         response,
	})
	writeJSON(filepath.Join(evidenceDir, "audit-entries.json"), audit.entries)
	writeJSON(filepath.Join(evidenceDir, "chain-verification.json"), verification)
	writeEnv(evidenceDir)

	var failures []string
	if response.Outcome != "refused" || response.Mandate != nil {
		failures = append(failures, "unlawful purpose produced a mandate")
	}
	if response.Refusal == nil || response.Refusal.Criterion != "citizenship" || response.Refusal.Verb != "select" {
		failures = append(failures, "refusal did not identify citizenship/select")
	}
	if len(audit.entries) != 1 || audit.entries[0].Type != "mandate-refused" {
		failures = append(failures, "mandate-refused audit entry was not written")
	}
	if !verification.OK {
		failures = append(failures, "audit chain did not verify")
	}
	if len(audit.entries) == 1 {
		data, _ := json.Marshal(audit.entries[0])
		if strings.Contains(string(data), "citizens") || strings.Contains(string(data), "Check the CVs") {
			failures = append(failures, "audit entry contains raw purpose text")
		}
	}

	writeResult(evidenceDir, len(failures) == 0, failures)
	if len(failures) > 0 {
		fatal(fmt.Errorf("A1 failed: %s", strings.Join(failures, "; ")))
	}
	fmt.Printf("A1 passed. Evidence: %s\n", evidenceDir)
}

type memoryAudit struct {
	entries []chain.Entry
	prev    chain.Entry
}

func (m *memoryAudit) Append(_ context.Context, entry chain.Entry) (chain.Entry, error) {
	stored, err := chain.Append(m.prev, entry)
	if err != nil {
		return chain.Entry{}, err
	}
	m.entries = append(m.entries, stored)
	m.prev = stored
	return stored, nil
}

func cvScreenAbilitySpec() gearv1.AbilitySpec {
	return gearv1.AbilitySpec{
		Version:       "0.3.0",
		Certification: "certified",
		DeclaredTriggers: []gearv1.TriggerDecl{
			{Type: "folder", ID: "applications-inbox"},
		},
		ConnectorScopes: []gearv1.ConnectorScope{
			{Connector: "applications-store", Scope: "read"},
			{Connector: "candidate-record", Scope: "write"},
		},
		ActionClasses: []string{"RECORD_ANNOTATE", "RECORD_MODIFY", "CANDIDATE_RANK", "OUTBOUND_COMMS"},
		Reversibility: map[string]string{
			"RECORD_ANNOTATE": "reversible",
			"RECORD_MODIFY":   "reversible",
			"CANDIDATE_RANK":  "reversible",
			"OUTBOUND_COMMS":  "irreversible",
		},
		DataClasses: []string{"personal", "protected-employment"},
		Ceilings:    gearv1.Ceilings{DailyActions: 500},
	}
}

func writeResult(dir string, pass bool, failures []string) {
	verdict := "FAIL"
	if pass {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A1 Mandate Refusal Experiment\n\n")
	b.WriteString("## Hypothesis\n\n")
	b.WriteString("The protected-attribute selective purpose is refused, no mandate is produced, clarification is returned, and a `mandate-refused` audit entry is retained.\n\n")
	b.WriteString("## Method\n\n")
	b.WriteString("The harness submitted the unlawful synthetic CV-screening purpose to the deterministic `gear-mandate` derivation path with a synthetic operator response reference, then verified the retained audit chain.\n\n")
	b.WriteString("## Result\n\n")
	b.WriteString(verdict + "\n\n")
	b.WriteString("## Verdict\n\n")
	b.WriteString(verdict + "\n\n")
	b.WriteString("## Falsification Condition\n\n")
	b.WriteString("A1 is falsified if the unlawful purpose creates a mandate, omits the lawful alternatives, fails to write `mandate-refused`, or fails audit-chain verification.\n")
	if len(failures) > 0 {
		b.WriteString("\n## Failures\n\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}
	writeFile(filepath.Join(dir, "RESULT.md"), b.String())
}

func writeEnv(dir string) {
	var b strings.Builder
	b.WriteString("# A1 Environment\n\n")
	if out, err := exec.Command("git", "rev-parse", "HEAD").CombinedOutput(); err == nil {
		b.WriteString("- Git SHA: `" + strings.TrimSpace(string(out)) + "`\n")
	}
	b.WriteString("- Harness time: `" + time.Now().UTC().Format(time.RFC3339) + "`\n")
	writeFile(filepath.Join(dir, "ENV.md"), b.String())
}

func writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal(err)
	}
	writeFile(path, string(data)+"\n")
}

func writeFile(path string, value string) {
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		fatal(err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
