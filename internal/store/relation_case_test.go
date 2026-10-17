package store

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestRelationCaseMatchesLanguage(t *testing.T) {
	for _, lang := range []string{"go", "vbnet"} {
		t.Run(lang, func(t *testing.T) {
			s := openTestStore(t)
			for _, v := range []struct {
				path, handle, name string
				rels               []model.Relation
			}{{"defs", "target", "Validate", nil}, {"caller", "caller", "Caller", []model.Relation{{FromHandle: "caller", UnresolvedTo: "validate", Kind: "calls", Confidence: .5}}}} {
				err := s.ReplaceFile(context.Background(), model.SourceFile{Path: v.path, Language: lang, SHA256: v.handle}, model.Extraction{Symbols: []model.Symbol{{Handle: v.handle, FilePath: v.path, Language: lang, Kind: "function", Name: v.name, QualifiedName: v.name, StartLine: 1, EndLine: 1}}, Chunks: []model.Chunk{{Handle: v.handle + "-chunk", FilePath: v.path, Language: lang, SymbolHandle: v.handle, SymbolName: v.name, StartLine: 1, EndLine: 1, Content: v.name, ContentHash: v.name}}, Relations: v.rels})
				if err != nil {
					t.Fatal(err)
				}
			}
			hits, err := s.RelatedCandidateHits(context.Background(), []string{"target-chunk"}, "callers")
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if lang == "vbnet" {
				want = 1
			}
			if len(hits) != want {
				t.Fatalf("hits=%+v want=%d", hits, want)
			}
		})
	}
}
