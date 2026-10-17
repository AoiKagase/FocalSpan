package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCallResolutionRejectsOtherLanguage(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "a.go"), "package p\nfunc Caller(){ Missing() }\n")
	writeAppFile(t, filepath.Join(root, "b.py"), "def Missing():\n    pass\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	symbols, err := s.Store.Symbols(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	langs := map[string]string{}
	for _, sym := range symbols {
		langs[sym.Handle] = sym.Language
	}
	rels, err := s.Store.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rels {
		if r.Kind == "calls" && langs[r.FromHandle] == "go" && langs[r.ToHandle] == "python" {
			t.Fatalf("unrelated cross-language call resolved: %+v", r)
		}
	}
}
