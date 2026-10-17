package store

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestRelationUnderscoreIsLiteral(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, v := range []struct {
		path, handle, name, body string
		rels                     []model.Relation
	}{{"a.go", "target", "foo_bar", "func foo_bar(){}", nil}, {"b.go", "caller", "Caller", "func Caller(){ module.fooXbar() }", []model.Relation{{FromHandle: "caller", UnresolvedTo: "module.fooXbar", Kind: "calls", Confidence: .3}}}} {
		err = s.ReplaceFile(ctx, model.SourceFile{Path: v.path, Language: "go", SHA256: v.handle}, model.Extraction{Symbols: []model.Symbol{{Handle: v.handle, FilePath: v.path, Language: "go", Kind: "function", Name: v.name, QualifiedName: v.name, StartLine: 1, EndLine: 1}}, Chunks: []model.Chunk{{Handle: v.handle + "-chunk", FilePath: v.path, Language: "go", SymbolHandle: v.handle, SymbolName: v.name, StartLine: 1, EndLine: 1, Content: v.body, ContentHash: v.handle}}, Relations: v.rels})
		if err != nil {
			t.Fatal(err)
		}
	}
	hits, err := s.RelatedCandidateHits(ctx, []string{"target-chunk"}, "callers")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("underscore wildcard match: %+v", hits)
	}
	legacy, err := s.RelatedCandidates(ctx, []string{"target-chunk"}, "callers")
	if err != nil || len(legacy) != 0 {
		t.Fatalf("legacy wildcard match: %+v %v", legacy, err)
	}
}
