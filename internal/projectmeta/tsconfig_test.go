package projectmeta

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestTSConfigReadsCompilerOptionsAliases(t *testing.T) {
	text := `{"baseUrl":"legacy","paths":{"@app/*":["wrong/*"]},"compilerOptions":{"baseUrl":"../sources","paths":{"@app/*":["lib/*"]}}}`
	facts, diagnostics, err := ParseFile(context.Background(), "", model.SourceFile{Path: "configs/tsconfig.json", Content: []byte(text)})
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("%v %+v", err, diagnostics)
	}
	for _, f := range facts {
		if f.Kind == "path-alias" && f.Name == "@app/*" {
			if f.Target != "../sources/lib/*" {
				t.Fatalf("alias target=%s", f.Target)
			}
			return
		}
	}
	t.Fatalf("alias missing: %+v", facts)
}
func TestTSConfigKeepsLegacyAliases(t *testing.T) {
	facts, _, err := ParseFile(context.Background(), "", model.SourceFile{Path: "jsconfig.json", Content: []byte(`{"baseUrl":"src","paths":{"@app/*":["lib/*"]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range facts {
		if f.Kind == "path-alias" && f.Target == "src/lib/*" {
			return
		}
	}
	t.Fatalf("legacy alias missing: %+v", facts)
}
