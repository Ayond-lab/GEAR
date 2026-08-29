package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	evidenceDir := getenv("EVIDENCE_DIR", filepath.Join("evidence", "A5", time.Now().UTC().Format(time.RFC3339)))
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		fatal(err)
	}
	log := &commandLog{dir: evidenceDir}
	cluster := getenv("K3D_CLUSTER", "gear-lab")

	_, _ = log.run("kubectl", "config", "use-context", "k3d-"+cluster)
	writeEnv(evidenceDir, log)
	_, _ = log.run("kubectl", "apply", "-f", "deploy/smoke/fixtures/ability.yaml")
	out, err := log.run("kubectl", "apply", "-f", "deploy/smoke/fixtures/mandate-candidate-rank-permit.yaml")
	_, _ = log.run("kubectl", "-n", "gear-system", "logs", "deployment/gear-webhooks", "--tail=200")

	transcript := string(out)
	passed := err != nil && strings.Contains(transcript, "CANDIDATE_RANK was refused by legality gate")
	writeResult(evidenceDir, passed, transcript)
	if !passed {
		fatal(fmt.Errorf("A5 failed: expected legality-gate admission rejection, got: %s", transcript))
	}
	fmt.Printf("A5 passed. Evidence: %s\n", evidenceDir)
}

type commandLog struct {
	dir     string
	counter int
}

func (l *commandLog) run(name string, args ...string) ([]byte, error) {
	l.counter++
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	record := commandRecord(name, args, output, err)
	file := filepath.Join(l.dir, fmt.Sprintf("%03d-%s.txt", l.counter, safeName(name, args)))
	writeFile(file, record)
	appendFile(filepath.Join(l.dir, "commands.txt"), record+"\n")
	return output, err
}

func commandRecord(name string, args []string, output []byte, err error) string {
	var b bytes.Buffer
	b.WriteString("$ " + strings.Join(append([]string{name}, args...), " ") + "\n")
	if len(output) > 0 {
		b.WriteString("\n# output\n")
		b.Write(output)
		if !bytes.HasSuffix(output, []byte("\n")) {
			b.WriteString("\n")
		}
	}
	if err != nil {
		b.WriteString("\n# error\n")
		b.WriteString(err.Error() + "\n")
	}
	return b.String()
}

func writeResult(dir string, pass bool, transcript string) {
	verdict := "FAIL"
	if pass {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A5 Admission Rejection Experiment\n\n")
	b.WriteString("## Hypothesis\n\n")
	b.WriteString("A signed mandate that grants `CANDIDATE_RANK: permit` for the protected/selective citizenship purpose is rejected by Kubernetes admission.\n\n")
	b.WriteString("## Method\n\n")
	b.WriteString("The harness used the active cluster-admin context to run `kubectl apply` against the invalid signed mandate fixture and captured webhook logs.\n\n")
	b.WriteString("## Result\n\n")
	b.WriteString(verdict + "\n\n")
	b.WriteString("```text\n")
	b.WriteString(transcript)
	if !strings.HasSuffix(transcript, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n\n")
	b.WriteString("## Verdict\n\n")
	b.WriteString(verdict + "\n\n")
	b.WriteString("## Falsification Condition\n\n")
	b.WriteString("A5 is falsified if `kubectl apply` succeeds or if the rejection is not caused by the legality-gate refusal for `CANDIDATE_RANK`.\n")
	writeFile(filepath.Join(dir, "RESULT.md"), b.String())
}

func writeEnv(dir string, log *commandLog) {
	var b strings.Builder
	b.WriteString("# A5 Environment\n\n")
	if out, err := log.run("git", "rev-parse", "HEAD"); err == nil {
		b.WriteString("- Git SHA: `" + strings.TrimSpace(string(out)) + "`\n")
	}
	if out, err := log.run("kubectl", "config", "current-context"); err == nil {
		b.WriteString("- Kubernetes context: `" + strings.TrimSpace(string(out)) + "`\n")
	}
	b.WriteString("- Harness time: `" + time.Now().UTC().Format(time.RFC3339) + "`\n")
	writeFile(filepath.Join(dir, "ENV.md"), b.String())
}

func safeName(name string, args []string) string {
	parts := append([]string{name}, args...)
	for i, part := range parts {
		part = strings.Trim(part, "-")
		part = strings.ReplaceAll(part, "/", "-")
		part = strings.ReplaceAll(part, ".", "-")
		part = strings.ReplaceAll(part, "=", "-")
		part = strings.ReplaceAll(part, ":", "-")
		if len(part) > 24 {
			part = part[:24]
		}
		if part == "" {
			part = "arg"
		}
		parts[i] = part
	}
	return strings.Join(parts, "-")
}

func writeFile(path string, value string) {
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		fatal(err)
	}
}

func appendFile(path string, value string) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(value); err != nil {
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
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
