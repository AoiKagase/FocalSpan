package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
)

func TestGitSafetyProbeFailureDoesNotWalkIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "private"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("private/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private", "customer.txt"), []byte("dummy private data"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	files, _, err := NewScanner(root, config.Default()).Scan(context.Background())
	if err == nil {
		t.Fatalf("unavailable Git was treated as non-Git: files=%+v", files)
	}
}

func TestGitSafetyNonGitWithoutExecutable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	found, err := ProbeGitRepository(context.Background(), root)
	if err != nil || found {
		t.Fatalf("non-Git directory: found=%t err=%v", found, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ProbeGitRepository(ctx, root); err != context.Canceled {
		t.Fatalf("cancel error=%v", err)
	}
}

func TestGitSafetyBrokenWorktreeMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: nonexistent\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeGitRepository(context.Background(), root); err == nil {
		t.Fatal("broken worktree marker accepted")
	}
}
