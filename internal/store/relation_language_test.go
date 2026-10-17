package store

import (
	"context"
	"fmt"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestRelationLanguageFilterBeforeCandidateLimit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	seed := func(path, handle, name, lang string, rels []model.Relation) {
		t.Helper()
		err := s.ReplaceFile(ctx, model.SourceFile{Path: path, Language: lang, SHA256: handle}, model.Extraction{Symbols: []model.Symbol{{Handle: handle, FilePath: path, Language: lang, Kind: "function", Name: name, QualifiedName: name, StartLine: 1, EndLine: 1}}, Chunks: []model.Chunk{{Handle: handle + "-chunk", FilePath: path, Language: lang, SymbolHandle: handle, SymbolName: name, StartLine: 1, EndLine: 1, Content: name, ContentHash: handle}}, Relations: rels})
		if err != nil {
			t.Fatal(err)
		}
	}
	seed("target.go", "target", "Target", "go", nil)
	for i := 0; i < 131; i++ {
		lang := "python"
		confidence := .9
		if i == 130 {
			lang = "go"
			confidence = .3
		}
		handle := fmt.Sprintf("caller%03d", i)
		seed(handle, handle, handle, lang, []model.Relation{{FromHandle: handle, UnresolvedTo: "Target", Kind: "calls", Confidence: confidence}})
	}
	hits, err := s.RelatedCandidateHits(ctx, []string{"target-chunk"}, "callers")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Candidate.Handle != "caller130-chunk" {
		t.Fatalf("incorrect language candidates: %+v", hits)
	}
	outgoing, err := s.RelatedCandidateHits(ctx, []string{"caller000-chunk"}, "callees")
	if err != nil || len(outgoing) != 0 {
		t.Fatalf("cross-language outgoing: %+v %v", outgoing, err)
	}
}
