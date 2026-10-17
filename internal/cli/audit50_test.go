package cli

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/config"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestF12EffectiveGitignoreRule(t *testing.T) {
	for _, old := range []string{"# .focalspan/\n", "!.focalspan/\n", "nested/.focalspan/\n", "/.focalspan/\n!/.focalspan/\n"} {
		t.Run(old, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, ".gitignore")
			os.WriteFile(p, []byte(old), 0600)
			if err := ensureGitignore(root); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(p)
			if !strings.HasSuffix(string(b), "/.focalspan/\n") {
				t.Fatalf("no final root rule: %q", b)
			}
			if err := ensureGitignore(root); err != nil {
				t.Fatal(err)
			}
			again, _ := os.ReadFile(p)
			if string(again) != string(b) {
				t.Fatal("not idempotent")
			}
		})
	}
}
func TestF13RejectUnexpectedArguments(t *testing.T) {
	for _, cmd := range []string{"update", "serve"} {
		t.Run(cmd, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"--root", root, "typo"}
			var err error
			if cmd == "update" {
				err = runUpdate(context.Background(), args, io.Discard, io.Discard)
			} else {
				err = runServe(context.Background(), append([]string{"--worker"}, args...))
			}
			if err == nil || !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("error=%v", err)
			}
			if _, e := os.Stat(filepath.Join(root, ".focalspan")); !os.IsNotExist(e) {
				t.Fatal("created index")
			}
		})
	}
}

func TestF12ConfiguredGitignoreRule(t *testing.T) {
	for _, dir := range []string{"index data", "cache[1]*?"} {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			cmd := exec.Command("git", "init", root)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			cfg := config.Default()
			cfg.IndexDirectory = dir
			b, _ := json.Marshal(cfg)
			os.WriteFile(filepath.Join(root, config.FileName), b, 0600)
			os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# old\r\n"), 0600)
			if err := ensureGitignore(root); err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{"index.db", "index.lock"} {
				cmd := exec.Command("git", "-C", root, "check-ignore", "--", filepath.ToSlash(filepath.Join(dir, file)))
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("not ignored %s: %v %s", file, err, out)
				}
			}
		})
	}
}

func TestF13RejectLeadingTypoBeforeRoot(t *testing.T) {
	for _, command := range []string{"update", "serve"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"typo", "--root", root}
			var err error
			if command == "update" {
				err = runUpdate(context.Background(), args, io.Discard, io.Discard)
			} else {
				err = runServe(context.Background(), args)
			}
			if err == nil || !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("error=%v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".focalspan")); !os.IsNotExist(err) {
				t.Fatal("created index before rejecting arguments")
			}
		})
	}
}
