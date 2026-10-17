package indexer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/store"
)

func TestTraversalFailurePreservesIndexAndCompletion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() {}\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor()))
	if _, err = ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, key := range []string{"last_revision", "last_successful_index", "linking_pending"} {
		before[key], err = s.Meta(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Model a scan root becoming unavailable while the already-open store remains usable.
	ix.root = filepath.Join(root, "missing")
	if _, err = ix.Run(ctx, false); err == nil {
		t.Fatal("unavailable scan root accepted")
	}
	if _, found, err := s.FileHash(ctx, "auth.go"); err != nil || !found {
		t.Fatalf("old source lost: found=%v err=%v", found, err)
	}
	for key, want := range before {
		if got, err := s.Meta(ctx, key); err != nil || got != want {
			t.Fatalf("%s changed: got=%q want=%q err=%v", key, got, want, err)
		}
	}
}

func TestFileReadFailurePreservesIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "auth.go")
	write(t, path, "package auth\nfunc ValidateToken() {}\n")
	for _, args := range [][]string{{"init"}, {"add", "auth.go"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor()))
	if _, err = ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	revision, _ := s.Meta(ctx, "last_revision")
	completed, _ := s.Meta(ctx, "last_successful_index")
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = ix.Run(ctx, false); err == nil {
		t.Fatal("unreadable tracked source accepted")
	}
	if _, found, err := s.FileHash(ctx, "auth.go"); err != nil || !found {
		t.Fatalf("source lost: found=%v err=%v", found, err)
	}
	if got, _ := s.Meta(ctx, "last_revision"); got != revision {
		t.Fatal("revision changed after read failure")
	}
	if got, _ := s.Meta(ctx, "last_successful_index"); got != completed {
		t.Fatal("completion changed after read failure")
	}
}
