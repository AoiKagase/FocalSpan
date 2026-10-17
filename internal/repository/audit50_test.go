package repository

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestF02SecretAliasExcluded(t *testing.T) {
	root := t.TempDir()
	secret, err := os.ReadFile(filepath.Join("..", "..", "testdata", "audit50", "F02", "secret.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), secret, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real.go"), []byte("package p\nfunc Real(){}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for alias, target := range map[string]string{"public.txt": ".env", "normal.go": "real.go"} {
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(root, alias)); err != nil {
			if errors.Is(err, syscall.Errno(1314)) {
				t.Skip("Windows symlink privilege missing; required acceptance pending")
			}
			t.Fatalf("required symlink acceptance unavailable: %v", err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "public.txt", "normal.go", "real.go"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	for _, tc := range []struct {
		name       string
		include    []string
		wantSecret bool
	}{
		{"default", nil, false}, {"alias include", []string{"public.txt"}, false}, {"physical include", []string{".env"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Include = tc.include
			files, _, err := NewScanner(root, cfg).Scan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			secretFound, normalFound := false, false
			for _, f := range files {
				if f.Path == "public.txt" {
					secretFound = true
				}
				if f.Path == "normal.go" {
					normalFound = true
				}
			}
			if secretFound != tc.wantSecret {
				t.Errorf("secret alias included=%v want %v", secretFound, tc.wantSecret)
			}
			if !normalFound {
				t.Error("ordinary inside-root alias was lost")
			}
		})
	}
}
