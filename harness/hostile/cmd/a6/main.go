package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	gearLab        = "gear-lab"
	gearSystem     = "gear-system"
	withControl    = "a6-with-control"
	withoutControl = "a6-without-control"
	resultPrefix   = "A6_RESULT "
)

type config struct {
	Cluster            string
	RunnerImage        string
	TargetImage        string
	PEPImage           string
	NetinitImage       string
	EvidenceDir        string
	NegativeStartDelay string
}

type commandLog struct {
	dir     string
	counter int
}

type runResult struct {
	Control   string         `json:"control"`
	StartedAt string         `json:"startedAt"`
	Attacks   []attackResult `json:"attacks"`
}

type attackResult struct {
	ID             string   `json:"id"`
	Description    string   `json:"description"`
	Target         string   `json:"target"`
	Outcome        string   `json:"outcome"`
	Reached        bool     `json:"reached"`
	HTTPStatus     int      `json:"httpStatus,omitempty"`
	Resolved       []string `json:"resolved,omitempty"`
	Error          string   `json:"error,omitempty"`
	DurationMillis int64    `json:"durationMillis"`
}

func main() {
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.EvidenceDir, 0o755); err != nil {
		fatal(err)
	}
	log := &commandLog{dir: cfg.EvidenceDir}

	if err := runA6(cfg, log); err != nil {
		if _, statErr := os.Stat(filepath.Join(cfg.EvidenceDir, "RESULT.md")); errors.Is(statErr, os.ErrNotExist) {
			writeResultMarkdown(cfg.EvidenceDir, nil, nil, []string{err.Error()})
		}
		fatal(err)
	}
}

func runA6(cfg config, log *commandLog) error {
	if _, err := log.run("", "kubectl", "config", "use-context", "k3d-"+cfg.Cluster); err != nil {
		return err
	}

	writeEnv(cfg, log)

	if err := applyLabPrerequisites(log); err != nil {
		return err
	}
	cleanupTargets(log)
	if err := applyTargets(cfg, log); err != nil {
		return err
	}
	for _, deployment := range []string{"a6-gear-policy", "a6-gear-inference", "a6-gear-fixture-store"} {
		if _, err := log.run("", "kubectl", "-n", gearSystem, "rollout", "status", "deployment/"+deployment, "--timeout=120s"); err != nil {
			return err
		}
	}

	policyIPOutput, err := log.run("", "kubectl", "-n", gearSystem, "get", "pod", "-l", "gear.eu/a6-target=true,app.kubernetes.io/name=gear-policy", "-o", "jsonpath={.items[0].status.podIP}")
	if err != nil {
		return err
	}
	policyIP := strings.TrimSpace(string(policyIPOutput))
	if policyIP == "" {
		return errors.New("a6-gear-policy target pod has no Pod IP")
	}

	_, _ = log.run("", "kubectl", "-n", gearLab, "delete", "pod", withControl, withoutControl, "--ignore-not-found=true")

	if err := applyPod(log, withControlPod(cfg, policyIP)); err != nil {
		return err
	}
	withLogs, err := waitForResults(log, withControl, 120*time.Second)
	if err != nil {
		capturePodEvidence(log, withControl)
		return err
	}
	withResult, err := parseResult(withLogs)
	if err != nil {
		return err
	}
	writeJSON(filepath.Join(cfg.EvidenceDir, "with-control-results.json"), withResult)
	capturePodEvidence(log, withControl)

	if err := applyPod(log, withoutControlPod(cfg, policyIP)); err != nil {
		return err
	}
	if _, err := log.run("", "kubectl", "-n", gearLab, "label", "pod", withoutControl, "gear.eu/ability=cv-screen", "--overwrite"); err != nil {
		return err
	}
	withoutLogs, err := waitForResults(log, withoutControl, 120*time.Second)
	if err != nil {
		capturePodEvidence(log, withoutControl)
		return err
	}
	withoutResult, err := parseResult(withoutLogs)
	if err != nil {
		return err
	}
	writeJSON(filepath.Join(cfg.EvidenceDir, "without-control-results.json"), withoutResult)
	capturePodEvidence(log, withoutControl)
	captureClusterEvidence(log)

	failures := validateResults(withResult, withoutResult)
	writeResultMarkdown(cfg.EvidenceDir, withResult, withoutResult, failures)
	if len(failures) > 0 {
		return fmt.Errorf("A6 failed: %s", strings.Join(failures, "; "))
	}

	fmt.Printf("A6 passed. Evidence: %s\n", cfg.EvidenceDir)
	return nil
}

