package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCallResolutionDoesNotPreferFileBasename(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "target.go"), "package p\nfunc Target() int {return 1}\n")
	writeAppFile(t, filepath.Join(root, "caller.go"), "package p\nfunc Caller() int {return Target()}\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	rels, err := s.Store.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rels {
		if r.Kind == "calls" && r.ToHandle != "" {
			return
		}
	}
	t.Fatalf("unique Target unresolved in target.go: %+v", rels)
}
