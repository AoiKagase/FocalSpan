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

func TestUpdateIfRepoIsQuietOutsideGit(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"update", "--root", root, "--if-repo", "--quiet"}, &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
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

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
