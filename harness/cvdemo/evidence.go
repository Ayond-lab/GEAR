package cvdemoharness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func NewEvidenceDir(id string) string {
	dir := filepath.Join("evidence", id, time.Now().UTC().Format(time.RFC3339))
	Must(os.MkdirAll(dir, 0o755))
	return dir
}

func WriteJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	Must(err)
	Must(os.WriteFile(path, append(data, '\n'), 0o644))
}

func WriteText(path, value string) {
	Must(os.WriteFile(path, []byte(value), 0o644))
}

func WriteEnv(dir, id string) {
	WriteText(filepath.Join(dir, "ENV.md"), fmt.Sprintf(`# %s Environment

- timestamp: %s
- gitSHA: %s
- goVersion: %s
- osArch: %s/%s
- dataBoundary: synthetic fixture namespace only
`, id, time.Now().UTC().Format(time.RFC3339), commandOutput("git", "rev-parse", "HEAD"), runtime.Version(), runtime.GOOS, runtime.GOARCH))
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
