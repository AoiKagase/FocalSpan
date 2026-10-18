package linker

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestCS03LinkerPreservesCSharpReceiverScope(t *testing.T) {
	for _, tc := range []struct{ name, call, target, want string }{
		{"bare foreign", "Save();", "class Target { public static void Save() {} }", ""},
		{"unknown receiver", "foreign.Save();", "class Target { public static void Save() {} }", ""},
		{"conditional receiver", "foreign?.Save();", "class Target { public static void Save() {} }", ""},
		{"type receiver", "Target.Save();", "class Target { public static void Save() {} }", "Demo.Target.Save"},
		{"qualified receiver", "Demo.Target.Save();", "class Target { public static void Save() {} }", "Demo.Target.Save"},
		{"partial this", "this.Save();", "partial class Caller { void Save() {} }", "Demo.Caller.Save"},
		{"partial bare", "Save();", "partial class Caller { void Save() {} }", "Demo.Caller.Save"},
		{"ambiguous overload", "Target.Save(1);", "class Target { public static void Save(int x) {} public static void Save(string x) {} }", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := openLinkerStore(t)
			var from string
			for _, f := range []struct{ path, source string }{{"Caller.cs", "namespace Demo; partial class Caller { void Run() { " + tc.call + " } }"}, {"Target.cs", "namespace Demo; " + tc.target}} {
				file := model.SourceFile{Path: f.path, Language: "csharp", Content: []byte(f.source), SHA256: f.path}
				extraction, err := csharp.NewExtractor().Extract(ctx, file)
				if err != nil {
					t.Fatal(err)
				}
				for _, sym := range extraction.Symbols {
					if sym.QualifiedName == "Demo.Caller.Run" {
						from = sym.Handle
					}
				}
				if err := s.ReplaceFile(ctx, file, extraction); err != nil {
					t.Fatal(err)
				}
			}
			if err := (&Linker{Store: s}).Link(ctx, nil); err != nil {
				t.Fatal(err)
			}
			relations, err := s.Relations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			symbols, err := s.Symbols(ctx)
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]string{}
			for _, sym := range symbols {
				names[sym.Handle] = sym.QualifiedName
			}
			count := 0
			for _, r := range relations {
				if r.FromHandle == from && r.Kind == "calls" {
					count++
					if tc.want == "" {
						if r.ToHandle != "" || r.UnresolvedTo == "" {
							t.Errorf("unexpected resolution: %+v", r)
						}
					} else if r.ToHandle == "" || names[r.ToHandle] != tc.want {
						t.Errorf("relation=%+v want %s", r, tc.want)
					}
				}
			}
			if count != 1 {
				t.Errorf("calls=%d, want 1", count)
			}
		})
	}
}
