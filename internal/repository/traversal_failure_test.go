package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScannerRejectsMissingRoot(t *testing.T) {
	_, _, err := NewScanner(filepath.Join(t.TempDir(), "missing"), testConfig()).Scan(context.Background())
	if err == nil {
		t.Fatal("missing root accepted as an empty successful scan")
	}
}

func TestScannerRejectsFileRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(root, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewScanner(root, testConfig()).Scan(context.Background()); err == nil {
		t.Fatal("regular-file root accepted as empty successful walk")
	}
}

func TestScannerExcludesConfiguredIndexDirectory(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig()
	cfg.IndexDirectory = ".custom-index"
	if err := os.Mkdir(filepath.Join(root, cfg.IndexDirectory), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, cfg.IndexDirectory, "state.go"), []byte("package generated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, _, err := NewScanner(root, cfg).Scan(context.Background())
	if err != nil || len(files) != 0 {
		t.Fatalf("index directory scanned: %+v err=%v", files, err)
	}
}

func TestScannerCancelsEmptyWalk(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := NewScanner(t.TempDir(), testConfig()).Scan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}
