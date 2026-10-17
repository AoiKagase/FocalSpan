package indexer

import (
	"context"
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
