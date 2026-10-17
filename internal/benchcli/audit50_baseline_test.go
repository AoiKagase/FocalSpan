package benchcli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/benchmark"
	"os"
	"path/filepath"
	"testing"
)

func TestF04CompareMeasuredBaseline(t *testing.T) {
	baseline := "../../docs/benchmarks/results-whole-audit-baseline-v2.json"
	data, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	var report benchmark.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	args := []string{"compare", "--baseline", baseline, "--candidate", baseline, "--case", "php-extractor-integration", "--case", "cpp-extractor-registry"}
	if code := Run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("self compare code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	// One row worsens while all other quality metrics and matrix keys stay fixed.
	report.Quality[0].ForbiddenViolations++
	candidate := filepath.Join(t.TempDir(), "regression.json")
	data, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(candidate, data, 0600)
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"compare", "--baseline", baseline, "--candidate", candidate}, &out, &errOut); code != 2 {
		t.Fatalf("regression compare code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}
