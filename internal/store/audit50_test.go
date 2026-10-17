package store_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"reflect"
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
	ctx := context.Background()
	symbol := model.Symbol{Handle: "caller", FilePath: "caller.go", Name: "Caller", Language: "go", Kind: "function", StartLine: 1, EndLine: 1}
	if err := s.ReplaceFile(ctx, model.SourceFile{Path: "caller.go", Language: "go", SHA256: "x"}, model.Extraction{Symbols: []model.Symbol{symbol}, Relations: []model.Relation{{FromHandle: "caller", UnresolvedTo: "Target", Kind: "calls", Confidence: 1}}}); err != nil {
		t.Fatal(err)
	}
	small, err := s.RelationsForLink(ctx, store.LinkScope{ChangedPaths: []string{"caller.go"}})
	if err != nil || len(small) != 1 {
		t.Fatalf("small control: %+v %v", small, err)
	}
	paths := append(append([]string(nil), keys...), "caller.go", "caller.go")
	large, err := s.RelationsForLink(ctx, store.LinkScope{ChangedPaths: paths, LookupKeys: keys})
	if err != nil || !reflect.DeepEqual(small, large) {
		t.Fatalf("mixed large scope differs: %+v %v", large, err)
	}
	empty, err := s.RelationsForLink(ctx, store.LinkScope{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty=%+v %v", empty, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.RelationsForLink(canceled, store.LinkScope{LookupKeys: keys}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
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
	for _, scoped := range []bool{false, true} {
		var reader model.CandidateReader = s
		if scoped {
			reader = s.ScopedCandidates(model.CandidateScope{Paths: []string{"large.go"}})
		}
		calls := []struct {
			name string
			fn   func() ([]model.RankedCandidate, error)
		}{
			{"FTS", func() ([]model.RankedCandidate, error) { return reader.SearchFTS(ctx, "LargeFunction", 10) }},
			{"qualified", func() ([]model.RankedCandidate, error) {
				return reader.SearchQualifiedSymbols(ctx, []string{"p.LargeFunction"}, 10)
			}},
			{"prefix", func() ([]model.RankedCandidate, error) {
				return reader.SearchSymbolPrefixes(ctx, []string{"Large"}, 10)
			}},
			{"path", func() ([]model.RankedCandidate, error) { return reader.SearchPaths(ctx, []string{"large.go"}, 10) }},
			{"related", func() ([]model.RankedCandidate, error) {
				return reader.RelatedCandidates(ctx, []string{symbol.Handle}, "self")
			}},
		}
		for _, call := range calls {
			got, err := call.fn()
			if err != nil || len(got) != 1 || got[0].EstimatedTokens != 5001 {
				t.Errorf("scoped=%v %s: %+v %v", scoped, call.name, got, err)
			}
		}
	}

}
