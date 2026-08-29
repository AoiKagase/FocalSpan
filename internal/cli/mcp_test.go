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

func TestMCPProjectDryRunJSONAndPrintAreReadOnly(t *testing.T) {
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
	if code := Run(context.Background(), []string{"mcp", "install", "codex", "--root", root, "--dry-run", "--json"}, &out, &errOut); code != 0 {
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
	out.Reset()
	if code := Run(context.Background(), []string{"mcp", "print", "codex", "--root", root}, &out, &errOut); code != 0 {
		t.Fatalf("print code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "[mcp_servers.focalspan]") || !strings.Contains(out.String(), "enabled_tools") {
		t.Fatalf("print output=%s", out.String())
	}
}

func TestMCPDispatchValidationAndStdoutSeparation(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp"}, &out, &errOut); code == 0 || out.Len() != 0 {
		t.Fatalf("missing nested command code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"mcp", "install", "other"}, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "unknown MCP client") {
		t.Fatalf("unknown client code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"mcp", "status", "codex", "--scope", "bad"}, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid scope") {
		t.Fatalf("invalid scope code=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
	}
}

func TestGlobalInstallShortcutUsesCodexUserScope(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"install", "--root", root, "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["scope"] != "user" || result["action"] != "create" || result["dry_run"] != true {
		t.Fatalf("result=%v", result)
	}
}

func TestMCPGlobalFlagUsesCodexUserScope(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "codex", "--root", root, "--global", "--dry-run", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, out.String())
	}
	if result["scope"] != "user" {
		t.Fatalf("result=%v", result)
	}
}

func TestMCPDefaultsTheOnlySupportedClient(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "install", "--root", root, "--global", "--dry-run", "--json"}, &out, &errOut); code != 0 {
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
