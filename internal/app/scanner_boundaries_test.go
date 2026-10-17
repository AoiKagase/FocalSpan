package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexSkipsGitlinkDirectory(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	writeAppFile(t, filepath.Join(root, "a.go"), "package p\nfunc Run(){}\n")
	commitGitChangesFixture(t, root)
	oid, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	runAppGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(string(oid))+",deps/sub")
	if err := os.MkdirAll(filepath.Join(root, "deps", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
}
