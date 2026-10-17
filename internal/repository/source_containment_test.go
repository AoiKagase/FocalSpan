package repository

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceContainmentAllowsInsideJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction")
	}
	root := t.TempDir()
	target := filepath.Join(root, "actual")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "source.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junction: %v %s", err, out)
	}
	defer os.Remove(link)
	got, err := ContainedPath(root, filepath.Join(link, "source.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(target, "source.go") {
		t.Fatalf("path=%s", got)
	}
}

func TestAuditSourceJunctionContainment(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction")
	}
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.go"), []byte("package private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "source")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("junction: %v %s", err, out)
	}
	defer os.Remove(link)
	path, err := ContainedPath(root, filepath.Join(link, "private.go"))
	if err == nil {
		t.Fatalf("external source accepted through junction: %s", path)
	}
}

func TestAuditScannerMustNotReadOutsideJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction")
	}
	root, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(root, "source")
	if err := os.Mkdir(link, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(link, "private.go")
	if err := os.WriteFile(original, []byte("package original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "source/private.go"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	content := []byte("package private\nconst Secret = \"outside-source\"\n")
	if err := os.WriteFile(filepath.Join(outside, "private.go"), content, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("junction: %v %s", err, out)
	}
	defer os.Remove(link)
	files, _, err := NewScanner(root, config.Default()).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if string(file.Content) == string(content) {
			t.Fatalf("scanner read external source as repository file: %s", file.Path)
		}
	}
}
