package projectmeta

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestMetadataBOMPreservesFirstDeclaration(t *testing.T) {
	for _, tc := range []struct{ path, source, kind, name string }{
		{"package.json", `{"name":"auth-app"}`, "package", "auth-app"},
		{"go.mod", "module example.com/auth\n", "module", "example.com/auth"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			facts, diagnostics, err := Discover(context.Background(), t.TempDir(), []model.SourceFile{{Path: tc.path, Content: []byte("\xef\xbb\xbf" + tc.source)}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("metadata diagnostics=%+v err=%v", diagnostics, err)
			}
			for _, fact := range facts {
				if fact.Kind == tc.kind && fact.Target == tc.name && fact.SourcePath == tc.path {
					return
				}
			}
			t.Fatalf("BOM lost first manifest declaration: %+v", facts)
		})
	}
}
