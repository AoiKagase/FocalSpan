package gitx

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDiffPinsPathPrefixes(t *testing.T) {
	for _, option := range []string{"diff.noprefix", "diff.mnemonicprefix"} {
		t.Run(option, func(t *testing.T) {
			root := safetyRepository(t)
			runGit(t, root, "config", option, "true")
			runGit(t, root, "config", "color.ui", "always")
			writeTestFile(t, filepath.Join(root, "a.txt"), "new\n")
			client := NewClient(root)
			for _, staged := range []bool{false, true} {
				var files []ChangedFile
				var err error
				if staged {
					runGit(t, root, "add", "a.txt")
					files, err = client.Diff(context.Background(), DiffRequest{Staged: true})
				} else {
					files, err = client.WorkingTreeDiff(context.Background())
				}
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, f := range files {
					if f.Path == "a.txt" && len(f.Ranges) > 0 {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing change: %+v", files)
				}
			}
		})
	}
}
