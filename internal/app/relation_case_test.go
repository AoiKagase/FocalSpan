package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLocalRelationsRespectCase(t *testing.T) {
	for _, tc := range []struct{ path, source string }{{"a.go", "package p\nfunc Validate(){}\nfunc Caller(){validate()}\n"}, {"a.js", "function Validate() {}\nfunction Caller() { validate(); }\n"}, {"a.py", "def Validate():\n    pass\ndef Caller():\n    validate()\n"}, {"a.rs", "fn Validate() {}\nfn Caller() { validate(); }\n"}} {
		t.Run(tc.path, func(t *testing.T) {
			root := t.TempDir()
			writeAppFile(t, filepath.Join(root, tc.path), tc.source)
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
			target := ""
			for _, sym := range symbols {
				if sym.Name == "Validate" {
					target = sym.Handle
				}
			}
			if target == "" {
				t.Fatal("target missing")
			}
			relations, err := s.Store.Relations(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, rel := range relations {
				if rel.Kind == "calls" && rel.ToHandle == target {
					t.Fatalf("wrong-case call resolved: %+v", rel)
				}
			}
			hits, err := s.Store.RelatedCandidateHits(context.Background(), []string{target}, "callers")
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 0 {
				t.Fatalf("wrong-case lexical call returned: %+v", hits)
			}
		})
	}
}
