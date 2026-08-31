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
	for _, job := range []string{"test", "race", "public-benchmark-smoke", "public-benchmark-full"} {
		section := workflowJobSection(t, workflow, job)
		if !strings.Contains(section, "fetch-depth: 0") {
			t.Errorf("history-dependent job %q uses a shallow checkout", job)
		}
	}
}

func TestCISeparatesTwoCaseSmokeFromManualFullBenchmark(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.Contains(workflow, "workflow_dispatch:") {
		t.Fatal("workflow has no manual dispatch trigger")
	}
	smoke := workflowJobSection(t, workflow, "public-benchmark-smoke")
	if !strings.Contains(smoke, "github.event_name != 'workflow_dispatch'") || !strings.Contains(smoke, "--repeat 1") {
		t.Fatalf("smoke job is not limited to automatic repeat-1 runs:\n%s", smoke)
	}
	if got := strings.Count(smoke, "--case "); got != 6 {
		t.Fatalf("smoke job --case count = %d, want 6 (two each for validate, run, and compare)", got)
	}
	if strings.Contains(smoke, "--repeat 3") {
		t.Fatal("smoke job contains the full repeat count")
	}
	full := workflowJobSection(t, workflow, "public-benchmark-full")
	if !strings.Contains(full, "github.event_name == 'workflow_dispatch'") || !strings.Contains(full, "--repeat 3") {
		t.Fatalf("full job is not manual repeat-3 only:\n%s", full)
	}
	if strings.Contains(full, "--case ") {
		t.Fatal("manual full job filters the eight-case suite")
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
