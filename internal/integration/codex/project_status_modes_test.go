package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectStatusDetectsAutoUpdateDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex", "config.toml")
	expected := testRegistrationSpec()
	expected.Args = []string{"serve", "--root", root}
	actual := expected.clone()
	actual.Args = append(actual.Args, "--no-auto-update")
	if _, err := InstallProject(context.Background(), path, "focalspan", actual, false); err != nil {
		t.Fatal(err)
	}
	status := StatusProject(path, root, "focalspan", expected)
	if status.Matches || status.State != StateManagedDrift {
		t.Fatalf("auto-update drift hidden: %+v", status)
	}
	if status = StatusProject(path, root, "focalspan", actual); !status.Matches {
		t.Fatalf("matching no-auto spec rejected: %+v", status)
	}
}
func TestProjectStatusUnmanagedMatchAndConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex", "config.toml")
	spec := testRegistrationSpec()
	spec.Args = []string{"serve", "--root", root}
	if _, err := InstallProject(context.Background(), path, "focalspan", spec, false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(b), managedBeginMarker("focalspan"), ""), managedEndMarker("focalspan"), "")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	status := StatusProject(path, root, "focalspan", spec)
	if status.Managed || !status.Matches || status.State != StateUnmanagedMatch {
		t.Fatalf("unmanaged match rejected: %+v", status)
	}
	text = strings.Replace(text, "enabled = true", "enabled = false", 1)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	status = StatusProject(path, root, "focalspan", spec)
	if status.Managed || status.Matches || status.State != StateConflict {
		t.Fatalf("unmanaged conflict hidden: %+v", status)
	}
}
