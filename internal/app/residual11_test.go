package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestR01ShadowedCallsStayUnlinked(t *testing.T) {
	source, e := os.ReadFile("../../testdata/regressions/residual11/jsts-shadow.ts")
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	if e = os.WriteFile(filepath.Join(root, "a.ts"), source, 0600); e != nil {
		t.Fatal(e)
	}
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	symbols, e := s.Store.Symbols(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	for _, v := range symbols {
		names[v.Handle] = v.Name
	}
	relations, e := s.Store.Relations(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range relations {
		if names[r.FromHandle] == "caller" && r.Kind != "contains" && names[r.ToHandle] == "target" {
			t.Errorf("shadowed target resolved after linking: %+v", r)
		}
	}
}

func TestR05RelativeImportIntegration(t *testing.T) {
	source, e := os.ReadFile("../../testdata/regressions/residual11/jsts-module.ts")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ code, path string }{
		{"import './dep.ts';", "dep.ts"}, {"import './dep';", "dep.ts"}, {"import './dep';", "dep/index.ts"},
		{"const loaded = require('./dep');", "dep.ts"}, {"export * from './dep';", "dep.ts"}, {"import {work} from './dep';", "dep.ts"},
	} {
		t.Run(tc.code+tc.path, func(t *testing.T) {
			root := t.TempDir()
			if e := os.MkdirAll(filepath.Dir(filepath.Join(root, tc.path)), 0700); e != nil {
				t.Fatal(e)
			}
			writeAppFile(t, filepath.Join(root, tc.path), string(source))
			writeAppFile(t, filepath.Join(root, "main.ts"), tc.code+"\n")
			s, e := NewWithConfig(root, config.Default())
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			for _, full := range []bool{true, false} {
				if _, e = s.Index(context.Background(), full); e != nil {
					t.Fatal(e)
				}
				syms, e := s.Store.Symbols(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				byHandle := map[string]string{}
				for _, v := range syms {
					if v.Kind == "module" {
						byHandle[v.Handle] = v.FilePath
					}
				}
				rels, e := s.Store.Relations(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				found := false
				for _, r := range rels {
					if byHandle[r.FromHandle] == "main.ts" && byHandle[r.ToHandle] == tc.path && (r.Kind == "imports" || r.Kind == "exports") {
						found = true
					}
				}
				if !found {
					t.Errorf("full=%v: file import unresolved: %+v", full, rels)
				}
			}
		})
	}
}