func loadConfig() config {
	return config{
		Cluster:            getenv("K3D_CLUSTER", "gear-lab"),
		RunnerImage:        getenv("HOSTILE_RUNNER_IMAGE", "ghcr.io/ayond-lab/gear-hostile-runner:dev"),
		TargetImage:        getenv("HOSTILE_TARGET_IMAGE", "ghcr.io/ayond-lab/gear-hostile-target:dev"),
		PEPImage:           getenv("PEP_IMAGE", "ghcr.io/ayond-lab/gear-pep:dev"),
		NetinitImage:       getenv("NETINIT_IMAGE", "ghcr.io/ayond-lab/gear-netinit:dev"),
		EvidenceDir:        getenv("EVIDENCE_DIR", filepath.Join("evidence", "A6", time.Now().UTC().Format(time.RFC3339))),
		NegativeStartDelay: getenv("A6_NEGATIVE_START_DELAY_SECONDS", "6"),
	}
}

func writeEnv(cfg config, log *commandLog) {
	var b strings.Builder
	b.WriteString("# A6 Environment\n\n")
	b.WriteString("- Cluster: `k3d-" + cfg.Cluster + "`\n")
	b.WriteString("- Runner image: `" + cfg.RunnerImage + "`\n")
	b.WriteString("- Target image: `" + cfg.TargetImage + "`\n")
	b.WriteString("- PEP image: `" + cfg.PEPImage + "`\n")
	b.WriteString("- Netinit image: `" + cfg.NetinitImage + "`\n")

	if out, err := log.run("", "git", "rev-parse", "HEAD"); err == nil {
		b.WriteString("- Git SHA: `" + strings.TrimSpace(string(out)) + "`\n")
	}
	if out, err := log.run("", "kubectl", "config", "current-context"); err == nil {
		b.WriteString("- Kubernetes context: `" + strings.TrimSpace(string(out)) + "`\n")
	}

	_, _ = log.run("", "kubectl", "get", "nodes", "-o", "wide")
	_, _ = log.run("", "kubectl", "-n", "kube-system", "get", "daemonset", "cilium", "-o", "wide")
	if _, err := exec.LookPath("cilium"); err == nil {
		_, _ = log.run("", "cilium", "status", "--wait")
	}
	if _, err := exec.LookPath("hubble"); err != nil {
		_ = os.WriteFile(filepath.Join(cfg.EvidenceDir, "hubble-observe.txt"), []byte("hubble CLI not available in this environment\n"), 0o644)
	}

	writeFile(filepath.Join(cfg.EvidenceDir, "ENV.md"), b.String())
}

func applyLabPrerequisites(log *commandLog) error {
	return applyYAML(log, "lab-prerequisites", `apiVersion: v1
kind: Namespace
metadata:
  name: gear-system
  labels:
    kubernetes.io/metadata.name: gear-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: gear-lab
  labels:
    kubernetes.io/metadata.name: gear-lab
---
apiVersion: v1
kind: Secret
metadata:
  name: gear-connector-credentials
  namespace: gear-lab
type: Opaque
data: {}
`)
}

