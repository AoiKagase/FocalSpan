package gitx

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDiffDisablesConfiguredColors(t *testing.T) {
	root := safetyRepository(t)
	runGit(t, root, "config", "color.ui", "always")
	writeTestFile(t, filepath.Join(root, "a.txt"), "new\n")
	files, err := NewClient(root).WorkingTreeDiff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "a.txt" && len(f.Ranges) > 0 {
			return
		}
	}
	t.Fatalf("colored diff lost changes: %+v", files)
}
