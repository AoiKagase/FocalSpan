package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRegistrationSpec() RegistrationSpec {
	return RegistrationSpec{
		Command:           `C:\Tools\focalspan.exe`,
		Args:              []string{"serve", "--root", `C:\repo`},
		Enabled:           true,
		StartupTimeoutSec: 30,
		ToolTimeoutSec:    60,
		EnabledTools:      append([]string(nil), EnabledTools...),
	}
}

func TestProjectInstallPreservesUnmanagedContentAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	original := "# keep this comment\n[profiles]\nname = \"test\"\n\n[mcp_servers.other]\ncommand = \"other\"\nargs = []\n"
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := testRegistrationSpec()
	first, err := InstallProject(context.Background(), configPath, "focalspan", spec, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Action != "create" {
		t.Fatalf("first action=%q", first.Action)
	}
	afterFirst, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := InstallProject(context.Background(), configPath, "focalspan", spec, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Action != "unchanged" {
		t.Fatalf("second action=%q", second.Action)
	}
	afterSecond, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterFirst) != string(afterSecond) {
		t.Fatal("idempotent install changed the file")
	}
	if !strings.HasPrefix(string(afterFirst), original) {
		t.Fatal("unmanaged prefix changed")
	}

	changed := spec
	changed.Args = append([]string(nil), spec.Args...)
	changed.Args = append(changed.Args, "--no-auto-update")
	updated, err := InstallProject(context.Background(), configPath, "focalspan", changed, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Action != "update" {
		t.Fatalf("update action=%q", updated.Action)
	}
	afterUpdate, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(afterUpdate), original) || !strings.Contains(string(afterUpdate), "--no-auto-update") {
		t.Fatal("managed update changed unmanaged content or missed new args")
	}
}

func TestProjectInstallRejectsUnmanagedCollisionAndInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	spec := testRegistrationSpec()
	if err := os.WriteFile(configPath, []byte("[mcp_servers.focalspan]\ncommand = \"someone-else\"\nargs = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(configPath)
	if _, err := InstallProject(context.Background(), configPath, "focalspan", spec, false); err == nil {
		t.Fatal("unmanaged collision was accepted")
	}
	after, _ := os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("collision changed the config")
	}
	if err := os.WriteFile(configPath, []byte("[mcp_servers.focalspan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ = os.ReadFile(configPath)
	if _, err := InstallProject(context.Background(), configPath, "focalspan", spec, false); err == nil {
		t.Fatal("invalid TOML was accepted")
	}
	after, _ = os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("invalid TOML changed the config")
	}
}

func TestProjectUninstallOnlyRemovesManagedBlock(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	spec := testRegistrationSpec()
	if _, err := InstallProject(context.Background(), configPath, "focalspan", spec, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, append([]byte("[profiles]\nname = \"keep\"\n"), func() []byte { b, _ := os.ReadFile(configPath); return b }()...), 0o600); err != nil {
		// This branch is not expected; the test below exercises the normal
		// managed-block path and keeps the setup deliberately local.
		t.Fatal(err)
	}
	// Restore a valid config with an unmanaged prefix plus the managed block.
	managed, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, append([]byte("[profiles]\nname = \"keep\"\n"), managed[bytesIndex(managed):]...), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := UninstallProject(context.Background(), configPath, "focalspan", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "removed" {
		t.Fatalf("action=%q", result.Action)
	}
	remaining, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remaining), "[profiles]") || strings.Contains(string(remaining), "FOCALSPAN MANAGED") {
		t.Fatalf("unexpected remaining config: %s", remaining)
	}
}

func bytesIndex(content []byte) int {
	const marker = "# BEGIN FOCALSPAN MANAGED MCP: focalspan"
	return strings.Index(string(content), marker)
}
