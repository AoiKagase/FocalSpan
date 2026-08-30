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
	if result["root"] != root || result["root_mode"] != nil {
		t.Fatalf("project root contract=%v", result)
	}
	args, ok := result["args"].([]any)
	if !ok || len(args) < 3 || args[0] != "serve" || args[1] != "--root" || args[2] != root {
		t.Fatalf("project args=%v", result["args"])
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
	if result["name"] != "focalspan" || result["root_mode"] != "runtime_cwd" {
		t.Fatalf("result=%v", result)
	}
	if _, found := result["root"]; found {
		t.Fatalf("global result exposed root: %v", result)
	}
	args, ok := result["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "serve" {
		t.Fatalf("global args=%v", result["args"])
	}
	if strings.Contains(out.String(), root) {
		t.Fatalf("global dry-run exposed root %q: %s", root, out.String())
	}
}

func TestMCPGlobalDryRunNeedsNoRoot(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["name"] != "focalspan" || result["root_mode"] != "runtime_cwd" || result["root"] != nil {
		t.Fatalf("result=%v", result)
	}
	if diagnostics, found := result["diagnostics"]; found {
		encoded, _ := json.Marshal(diagnostics)
		if strings.Contains(strings.ToLower(string(encoded)), "legacy") {
			t.Fatalf("unexpected legacy migration: %v", diagnostics)
		}
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
