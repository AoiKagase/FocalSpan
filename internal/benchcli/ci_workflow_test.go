package benchcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflowCoversPublicVerificationWithoutPrivateState(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := strings.ReplaceAll(string(data), "\r\n", "\n")
	required := []string{
		"permissions:\n  contents: read",
		"go-version-file: go.mod",
		"go test ./...",
		"go vet ./...",
		"go test -race ./...",
		"runner: windows-latest",
		"goos: windows",
		"runner: ubuntu-latest",
		"goos: linux",
		"runner: macos-latest",
		"goos: darwin",
		"goarch: arm64",
		"CGO_ENABLED: 0",
		"go run ./cmd/focalspan-bench validate",
		"go run ./cmd/focalspan-bench run",
		"go run ./cmd/focalspan-bench compare",
		"docs/benchmarks/results-v0.5.json",
	}
	for _, snippet := range required {
		if !strings.Contains(workflow, snippet) {
			t.Errorf("workflow missing %q", snippet)
		}
	}
	for _, forbidden := range []string{"--registry", "--repo", "--keep-workspace", "upload-artifact", "contents: write", "pull_request_target"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("workflow contains forbidden %q", forbidden)
		}
	}
}

func TestCIHistoryDependentJobsCheckoutFullHistory(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, job := range []string{"test", "race", "public-benchmark"} {
		section := workflowJobSection(t, workflow, job)
		if !strings.Contains(section, "fetch-depth: 0") {
			t.Errorf("history-dependent job %q uses a shallow checkout", job)
		}
	}
}

func workflowJobSection(t *testing.T, workflow, job string) string {
	t.Helper()
	startMarker := "  " + job + ":"
	found := false
	var section strings.Builder
	for _, line := range strings.Split(workflow, "\n") {
		if line == startMarker {
			found = true
			continue
		}
		if !found {
			continue
		}
		if len(line) > 2 && strings.HasPrefix(line, "  ") && line[2] != ' ' && strings.HasSuffix(line, ":") {
			break
		}
		section.WriteString(line)
		section.WriteByte('\n')
	}
	if !found {
		t.Fatalf("workflow missing job %q", job)
	}
	return section.String()
}
