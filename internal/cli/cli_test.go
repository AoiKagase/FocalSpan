package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesFocalSpanDefaultsWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"init", "--root", root}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".focalspan.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".focalspan")); err != nil {
		t.Fatal(err)
	}
	if code := Run(context.Background(), []string{"init", "--root", root}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "already exists") {
		t.Fatalf("second init code=%d stderr=%s", code, errOut.String())
	}
}

func TestIndexStatusAndQueryJSON(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"index", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("index code=%d stderr=%s", code, errOut.String())
	}
	var indexResult map[string]any
	if err := json.Unmarshal(out.Bytes(), &indexResult); err != nil {
		t.Fatalf("index json=%v output=%s", err, out.String())
	}
	for _, want := range []string{"scanning repository", "checking", "parsing", "writing index", "complete"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("index stderr=%q missing %q", errOut.String(), want)
		}
	}
	out.Reset()
	if code := Run(context.Background(), []string{"status", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("status code=%d stderr=%s", code, errOut.String())
	}
	var status map[string]any
	if err := json.Unmarshal(out.Bytes(), &status); err != nil || status["file_count"] != float64(1) {
		t.Fatalf("status=%v err=%v output=%s", status, err, out.String())
	}
	out.Reset()
	if code := Run(context.Background(), []string{"query", "--root", root, "--query", "ValidateToken", "--budget", "512", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("query code=%d stderr=%s", code, errOut.String())
	}
	var query map[string]any
	if err := json.Unmarshal(out.Bytes(), &query); err != nil || query["items"] == nil || query["token_savings"] == nil {
		t.Fatalf("query=%v err=%v output=%s", query, err, out.String())
	}
}

func TestQueryDebugScoresPrintsScoreDetails(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"query", "--root", root, "--query", "ValidateToken", "--budget", "512", "--debug-scores"}, &out, &errOut); code != 0 {
		t.Fatalf("query code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "score:") || !strings.Contains(out.String(), "symbol-exact") || !strings.Contains(out.String(), "saved:") {
		t.Fatalf("debug output=%q", out.String())
	}
}

func TestQueryAcceptsAPositionalQuery(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"query", "ValidateToken", "--root", root, "--budget", "512", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "ValidateToken") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestBareQueryUsesTheSimpleCLIShortcut(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"ValidateToken", "--root", root, "--budget", "512", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "ValidateToken") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestUpdateIfRepoIsQuietOutsideGit(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"update", "--root", root, "--if-repo", "--quiet"}, &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestIndexQuietSuppressesProgressAndOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"index", "--root", root, "--quiet"}, &out, &errOut); code != 0 {
		t.Fatalf("index code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("quiet stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestUpdateReportsProgress(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"update", "--root", root}, &out, &errOut); code != 0 {
		t.Fatalf("update code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "updated") {
		t.Fatalf("update stdout=%q", out.String())
	}
	for _, want := range []string{"update: scanning repository", "update: parsing", "update: writing index", "update: complete"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("update stderr=%q missing %q", errOut.String(), want)
		}
	}
}

func TestImpactAndEvalJSONCommands(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	cases := `{"name":"token","query":"ValidateToken","token_budget":512,"expected_symbols":["ValidateToken"],"expected_paths":["auth.go"],"forbidden_paths":["other.go"]}` + "\n"
	casesPath := filepath.Join(root, "cases.jsonl")
	writeCLIFile(t, casesPath, cases)
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"impact", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("impact code=%d stderr=%s", code, errOut.String())
	}
	var impact map[string]any
	if err := json.Unmarshal(out.Bytes(), &impact); err != nil || impact["diagnostics"] == nil {
		t.Fatalf("impact=%v err=%v output=%s", impact, err, out.String())
	}
	out.Reset()
	if code := Run(context.Background(), []string{"eval", "--root", root, "--cases", casesPath, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("eval code=%d stderr=%s", code, errOut.String())
	}
	var evaluation map[string]any
	if err := json.Unmarshal(out.Bytes(), &evaluation); err != nil || evaluation["cases"] == nil {
		t.Fatalf("evaluation=%v err=%v output=%s", evaluation, err, out.String())
	}
}

func TestEvalAblationAllJSONAndRejectsUnknownMode(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	casesPath := filepath.Join(root, "cases.jsonl")
	writeCLIFile(t, casesPath, "{\"name\":\"token\",\"query\":\"ValidateToken\",\"token_budget\":512}\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"eval", "--root", root, "--cases", casesPath, "--ablation", "all", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("all code=%d stderr=%s", code, errOut.String())
	}
	var all struct {
		Reports map[string]json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(out.Bytes(), &all); err != nil || len(all.Reports) != 3 {
		t.Fatalf("all=%v err=%v output=%s", all.Reports, err, out.String())
	}
	for _, mode := range []string{"full", "fts-only", "no-relations"} {
		if _, ok := all.Reports[mode]; !ok {
			t.Fatalf("mode %q missing from %s", mode, out.String())
		}
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"eval", "--root", root, "--cases", casesPath, "--ablation", "invalid"}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "unknown ablation") {
		t.Fatalf("invalid code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestExplainJSONAndHumanOutputAreSourceFree(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return secretSourceMarker() }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"explain", "--root", root, "--query", "what calls ValidateToken?", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("explain code=%d stderr=%s", code, errOut.String())
	}
	if strings.Contains(out.String(), "secretSourceMarker") || !strings.Contains(out.String(), "\"plan\"") || !strings.Contains(out.String(), "\"candidates\"") {
		t.Fatalf("source or fields missing in explain JSON=%q", out.String())
	}
	out.Reset()
	if code := Run(context.Background(), []string{"explain", "--root", root, "--query", "ValidateToken", "--limit", "1"}, &out, &errOut); code != 0 {
		t.Fatalf("human explain code=%d stderr=%s", code, errOut.String())
	}
	for _, want := range []string{"query:", "intent:", "mode:", "final=", "fusion="} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human explain=%q missing %q", out.String(), want)
		}
	}
	if !strings.Contains(out.String(), "auth.go:3") {
		t.Fatalf("human explain=%q missing source span", out.String())
	}
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
