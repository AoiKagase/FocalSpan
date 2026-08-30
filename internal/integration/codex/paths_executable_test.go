package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultServerNameUsesOneGlobalNameAndKeepsLegacyRootNames(t *testing.T) {
	left := filepath.Join(t.TempDir(), "Book Stack")
	right := filepath.Join(t.TempDir(), "Book Stack")
	if got := DefaultServerName(ScopeUser, left); got != "focalspan" {
		t.Fatalf("user name=%q, want focalspan", got)
	}
	if got := DefaultServerName(ScopeUser, right); got != "focalspan" {
		t.Fatalf("second user name=%q, want focalspan", got)
	}
	first := LegacyUserServerName(left)
	if first == LegacyUserServerName(right) {
		t.Fatal("different roots produced the same legacy name")
	}
	if len(first) > 64 || !validName.MatchString(first) || !strings.HasPrefix(first, "focalspan-") {
		t.Fatalf("invalid legacy name %q", first)
	}
	if DefaultServerName(ScopeProject, left) != "focalspan" {
		t.Fatal("project default name changed")
	}
}

func TestResolveExecutableChecksRegularPersistentFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "focalspan.exe")
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, warning, err := ResolveExecutable(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != path || warning != "" {
		t.Fatalf("got=%q warning=%q", got, warning)
	}
	if _, _, err := ResolveExecutable(dir, false); err == nil {
		t.Fatal("directory was accepted as executable")
	}
}

func TestResolveExecutableRejectsGoBuildPathUnlessDryRun(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "go-build-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "focalspan")
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveExecutable(path, false); err == nil {
		t.Fatal("go-build executable was accepted for permanent install")
	}
	if _, warning, err := ResolveExecutable(path, true); err != nil || warning == "" {
		t.Fatalf("dry-run path err=%v warning=%q", err, warning)
	}
}