func applyTargets(cfg config, log *commandLog) error {
	return applyYAML(log, "targets", fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: a6-gear-policy
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-policy
spec:
  replicas: 1
  selector:
    matchLabels:
      gear.eu/a6-target: "true"
      app.kubernetes.io/name: gear-policy
  template:
    metadata:
      labels:
        gear.eu/a6-target: "true"
        app.kubernetes.io/name: gear-policy
    spec:
      automountServiceAccountToken: false
      containers:
        - name: target
          image: %q
          imagePullPolicy: IfNotPresent
          env:
            - name: A6_TARGET_NAME
              value: gear-policy
          ports:
            - name: http
              containerPort: 8080
            - name: mtls
              containerPort: 443
---
apiVersion: v1
kind: Service
metadata:
  name: a6-gear-policy
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
spec:
  clusterIP: None
  selector:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-policy
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: mtls
      port: 443
      targetPort: 443
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: a6-gear-inference
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-inference
spec:
  replicas: 1
  selector:
    matchLabels:
      gear.eu/a6-target: "true"
      app.kubernetes.io/name: gear-inference
  template:
    metadata:
      labels:
        gear.eu/a6-target: "true"
        app.kubernetes.io/name: gear-inference
    spec:
      automountServiceAccountToken: false
      containers:
        - name: target
          image: %q
          imagePullPolicy: IfNotPresent
          env:
            - name: A6_TARGET_NAME
              value: gear-inference
          ports:
            - name: http
              containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: a6-gear-inference
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
spec:
  clusterIP: None
  selector:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-inference
  ports:
    - name: http
      port: 8080
      targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: a6-gear-fixture-store
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-fixture-store
spec:
  replicas: 1
  selector:
    matchLabels:
      gear.eu/a6-target: "true"
      app.kubernetes.io/name: gear-fixture-store
  template:
    metadata:
      labels:
        gear.eu/a6-target: "true"
        app.kubernetes.io/name: gear-fixture-store
    spec:
      automountServiceAccountToken: false
      containers:
        - name: target
          image: %q
          imagePullPolicy: IfNotPresent
          env:
            - name: A6_TARGET_NAME
              value: gear-fixture-store
          ports:
            - name: http
              containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: a6-gear-fixture-store
  namespace: gear-system
  labels:
    gear.eu/a6-target: "true"
spec:
  clusterIP: None
  selector:
    gear.eu/a6-target: "true"
    app.kubernetes.io/name: gear-fixture-store
  ports:
    - name: http
      port: 8080
      targetPort: 8080
`, cfg.TargetImage, cfg.TargetImage, cfg.TargetImage))
}

func cleanupTargets(log *commandLog) {
	_, _ = log.run("", "kubectl", "-n", gearSystem, "delete", "service", "a6-gear-policy", "a6-gear-inference", "a6-gear-fixture-store", "--ignore-not-found=true")
	_, _ = log.run("", "kubectl", "-n", gearSystem, "delete", "deployment", "a6-gear-policy", "a6-gear-inference", "a6-gear-fixture-store", "--ignore-not-found=true")
}

func withControlPod(cfg config, policyIP string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    gear.eu/ability: cv-screen
    gear.eu/a6-control: with
spec:
  restartPolicy: Never
  containers:
    - name: ability
      image: %q
      imagePullPolicy: IfNotPresent
      env:
        - name: A6_CONTROL
          value: with-control
        - name: A6_POLICY_DNS
          value: a6-gear-policy.gear-system.svc.cluster.local
        - name: A6_INFERENCE_DNS
          value: a6-gear-inference.gear-system.svc.cluster.local
        - name: A6_FIXTURE_DNS
          value: a6-gear-fixture-store.gear-system.svc.cluster.local
        - name: A6_POLICY_IP
          value: %q
      securityContext:
        runAsUser: 1001
        runAsNonRoot: true
        allowPrivilegeEscalation: false
`, withControl, gearLab, cfg.RunnerImage, policyIP)
}

