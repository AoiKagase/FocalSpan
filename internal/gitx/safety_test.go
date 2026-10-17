package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func safetyRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	writeTestFile(t, filepath.Join(root, "a.txt"), "old\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "base")
	return root
}

func TestGitSafetyRefCannotWriteOutputFile(t *testing.T) {
	root := safetyRepository(t)
	writeTestFile(t, filepath.Join(root, "a.txt"), "new\n")
	victim := filepath.Join(t.TempDir(), "victim.txt")
	writeTestFile(t, victim, "KEEP")
	_, err := NewClient(root).Diff(context.Background(), DiffRequest{Base: "--output=" + filepath.ToSlash(victim)})
	after, readErr := os.ReadFile(victim)
	if readErr != nil || string(after) != "KEEP" || err == nil {
		t.Fatalf("option-shaped ref: err=%v read=%v content=%q", err, readErr, after)
	}
}

func TestGitSafetyRefValidationAndTags(t *testing.T) {
	root := safetyRepository(t)
	runGit(t, root, "tag", "-a", "release", "-m", "tag")
	writeTestFile(t, filepath.Join(root, "a.txt"), "new\n")
	for _, ref := range []string{"HEAD", "release"} {
		files, err := NewClient(root).Diff(context.Background(), DiffRequest{Base: ref})
		if err != nil || len(files) != 1 {
			t.Fatalf("ref=%q files=%+v err=%v", ref, files, err)
		}
	}
	for _, ref := range []string{"--stat", "--output=ignored", "HEAD\x00", "missing-ref"} {
		if _, err := NewClient(root).Diff(context.Background(), DiffRequest{Head: ref}); err == nil {
			t.Fatalf("accepted ref %q", ref)
		}
	}
}
