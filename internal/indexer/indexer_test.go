package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/store"
)

func TestIndexerIncrementalStatsAndDeletion(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() {}\n")
	write(t, filepath.Join(root, "README.md"), "# Auth\n\nexpired token\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor(), generic.NewExtractor()))
	first, err := ix.Run(context.Background(), true)
	if err != nil || first.FilesSeen != 2 || first.FilesAdded != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := ix.Run(context.Background(), false)
	if err != nil || second.FilesUnchanged != 2 || second.FilesChanged != 0 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	third, err := ix.Run(context.Background(), false)
	if err != nil || third.FilesChanged != 1 || third.FilesUnchanged != 1 {
		t.Fatalf("third=%+v err=%v", third, err)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	fourth, err := ix.Run(context.Background(), false)
	if err != nil || fourth.FilesDeleted != 1 {
		t.Fatalf("fourth=%+v err=%v", fourth, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
