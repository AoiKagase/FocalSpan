package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"path/filepath"
	"reflect"
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

func TestR04AliasScopeIntegration(t *testing.T) {
	cfgA := `{"compilerOptions":{"paths":{"@dep":["dep.ts"]}}}`
	rootCfg := `{"compilerOptions":{"paths":{"@dep":["rootdep.ts"]}}}`
	for _, tc := range []struct {
		name, main, want string
		extra            map[string]string
	}{
		{"sibling", "packages/b/main.ts", "", nil},
		{"same", "packages/a/main.ts", "packages/a/dep.ts", nil},
		{"single-letter-directory", "z/main.ts", "z/dep.ts", map[string]string{"tsconfig.json": rootCfg, "rootdep.ts": "// root", "z/tsconfig.json": cfgA, "z/dep.ts": "// inner"}},
		{"nearest", "packages/b/main.ts", "packages/b/dep.ts", map[string]string{"tsconfig.json": rootCfg, "rootdep.ts": "// root", "packages/b/tsconfig.json": cfgA, "packages/b/dep.ts": "// local"}},
		{"empty-boundary", "packages/b/main.ts", "", map[string]string{"tsconfig.json": rootCfg, "rootdep.ts": "// root", "packages/b/tsconfig.json": "{}"}},
		{"invalid-boundary", "packages/b/main.ts", "", map[string]string{"tsconfig.json": rootCfg, "rootdep.ts": "// root", "packages/b/tsconfig.json": "{"}},
		{"ts-before-js", "packages/b/main.ts", "packages/b/dep.ts", map[string]string{"packages/b/tsconfig.json": cfgA, "packages/b/dep.ts": "// ts", "packages/b/jsconfig.json": `{"compilerOptions":{"paths":{"@dep":["jsdep.ts"]}}}`, "packages/b/jsdep.ts": "// js"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{"packages/a/tsconfig.json": cfgA, "packages/a/dep.ts": "// side effect module", tc.main: "import '@dep';\n"}
			for p, v := range tc.extra {
				files[p] = v
			}
			for p, v := range files {
				if e := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); e != nil {
					t.Fatal(e)
				}
				writeAppFile(t, filepath.Join(root, p), v)
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
			paths := map[string]string{}
			for _, v := range symbols {
				paths[v.Handle] = v.FilePath
			}
			rels, e := s.Store.Relations(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			targets := map[string]bool{}
			for _, r := range rels {
				if paths[r.FromHandle] == tc.main && r.Kind == "imports" && r.ToHandle != "" {
					targets[paths[r.ToHandle]] = true
				}
			}
			if tc.want == "" && len(targets) != 0 || tc.want != "" && (!targets[tc.want] || len(targets) != 1) {
				t.Errorf("targets=%v want %q", targets, tc.want)
			}
		})
	}
}

func TestR04AliasManifestIncremental(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "main.ts"), "import '@dep';\n")
	writeAppFile(t, filepath.Join(root, "a.ts"), "// a")
	writeAppFile(t, filepath.Join(root, "b.ts"), "// b")
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, target := range []string{"a.ts", "b.ts", "", "a.ts"} {
		if target == "" {
			if e := os.Remove(filepath.Join(root, "tsconfig.json")); e != nil {
				t.Fatal(e)
			}
		} else {
			writeAppFile(t, filepath.Join(root, "tsconfig.json"), `{"compilerOptions":{"paths":{"@dep":["`+target+`"]}}}`)
		}
		if _, e = s.Index(context.Background(), false); e != nil {
			t.Fatal(e)
		}
		before, e := s.Store.Relations(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.Index(context.Background(), true); e != nil {
			t.Fatal(e)
		}
		after, e := s.Store.Relations(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(before, after) {
			t.Errorf("target=%s: incremental differs from full", target)
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
