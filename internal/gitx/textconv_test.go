package gitx

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGitSafetyDiffIgnoresTextconv(t *testing.T) {
	root := safetyRepository(t)
	writeTestFile(t, filepath.Join(root, ".gitattributes"), "a.txt diff=audit\n")
	// Execute an existing Git binary, never fixture/repository code.
	runGit(t, root, "config", "diff.audit.textconv", "git --version")
	writeTestFile(t, filepath.Join(root, "a.txt"), "new\n")
	files, err := NewClient(root).WorkingTreeDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path == "a.txt" && len(file.Ranges) > 0 {
			return
		}
	}
	t.Fatalf("original source change missing: %+v", files)
}
