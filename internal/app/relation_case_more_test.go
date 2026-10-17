package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOtherLocalRelationsRespectCase(t *testing.T) {
	for path, source := range map[string]string{"a.cpp": "void Validate() {}\nvoid Caller() { validate(); }\n", "a.cs": "class Service { void Validate() {} void Caller() { validate(); } }\n", "a.lua": "function Validate() end\nfunction Caller() validate() end\n", "a.rb": "def Validate\nend\ndef caller\n  validate()\nend\n", "a.sma": "stock Validate() {}\npublic Caller() { validate(); }\n", "a.zig": "fn Validate() void {}\nfn Caller() void { validate(); }\n"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeAppFile(t, filepath.Join(root, path), source)
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
			rels, err := s.Store.Relations(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, rel := range rels {
				if rel.Kind == "calls" && rel.ToHandle == target {
					t.Fatalf("wrong-case call resolved: %+v", rel)
				}
			}
		})
	}
}
