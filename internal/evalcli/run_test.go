package evalcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAblation(t *testing.T) {
	modes, err := parseAblation("all")
	if err != nil || len(modes) != 3 {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
	if _, err := parseAblation("unknown"); err == nil {
		t.Fatal("unknown ablation accepted")
	}
}

func TestRunEvidenceContractJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := filepath.Join(root, "evidence.jsonl")
	if err := os.WriteFile(cases, []byte(`{"name":"go","query":"ValidateToken","token_budget":1200,"mode":"focused","expected":[{"path":"main.go","symbol":"ValidateToken","roles":["target"]}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Run(context.Background(), []string{"--root", root, "--cases", cases, "--contract", "compare", "--json"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), `"expected_coverage": 1`) || !strings.Contains(out.String(), `"legacy_wire_tokens"`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"--root", root, "--cases", cases, "--contract", "bad"}, &out, &errOut); code == 0 {
		t.Fatalf("invalid contract succeeded: %s", out.String())
	}
}
