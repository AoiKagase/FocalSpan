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
