package projectmeta

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestR04ApplicableAliases(t *testing.T) {
	for _, body := range []string{`{}`, `{"compilerOptions":{"paths":{"@dep":["dep.ts"]}}}`, `{`} {
		facts, diag, e := ParseFile(context.Background(), "", model.SourceFile{Path: "nested/tsconfig.json", Content: []byte(body)})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, f := range facts {
			if f.Kind == "project-config" && f.SourcePath == "nested/tsconfig.json" {
				found = true
			}
		}
		if !found {
			t.Errorf("configuration boundary missing: %q %+v", body, facts)
		}
		if body == "{" && len(diag) == 0 {
			t.Error("malformed config lost diagnostic")
		}
	}
}
