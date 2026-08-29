package evidencepack

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildManifestFindsLatestRunsAndVerdicts(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A1", "2026-08-29T10:00:00Z", "RESULT.md"), "Verdict: FAIL\n")
	write(t, filepath.Join(root, "A1", "2026-08-29T11:00:00Z", "RESULT.md"), "Verdict: PASS\n")
	write(t, filepath.Join(root, "A1", "2026-08-29T11:00:00Z", "ENV.md"), "env\n")

	manifest, err := BuildManifest(root, time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CreatedAt != "2026-08-29T12:00:00Z" || len(manifest.Criteria) != 10 {
		t.Fatalf("unexpected manifest %#v", manifest)
	}
	if manifest.Criteria[0].ID != "A1" || manifest.Criteria[0].LatestRun != "2026-08-29T11:00:00Z" || manifest.Criteria[0].Verdict != "PASS" {
		t.Fatalf("unexpected A1 manifest entry %#v", manifest.Criteria[0])
	}
	if manifest.Criteria[1].ID != "A2" || manifest.Criteria[1].Verdict != "missing" {
		t.Fatalf("expected missing A2, got %#v", manifest.Criteria[1])
	}
	if len(manifest.Criteria[0].Files) != 2 || manifest.Criteria[0].Files[0].SHA256 == "" {
		t.Fatalf("expected file artifacts with hashes, got %#v", manifest.Criteria[0].Files)
	}
}

func TestWriteCreatesTarballWithManifest(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A8", "2026-08-29T12:00:00Z", "RESULT.md"), "Verdict: PASS\n")
	output := filepath.Join(t.TempDir(), "evidence-pack.tgz")

	_, err := Write(root, output, time.Date(2026, 8, 29, 12, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	foundManifest := false
	foundResult := false
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header.Name == "evidence-pack-manifest.json" {
			foundManifest = true
		}
		if header.Name == filepath.ToSlash(filepath.Join(filepath.Base(root), "A8", "2026-08-29T12:00:00Z", "RESULT.md")) {
			foundResult = true
		}
	}
	if !foundManifest || !foundResult {
		t.Fatalf("expected archive to contain manifest and result, foundManifest=%t foundResult=%t", foundManifest, foundResult)
	}
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
