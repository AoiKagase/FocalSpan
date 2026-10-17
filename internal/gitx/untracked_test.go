package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingTreeIncludesUntrackedSources(t *testing.T) {
	for _, unborn := range []bool{false, true} {
		t.Run(map[bool]string{false: "head", true: "unborn"}[unborn], func(t *testing.T) {
			root := t.TempDir()
			runGit(t, root, "init")
			if !unborn {
				writeTestFile(t, filepath.Join(root, "base.txt"), "base\n")
				runGit(t, root, "add", ".")
				runGit(t, root, "commit", "-m", "base")
			}
			writeTestFile(t, filepath.Join(root, "new.go"), "package p\nfunc NewFeature() {}\n")
			writeTestFile(t, filepath.Join(root, ".gitignore"), "ignored/\n")
			if err := os.Mkdir(filepath.Join(root, "ignored"), 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "ignored", "private.txt"), "dummy\n")
			files, err := NewClient(root).WorkingTreeDiff(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for _, f := range files {
				if f.Path == "new.go" && len(f.Ranges) > 0 {
					seen++
				}
				if f.Path == "ignored/private.txt" {
					t.Fatal("ignored untracked file returned")
				}
			}
			if seen != 1 {
				t.Fatalf("new.go occurrences=%d files=%+v", seen, files)
			}
		})
	}
}
