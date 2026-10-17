package goast

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestAuditGoRawStringImport(t *testing.T) {
	e, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "p.go", Language: "go", Content: []byte("package p\nimport `fmt`\nfunc Print(){fmt.Println(1)}\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range e.Relations {
		if r.Kind == "imports" {
			if r.UnresolvedTo != "fmt" {
				t.Fatalf("raw-string import not unquoted: %+v", r)
			}
			return
		}
	}
	t.Fatal("import missing")
}
