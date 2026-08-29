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

func TestMCPProjectDryRunJSONIsReadOnly(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	original := "# existing\n[profiles]\nname = \"safe\"\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "--project", "--root", root, "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["action"] != "create" || result["scope"] != "project" {
		t.Fatalf("result=%v", result)
	}
	unchanged, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != original {
		t.Fatal("dry-run changed project config")
	}
}

func TestMCPDispatchValidationAndStdoutSeparation(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp"}, &out, &errOut); code == 0 || out.Len() != 0 {
		t.Fatalf("missing nested command code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"mcp", "install", "codex"}, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "unexpected mcp argument") {
		t.Fatalf("client positional code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"mcp", "status", "--scope", "bad"}, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "flag provided but not defined") {
		t.Fatalf("retired scope code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
}

func TestRetiredGlobalInstallShortcutIsRejected(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"install"}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestMCPProjectFlagUsesProjectScope(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "--project", "--root", root, "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["scope"] != "project" {
		t.Fatalf("result=%v", result)
	}
}

func TestMCPInstallDefaultsToGlobalCodexScope(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "--root", root, "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["client"] != "codex" || result["scope"] != "user" {
		t.Fatalf("result=%v", result)
	}
}

func TestMCPRejectsScopeSpecificAndRetiredFlags(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"mcp", "install", "--project", "--root", root, "--codex", "codex", "--dry-run"},
		{"mcp", "install", "--project", "--root", root, "--force", "--dry-run"},
		{"mcp", "install", "--root", root, "--global", "--dry-run"},
		{"mcp", "status", "--root", root, "--auto-update=false"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code == 0 {
			t.Fatalf("args=%v unexpectedly succeeded: %s", args, out.String())
		}
	}
}