func withoutControlPod(cfg config, policyIP string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    gear.eu/a6-control: without
spec:
  automountServiceAccountToken: false
  restartPolicy: Never
  containers:
    - name: ability
      image: %q
      imagePullPolicy: IfNotPresent
      env:
        - name: A6_CONTROL
          value: without-control
        - name: A6_START_DELAY_SECONDS
          value: %q
        - name: A6_POLICY_DNS
          value: a6-gear-policy.gear-system.svc.cluster.local
        - name: A6_INFERENCE_DNS
          value: a6-gear-inference.gear-system.svc.cluster.local
        - name: A6_FIXTURE_DNS
          value: a6-gear-fixture-store.gear-system.svc.cluster.local
        - name: A6_POLICY_IP
          value: %q
      securityContext:
        runAsUser: 1001
        runAsNonRoot: true
        allowPrivilegeEscalation: false
    - name: gear-pep
      image: %q
      imagePullPolicy: IfNotPresent
      args:
        - --listen=127.0.0.1:9191
      ports:
        - name: pep-loopback
          containerPort: 9191
      env:
        - name: GEAR_PEP_LISTEN
          value: 127.0.0.1:9191
        - name: GEAR_ABILITY_REF
          value: cv-screen
      volumeMounts:
        - name: gear-pep-mtls
          mountPath: /var/run/gear/mtls
          readOnly: true
        - name: gear-connector-credentials
          mountPath: /var/run/gear/connectors
          readOnly: true
      securityContext:
        runAsUser: 1337
        runAsNonRoot: true
        allowPrivilegeEscalation: false
  volumes:
    - name: gear-pep-mtls
      secret:
        secretName: gear-pep-mtls
    - name: gear-connector-credentials
      secret:
        secretName: gear-connector-credentials
`, withoutControl, gearLab, cfg.RunnerImage, cfg.NegativeStartDelay, policyIP, cfg.PEPImage)
}

func applyPod(log *commandLog, manifest string) error {
	return applyYAML(log, "pod", manifest)
}

func applyYAML(log *commandLog, label string, manifest string) error {
	_, err := log.run(manifest, "kubectl", "apply", "-f", "-")
	return annotate(err, "apply "+label)
}

func waitForResults(log *commandLog, pod string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		out, _ := log.run("", "kubectl", "-n", gearLab, "logs", pod, "-c", "ability")
		last = string(out)
		if strings.Contains(last, resultPrefix) {
			writeFile(filepath.Join(log.dir, pod+"-ability.log"), last)
			return last, nil
		}
		time.Sleep(2 * time.Second)
	}
	writeFile(filepath.Join(log.dir, pod+"-ability.log"), last)
	return "", fmt.Errorf("timed out waiting for %s ability results", pod)
}

func parseResult(logs string) (*runResult, error) {
	index := strings.Index(logs, resultPrefix)
	if index < 0 {
		return nil, errors.New("A6 result marker not found")
	}
	payload := strings.TrimSpace(logs[index+len(resultPrefix):])
	var result runResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func capturePodEvidence(log *commandLog, pod string) {
	_, _ = log.run("", "kubectl", "-n", gearLab, "get", "pod", pod, "-o", "yaml")
	_, _ = log.run("", "kubectl", "-n", gearLab, "describe", "pod", pod)
	_, _ = log.run("", "kubectl", "-n", gearLab, "logs", pod, "-c", "ability")
	_, _ = log.run("", "kubectl", "-n", gearLab, "logs", pod, "-c", "gear-pep")
	_, _ = log.run("", "kubectl", "-n", gearLab, "logs", pod, "-c", "gear-uid-egress")
}

func captureClusterEvidence(log *commandLog) {
	_, _ = log.run("", "kubectl", "-n", gearLab, "get", "networkpolicy", "gear-ability-egress-baseline", "-o", "yaml")
	_, _ = log.run("", "kubectl", "-n", gearSystem, "get", "pods,services", "-l", "gear.eu/a6-target=true", "-o", "wide")
	if _, err := exec.LookPath("hubble"); err == nil {
		_, _ = log.run("", "hubble", "observe", "--namespace", gearLab, "--last", "80")
	}
}

func validateResults(withResult, withoutResult *runResult) []string {
	var failures []string
	if withResult == nil {
		failures = append(failures, "missing with-control result")
	}
	if withoutResult == nil {
		failures = append(failures, "missing without-control result")
	}
	if len(failures) > 0 {
		return failures
	}

	if len(withResult.Attacks) != 8 {
		failures = append(failures, fmt.Sprintf("with-control ran %d attacks, expected 8", len(withResult.Attacks)))
	}
	if len(withoutResult.Attacks) != 8 {
		failures = append(failures, fmt.Sprintf("without-control ran %d attacks, expected 8", len(withoutResult.Attacks)))
	}

	for _, attack := range withResult.Attacks {
		if attack.Reached {
			failures = append(failures, fmt.Sprintf("with-control attack %s reached %s", attack.ID, attack.Target))
		}
	}
	for _, attack := range withoutResult.Attacks {
		if !attack.Reached {
			failures = append(failures, fmt.Sprintf("without-control attack %s did not reach %s: %s", attack.ID, attack.Target, attack.Error))
		}
		if attack.ID == "mtls-policy-443" && attack.Outcome != "auth-failed" {
			failures = append(failures, fmt.Sprintf("without-control mTLS probe outcome %q, expected auth-failed", attack.Outcome))
		}
	}

	return failures
}

func writeResultMarkdown(dir string, withResult, withoutResult *runResult, failures []string) {
	verdict := "PASS"
	if len(failures) > 0 {
		verdict = "FAIL"
	}

	var b strings.Builder
	b.WriteString("# A6 Hostile Egress Experiment\n\n")
	b.WriteString("## Hypothesis\n\n")
	b.WriteString("Ability-container non-loopback egress fails when the UID-scoped init-container control is present, and the same egress succeeds when that init container is removed.\n\n")
	b.WriteString("## Method\n\n")
	b.WriteString("The harness deployed three synthetic GEAR destination services, ran eight hostile egress paths from a labelled ability pod, then ran the same paths from an equivalent pod without the UID egress init container. The negative-control pod received the `gear.eu/ability` label after creation so the NetworkPolicy baseline still applied.\n\n")
	b.WriteString("## Result\n\n")
	b.WriteString("| Condition | Attack | Reached target | Outcome |\n")
	b.WriteString("|---|---|---:|---|\n")
	for _, row := range resultRows("with-control", withResult) {
		b.WriteString(row)
	}
	for _, row := range resultRows("without-control", withoutResult) {
		b.WriteString(row)
	}
	b.WriteString("\n## Verdict\n\n")
	b.WriteString(verdict + "\n\n")
	b.WriteString("## Falsification Condition\n\n")
	b.WriteString("A6 is falsified if any with-control hostile egress reaches a non-loopback destination, or if the negative-control egress cannot reach the same destinations after the init container is removed.\n")
	if len(failures) > 0 {
		b.WriteString("\n## Failures\n\n")
		for _, failure := range failures {
			b.WriteString("- " + failure + "\n")
		}
	}

	writeFile(filepath.Join(dir, "RESULT.md"), b.String())
}

func resultRows(condition string, result *runResult) []string {
	if result == nil {
		return nil
	}
	rows := make([]string, 0, len(result.Attacks))
	for _, attack := range result.Attacks {
		reached := "no"
		if attack.Reached {
			reached = "yes"
		}
		rows = append(rows, fmt.Sprintf("| %s | `%s` | %s | `%s` |\n", condition, attack.ID, reached, attack.Outcome))
	}
	sort.Strings(rows)
	return rows
}

func (l *commandLog) run(stdin string, name string, args ...string) ([]byte, error) {
	l.counter++
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	output, err := cmd.CombinedOutput()
	record := commandRecord(name, args, stdin, output, err)
	file := filepath.Join(l.dir, fmt.Sprintf("%03d-%s.txt", l.counter, safeName(name, args)))
	writeFile(file, record)
	appendFile(filepath.Join(l.dir, "commands.txt"), record+"\n")
	return output, err
}

func commandRecord(name string, args []string, stdin string, output []byte, err error) string {
	var b bytes.Buffer
	b.WriteString("$ " + strings.Join(append([]string{name}, args...), " ") + "\n")
	if stdin != "" {
		b.WriteString("\n# stdin\n")
		b.WriteString(stdin)
		if !strings.HasSuffix(stdin, "\n") {
			b.WriteString("\n")
		}
	}
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

func safeName(name string, args []string) string {
	parts := append([]string{name}, args...)
	for i, part := range parts {
		part = strings.Trim(part, "-")
		part = strings.ReplaceAll(part, "/", "-")
		part = strings.ReplaceAll(part, ".", "-")
		part = strings.ReplaceAll(part, "=", "-")
		part = strings.ReplaceAll(part, "{", "")
		part = strings.ReplaceAll(part, "}", "")
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

func annotate(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
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
