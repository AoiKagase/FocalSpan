package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaintainerDocumentationDefinesBenchmarkBoundary(t *testing.T) {
	designPath := filepath.Join("..", "..", "docs", "design.md")
	designBytes, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatal(err)
	}
	design := strings.ReplaceAll(string(designBytes), "\r\n", "\n")
	for _, required := range []string{
		"## Development-only real-repository evaluation",
		"local Git repository\n  -> read-only git archive base snapshot",
		"current FocalSpan index, query, and Evidence pipeline",
		"human labels plus target-diff diagnostics",
		"deterministic quality report\n  -> separate volatile timing report",
		"never executes repository code",
		"makes no network request",
	} {
		if !strings.Contains(design, required) {
			t.Errorf("design documentation missing %q", required)
		}
	}
}
