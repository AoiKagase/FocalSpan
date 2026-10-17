package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectOperationsHonorCancelledContext(t *testing.T) {
	for _, uninstall := range []bool{false, true} {
		for _, dry := range []bool{false, true} {
			t.Run(map[bool]string{false: "install", true: "uninstall"}[uninstall]+map[bool]string{false: " write", true: " dry"}[dry], func(t *testing.T) {
				path := filepath.Join(t.TempDir(), ".codex", "config.toml")
				var before []byte
				if uninstall {
					if _, err := InstallProject(context.Background(), path, "focalspan", testRegistrationSpec(), false); err != nil {
						t.Fatal(err)
					}
					before, _ = os.ReadFile(path)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				var err error
				if uninstall {
					_, err = UninstallProject(ctx, path, "focalspan", dry)
				} else {
					_, err = InstallProject(ctx, path, "focalspan", testRegistrationSpec(), dry)
				}
				if !errors.Is(err, context.Canceled) {
					t.Errorf("cancellation ignored: %v", err)
				}
				after, readErr := os.ReadFile(path)
				if uninstall {
					if readErr != nil || string(before) != string(after) {
						t.Fatalf("cancelled uninstall changed config: %v", readErr)
					}
				} else if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatal("cancelled install created config")
				}
			})
		}
	}
}

type cancelWhenTempExists struct {
	context.Context
	dir    string
	cancel context.CancelFunc
}

func (c cancelWhenTempExists) Err() error {
	matches, _ := filepath.Glob(filepath.Join(c.dir, ".config.toml.focalspan-*"))
	if len(matches) > 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestAtomicWriteCancellationBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("KEEP"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = atomicWrite(cancelWhenTempExists{ctx, dir, cancel}, path, []byte("REPLACE"), 0600)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-commit cancellation ignored: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "KEEP" {
		t.Fatalf("original changed: %q %v", got, err)
	}
	after, err := os.Stat(path)
	if err != nil || after.Mode() != before.Mode() {
		t.Fatalf("file mode changed: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".config.toml.focalspan-*"))
	if len(leftovers) > 0 {
		t.Fatalf("owned temp left: %v", leftovers)
	}
}
