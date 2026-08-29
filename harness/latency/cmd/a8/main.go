package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	harness "gear/harness/cvdemo"
	"gear/internal/latency"
)

func main() {
	dir := harness.NewEvidenceDir("A8")
	result, err := latency.Run(context.Background(), latency.Config{Trials: 200, InferenceWorkers: 4})
	if err != nil {
		writeResult(dir, false, []string{err.Error()}, latency.Result{})
		panic(err)
	}

	harness.WriteJSON(filepath.Join(dir, "latency-histogram.json"), result.Histogram)
	harness.WriteJSON(filepath.Join(dir, "latency-summary.json"), result)
	harness.WriteJSON(filepath.Join(dir, "inference-load.json"), result.LoadProfile)
	harness.WriteText(filepath.Join(dir, "ENV.md"), envMarkdown(result))

	var failures []string
	if result.Trials < 200 {
		failures = append(failures, "fewer than 200 policy trials were recorded")
	}
	if result.InferenceIterations == 0 || result.InferenceWorkers == 0 {
		failures = append(failures, "inference load was not active")
	}
	if result.P95Micros < 0 || len(result.Histogram) == 0 {
		failures = append(failures, "p95 latency or histogram was not recorded")
	}
	if result.Decisions["authorise"] != result.Trials || result.AuditEntries != result.Trials {
		failures = append(failures, "policy trials were not all authorised with durable audit entries")
	}
	if !result.ChainVerification.OK {
		failures = append(failures, "latency audit chain did not verify")
	}

	passed := len(failures) == 0
	writeResult(dir, passed, failures, result)
	if !passed {
		fmt.Printf("A8 failed; evidence retained at %s\n", dir)
		os.Exit(1)
	}
	fmt.Printf("A8 passed; evidence retained at %s\n", dir)
}

func writeResult(dir string, passed bool, failures []string, result latency.Result) {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	var b strings.Builder
	b.WriteString("# A8 Policy Latency\n\n")
	b.WriteString("Hypothesis: `gear-policy` p95 latency is recorded over at least 200 trials while `gear-inference` is under load.\n\n")
	b.WriteString("Method: run 200 deterministic policy adjudications against the in-process policy core while four concurrent workers exercise the synthetic work-authorisation extractor.\n\n")
	b.WriteString(fmt.Sprintf("Result: trials=%d p95Micros=%d inferenceIterations=%d.\n\n", result.Trials, result.P95Micros, result.InferenceIterations))
	b.WriteString("Verdict: " + verdict + "\n\n")
	b.WriteString("Falsification condition: A8 fails if fewer than 200 trials run, inference load is inactive, p95 is not recorded, or any policy decision lacks durable audit evidence.\n")
	if len(failures) > 0 {
		b.WriteString("\nFailures:\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}
	harness.WriteText(filepath.Join(dir, "RESULT.md"), b.String())
}

func envMarkdown(result latency.Result) string {
	return fmt.Sprintf(`# A8 Environment

- timestamp: %s
- gitSHA: %s
- goVersion: %s
- policyTrials: %d
- inferenceWorkers: %d
- inferenceIterations: %d
- chainVerified: %t
- shopspringDecimal: %s
`, time.Now().UTC().Format(time.RFC3339), gitSHA(), runtime.Version(), result.Trials, result.InferenceWorkers, result.InferenceIterations, result.ChainVerification.OK, debugVersion("github.com/shopspring/decimal"))
}

func gitSHA() string {
	return commandOutput("git", "rev-parse", "HEAD")
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func debugVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			return dep.Version
		}
	}
	return "unknown"
}
