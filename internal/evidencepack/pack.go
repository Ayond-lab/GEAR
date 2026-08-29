package evidencepack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Manifest struct {
	CreatedAt string      `json:"createdAt"`
	GitSHA    string      `json:"gitSha"`
	Criteria  []Criterion `json:"criteria"`
}

type Criterion struct {
	ID        string         `json:"id"`
	LatestRun string         `json:"latestRun,omitempty"`
	Verdict   string         `json:"verdict"`
	Files     []FileArtifact `json:"files"`
}

type FileArtifact struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func BuildManifest(root string, now time.Time) (Manifest, error) {
	criteria := make([]Criterion, 0, 10)
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("A%d", i)
		criterion, err := buildCriterion(root, id)
		if err != nil {
			return Manifest{}, err
		}
		criteria = append(criteria, criterion)
	}
	return Manifest{
		CreatedAt: now.UTC().Format(time.RFC3339),
		GitSHA:    commandOutput("git", "rev-parse", "HEAD"),
		Criteria:  criteria,
	}, nil
}

func Write(root, output string, now time.Time) (Manifest, error) {
	manifest, err := BuildManifest(root, now)
	if err != nil {
		return Manifest{}, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := writeArchive(root, output, append(manifestData, '\n')); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func buildCriterion(root, id string) (Criterion, error) {
	dir := filepath.Join(root, id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Criterion{ID: id, Verdict: "missing"}, nil
		}
		return Criterion{}, err
	}
	var runs []string
	for _, entry := range entries {
		if entry.IsDir() {
			runs = append(runs, entry.Name())
		}
	}
	sort.Strings(runs)
	if len(runs) == 0 {
		return Criterion{ID: id, Verdict: "missing"}, nil
	}
	latest := runs[len(runs)-1]
	latestDir := filepath.Join(dir, latest)
	files, err := artifactFiles(root, latestDir)
	if err != nil {
		return Criterion{}, err
	}
	verdict := verdictFromResult(filepath.Join(latestDir, "RESULT.md"))
	return Criterion{ID: id, LatestRun: latest, Verdict: verdict, Files: files}, nil
}

func artifactFiles(root, dir string) ([]FileArtifact, error) {
	var files []FileArtifact
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, FileArtifact{Path: filepath.ToSlash(rel), Bytes: info.Size(), SHA256: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func writeArchive(root, output string, manifest []byte) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{Name: "evidence-pack-manifest.json", Mode: 0o644, Size: int64(len(manifest)), ModTime: time.Now().UTC()}); err != nil {
		return err
	}
	if _, err := tw.Write(manifest); err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(root), path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func verdictFromResult(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}
	text := strings.ToUpper(string(data))
	if strings.Contains(text, "VERDICT: PASS") || strings.Contains(text, "VERDICT\n\nPASS") {
		return "PASS"
	}
	if strings.Contains(text, "VERDICT: FAIL") || strings.Contains(text, "VERDICT\n\nFAIL") {
		return "FAIL"
	}
	return "unknown"
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
