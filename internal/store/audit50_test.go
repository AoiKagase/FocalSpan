package store_test

import (
	"context"
	"fmt"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"testing"
)

func TestF08LargeRelinkScope(t *testing.T) {
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	keys := make([]string, 40000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i)
	}
	if _, err := s.RelationsForLink(context.Background(), store.LinkScope{LookupKeys: keys}); err != nil {
		t.Fatalf("large scope fails: %v", err)
	}
}
func TestF23NoSearchableTerms(t *testing.T) {
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "a.go", Language: "go", SHA256: "hash"}, model.Extraction{Chunks: []model.Chunk{{Handle: "c", FilePath: "a.go", Content: "hello", StartLine: 1, EndLine: 1}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SearchFTS(context.Background(), "", 100); err != nil {
		t.Fatalf("normalized empty query reaches invalid FTS expression: %v", err)
	}
}

func TestF22EstimatedTokensRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	symbol := model.Symbol{Handle: "large-symbol", FilePath: "large.go", Language: "go", Kind: "function", Name: "LargeFunction", QualifiedName: "p.LargeFunction", StartLine: 1, EndLine: 2, StartByte: 0, EndByte: 10, Confidence: 1}
	chunk := model.Chunk{Handle: "large-chunk", FilePath: "large.go", Language: "go", Kind: "function", SymbolHandle: symbol.Handle, SymbolName: symbol.Name, StartLine: 1, EndLine: 2, StartByte: 0, EndByte: 10, Content: "LargeFunction", ContentHash: "hash", EstimatedTokens: 5001}
	if err := s.ReplaceFile(ctx, model.SourceFile{Path: "large.go", Language: "go", SHA256: "hash"}, model.Extraction{Symbols: []model.Symbol{symbol}, Chunks: []model.Chunk{chunk}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.SearchExactSymbols(ctx, []string{"LargeFunction"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates=%+v", got)
	}
	if got[0].EstimatedTokens != 5001 {
		t.Fatalf("persisted tokens=5001, retrieved=%d; large-span ranking penalty cannot run", got[0].EstimatedTokens)
	}
}
