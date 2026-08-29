package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"gear/internal/chain"
)

type verificationArtifact struct {
	Scenario   string        `json:"scenario"`
	EntryCount int           `json:"entryCount"`
	OK         bool          `json:"ok"`
	Affected   []chain.Range `json:"affected"`
}

func main() {
	ctx := context.Background()
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	evidenceDir := filepath.Join("evidence", "A7", timestamp)
	must(os.MkdirAll(evidenceDir, 0o755))

	dbPath := filepath.Join(evidenceDir, "audit.db")
	store, err := chain.OpenBoltStore(dbPath)
	must(err)
	for i := 1; i <= 500; i++ {
		_, err := store.Append(ctx, chain.Entry{
			TS:           "2026-08-26T00:00:00Z",
			Type:         "decision",
			ActionRef:    fmt.Sprintf("ga-a7-%03d", i),
			Actor:        "gear-policy",
			Mandate:      "MND-2026-021:2",
			Rule:         "R-PERMIT:1",
			Decision:     "authorise",
			InputsDigest: "sha256:a7-inputs",
			Model:        "none",
			DataAccessed: []string{"sha256:a7-payload"},
		})
		must(err)
	}
	must(store.Close())

	reopened, err := chain.OpenBoltStore(dbPath)
	must(err)
	defer reopened.Close()

	entries, err := reopened.Entries(ctx)
	must(err)
	valid, err := reopened.Verify(ctx)
	must(err)

	writeJSON(filepath.Join(evidenceDir, "verification-valid.json"), verificationArtifact{
		Scenario:   "500-entry durable chain",
		EntryCount: len(entries),
		OK:         valid.OK,
		Affected:   valid.Affected,
	})
	writeJSON(filepath.Join(evidenceDir, "audit-sample.json"), sampledEntries(entries))

	modified := cloneEntries(entries)
	modified[249].Decision = "deny"
	modification := chain.Verify(modified)
	writeJSON(filepath.Join(evidenceDir, "tamper-modification.json"), verificationArtifact{
		Scenario:   "seq 250 decision modified",
		EntryCount: len(modified),
		OK:         modification.OK,
		Affected:   modification.Affected,
	})

	deleted := make([]chain.Entry, 0, len(entries)-1)
	deleted = append(deleted, entries[:249]...)
	deleted = append(deleted, entries[250:]...)
	deletion := chain.Verify(deleted)
	writeJSON(filepath.Join(evidenceDir, "tamper-deletion.json"), verificationArtifact{
		Scenario:   "seq 250 deleted",
		EntryCount: len(deleted),
		OK:         deletion.OK,
		Affected:   deletion.Affected,
	})

	reordered := cloneEntries(entries)
	reordered[249], reordered[250] = reordered[250], reordered[249]
	reordering := chain.Verify(reordered)
	writeJSON(filepath.Join(evidenceDir, "tamper-reordering.json"), verificationArtifact{
		Scenario:   "seq 250 and 251 reordered",
		EntryCount: len(reordered),
		OK:         reordering.OK,
		Affected:   reordering.Affected,
	})

	passed := valid.OK && len(entries) == 500 && !modification.OK && !deletion.OK && !reordering.OK
	writeText(filepath.Join(evidenceDir, "RESULT.md"), resultMarkdown(passed, modification, deletion, reordering))
	writeText(filepath.Join(evidenceDir, "ENV.md"), envMarkdown(timestamp, dbPath))

	if !passed {
		fmt.Printf("A7 failed; evidence retained at %s\n", evidenceDir)
		os.Exit(1)
	}
	fmt.Printf("A7 passed; evidence retained at %s\n", evidenceDir)
}

func sampledEntries(entries []chain.Entry) []chain.Entry {
	if len(entries) <= 4 {
		return cloneEntries(entries)
	}
	sample := cloneEntries(entries[:3])
	sample = append(sample, entries[len(entries)-1])
	return sample
}

func cloneEntries(entries []chain.Entry) []chain.Entry {
	clone := make([]chain.Entry, len(entries))
	copy(clone, entries)
	return clone
}

func resultMarkdown(passed bool, modification, deletion, reordering chain.Verification) string {
	verdict := "FAIL"
	if passed {
		verdict = "PASS"
	}
	return fmt.Sprintf(`# A7 Result

Hypothesis: an append-only audit chain of at least 500 entries verifies, and modification, deletion, and reordering are detected with affected sequence ranges.

Method: create 500 durable bbolt-backed audit entries, reopen the store, verify the clean chain, then verify three tampered copies.

Result:

- modification affected ranges: %s
- deletion affected ranges: %s
- reordering affected ranges: %s

Verdict: %s

Falsification condition: this criterion fails if the clean chain does not verify, has fewer than 500 entries, or any tampered chain verifies as valid.
`, formatRanges(modification.Affected), formatRanges(deletion.Affected), formatRanges(reordering.Affected), verdict)
}

func envMarkdown(timestamp, dbPath string) string {
	return fmt.Sprintf(`# A7 Environment

- timestamp: %s
- gitSHA: %s
- goVersion: %s
- osArch: %s/%s
- auditStore: go.etcd.io/bbolt %s
- auditDB: %s
`, timestamp, commandOutput("git", "rev-parse", "HEAD"), runtime.Version(), runtime.GOOS, runtime.GOARCH, moduleVersion("go.etcd.io/bbolt"), filepath.Base(dbPath))
}

func formatRanges(ranges []chain.Range) string {
	if len(ranges) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(ranges))
	for _, affected := range ranges {
		parts = append(parts, fmt.Sprintf("%d-%d", affected.Start, affected.End))
	}
	return strings.Join(parts, ", ")
}

func moduleVersion(path string) string {
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
