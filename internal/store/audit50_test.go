package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestF03DatabaseLeafBoundary(t *testing.T) {
	external := t.TempDir()
	db, err := Open(external, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(external, ".focalspan", "index.db")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal", ".v1.bak", ".v2.tmp", ".v1.bak-wal", ".v2.tmp-shm"} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".focalspan")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if suffix != "" {
				if err := os.WriteFile(filepath.Join(dir, "index.db"), before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			link := filepath.Join(dir, "index.db"+suffix)
			if err := os.Symlink(target, link); err != nil {
				if errors.Is(err, syscall.Errno(1314)) {
					t.Skip("Windows symlink privilege unavailable; real link acceptance runs on WSL-Ubuntu")
				}
				t.Fatal(err)
			}
			opened, err := OpenForUpdate(root, ".focalspan")
			if opened != nil {
				_ = opened.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "unsafe index file") {
				t.Errorf("linked managed file must be rejected before SQLite/recovery: %v", err)
			}
			after, readErr := os.ReadFile(target)
			if readErr != nil || !bytes.Equal(before, after) {
				t.Error("external database changed")
			}
			if _, err := os.Lstat(link); err != nil {
				t.Errorf("rejected link must not be removed: %v", err)
			}
		})
	}
	root := t.TempDir()
	normal, err := OpenForUpdate(root, ".focalspan")
	if err != nil {
		t.Fatalf("normal database rejected: %v", err)
	}
	normal.Close()
}
