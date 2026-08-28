package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientListsTrackedAndUntrackedNonIgnored(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	writeTestFile(t, filepath.Join(root, "tracked.go"), "package p")
	runGit(t, root, "add", "tracked.go")
	writeTestFile(t, filepath.Join(root, "new.go"), "package p")
	writeTestFile(t, filepath.Join(root, "ignored.tmp"), "ignored")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "*.tmp\n")
	files, err := NewClient(root).ListFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(files, ",")
	if !strings.Contains(joined, ".gitignore") || !strings.Contains(joined, "new.go") || !strings.Contains(joined, "tracked.go") || strings.Contains(joined, "ignored.tmp") {
		t.Fatalf("files=%v", files)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=focalspan", "GIT_AUTHOR_EMAIL=focalspan@example.invalid", "GIT_COMMITTER_NAME=focalspan", "GIT_COMMITTER_EMAIL=focalspan@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
