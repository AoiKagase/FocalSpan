package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestF11LanguageOverrideDispatch(t *testing.T) {
	registry := newExtractorRegistry()
	for _, tc := range []struct{ path, lang, content, want, name string }{
		{"logic.go", "python", "def real():\n    pass\n", "python-structural", "real"},
		{"logic.cs", "text", "ordinary text\n", "generic-structural", ""},
		{"logic.inc", "text", "ordinary text\n", "generic-structural", ""},
		{"logic.other", "nim", "proc real() = discard\n", "nim-structural", "real"},
		{"logic.other", "zig", "fn real() void {}\n", "zig-structural", "real"},
		{"logic.other", "dotnet-resource", "<root><data name=\"Real\"/></root>", "resx-structural", "Real"},
		{"logic.go", "", "package p\nfunc Real(){}\n", "go-ast", "Real"},
	} {
		t.Run(tc.path+"_"+tc.lang, func(t *testing.T) {
			e, ok := registry.For(tc.path, tc.lang)
			if !ok || e.Name() != tc.want {
				t.Fatalf("extractor %v ok=%v want %s", e, ok, tc.want)
			}
			got, err := registry.Extract(context.Background(), model.SourceFile{Path: tc.path, Language: tc.lang, Content: []byte(tc.content)})
			if err != nil {
				t.Fatal(err)
			}
			if tc.name != "" {
				found := false
				for _, s := range got.Symbols {
					found = found || s.Name == tc.name
				}
				if !found {
					t.Fatalf("expected declaration %s: %+v", tc.name, got.Symbols)
				}
			}
		})
	}
}
