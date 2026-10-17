package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMapRangesFollowsGitRenameAndCurrentDeletion(t *testing.T) {
	root := safetyRepository(t)
	runGit(t, root, "mv", "a.txt", "renamed.txt")
	c := NewClient(root)
	ranges, err := c.MapRangesToWorktree(context.Background(), "HEAD", []ChangedFile{{Path: "a.txt", Ranges: []LineRange{{1, 1}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].Path != "renamed.txt" || len(ranges[0].Ranges) != 1 {
		t.Fatalf("rename mapping=%+v", ranges)
	}
	if err := os.Remove(filepath.Join(root, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	ranges, err = c.MapRangesToWorktree(context.Background(), "HEAD", []ChangedFile{{Path: "a.txt", Ranges: []LineRange{{1, 1}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 0 {
		t.Fatalf("currently deleted source retained: %+v", ranges)
	}
}
func TestSingleRefImpactAlreadyUsesCurrentLines(t *testing.T) {
	root := safetyRepository(t)
	writeTestFile(t, filepath.Join(root, "a.txt"), "prefix\nold\n")
	for _, req := range []DiffRequest{{Base: "HEAD"}, {Head: "HEAD"}} {
		raw, err := NewClient(root).Diff(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		impact, err := NewClient(root).DiffForImpact(context.Background(), req.Base, req.Head)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != 1 || len(impact) != 1 || raw[0].Ranges[0] != impact[0].Ranges[0] {
			t.Fatalf("single-ref was remapped: raw=%+v impact=%+v", raw, impact)
		}
	}
}
