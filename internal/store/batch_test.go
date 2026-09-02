package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestSearchExactSymbolsBatchesMultipleValues(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "one.go", "go", "one", "ValidateToken", "ValidateToken", "one", 1, 1, 1)
	storeSeedFile(t, s, "two.go", "go", "two", "RefreshToken", "RefreshToken", "two", 1, 1, 1)
	var queries int
	ctx := withQueryCounter(context.Background(), &queries)
	got, err := s.SearchExactSymbols(ctx, []string{"ValidateToken", "RefreshToken"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Symbol != "ValidateToken" || got[1].Symbol != "RefreshToken" {
		t.Fatalf("exact=%+v", got)
	}
	if queries != 1 {
		t.Fatalf("query count=%d, want one batched query", queries)
	}
}

func TestRelatedCandidateHitsBatchesMultipleAnchors(t *testing.T) {
	s := openTestStore(t)
	seedRelatedChild(t, s, "one", "one-chunk", "child-one", "child-one-chunk")
	seedRelatedChild(t, s, "two", "two-chunk", "child-two", "child-two-chunk")
	var queries int
	ctx := withQueryCounter(context.Background(), &queries)
	hits, err := s.RelatedCandidateHits(ctx, []string{"one-chunk", "two-chunk"}, "children")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Candidate.Handle != "child-one-chunk" || hits[1].Candidate.Handle != "child-two-chunk" {
		t.Fatalf("hits=%+v", hits)
	}
	if queries != 1 {
		t.Fatalf("query count=%d, want one batched query", queries)
	}
}

func TestSearchBatchMethodsPreserveSequentialOrder(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "src/one.go", "go", "one", "ValidateToken", "App.ValidateToken", "one", 1, 1, 1)
	storeSeedFile(t, s, "src/two.go", "go", "two", "RefreshToken", "App.RefreshToken", "two", 1, 1, 1)
	values := []string{"ValidateToken", "RefreshToken"}
	tests := []struct {
		name   string
		batch  func([]string) ([]model.RankedCandidate, error)
		single func(string) ([]model.RankedCandidate, error)
	}{
		{name: "qualified", batch: func(values []string) ([]model.RankedCandidate, error) {
			return s.SearchQualifiedSymbols(context.Background(), values, 10)
		}, single: func(value string) ([]model.RankedCandidate, error) {
			return s.SearchQualifiedSymbols(context.Background(), []string{value}, 10)
		}},
		{name: "prefix", batch: func(values []string) ([]model.RankedCandidate, error) {
			return s.SearchSymbolPrefixes(context.Background(), values, 10)
		}, single: func(value string) ([]model.RankedCandidate, error) {
			return s.SearchSymbolPrefixes(context.Background(), []string{value}, 10)
		}},
		{name: "path", batch: func(values []string) ([]model.RankedCandidate, error) {
			return s.SearchPaths(context.Background(), values, 10)
		}, single: func(value string) ([]model.RankedCandidate, error) {
			return s.SearchPaths(context.Background(), []string{value}, 10)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			batchValues := values
			if test.name == "qualified" {
				batchValues = []string{"App.ValidateToken", "App.RefreshToken"}
			}
			if test.name == "path" {
				batchValues = []string{`src\\one.go`, `src/two.go`}
			}
			got, err := test.batch(batchValues)
			if err != nil {
				t.Fatal(err)
			}
			want := make([]model.RankedCandidate, 0, len(batchValues))
			for _, value := range batchValues {
				items, err := test.single(value)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, items...)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("batch=%+v, sequential=%+v", got, want)
			}
		})
	}
}

func TestRelatedCandidatesBatchesMultipleAnchors(t *testing.T) {
	s := openTestStore(t)
	seedRelatedChild(t, s, "one", "one-chunk", "child-one", "child-one-chunk")
	seedRelatedChild(t, s, "two", "two-chunk", "child-two", "child-two-chunk")
	var queries int
	ctx := withQueryCounter(context.Background(), &queries)
	candidates, err := s.RelatedCandidates(ctx, []string{"one-chunk", "two-chunk"}, "children")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0].Handle != "child-one-chunk" || candidates[1].Handle != "child-two-chunk" {
		t.Fatalf("candidates=%+v", candidates)
	}
	if queries != 1 {
		t.Fatalf("query count=%d, want one batched query", queries)
	}
}

func TestRelatedCandidateHitsBatchesCamelCasePatterns(t *testing.T) {
	s := openTestStore(t)
	seedUnresolvedCaller(t, s, "target-one", "target-one-chunk", "ValidateToken", "caller-one", "caller-one-chunk")
	seedUnresolvedCaller(t, s, "target-two", "target-two-chunk", "RefreshToken", "caller-two", "caller-two-chunk")
	var queries int
	hits, err := s.RelatedCandidateHits(withQueryCounter(context.Background(), &queries), []string{"target-one-chunk", "target-two-chunk"}, "callers")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Candidate.Handle != "caller-one-chunk" || hits[1].Candidate.Handle != "caller-two-chunk" {
		t.Fatalf("hits=%+v", hits)
	}
	if queries != 2 {
		t.Fatalf("query count=%d, want one camel-pattern and one relation query", queries)
	}
}

func seedUnresolvedCaller(t *testing.T, s *Store, target, targetChunk, targetName, caller, callerChunk string) {
	t.Helper()
	targetPath := target + ".go"
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: targetPath, Language: "go", SHA256: target}, model.Extraction{
		Symbols: []model.Symbol{{Handle: target, FilePath: targetPath, Language: "go", Kind: "function", Name: targetName, QualifiedName: targetName, StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: targetChunk, FilePath: targetPath, Language: "go", Kind: "function", SymbolHandle: target, SymbolName: targetName, Signature: "func " + targetName, StartLine: 1, EndLine: 2, Content: targetName, ContentHash: target}},
	}); err != nil {
		t.Fatal(err)
	}
	callerPath := caller + ".go"
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: callerPath, Language: "go", SHA256: caller}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: caller, FilePath: callerPath, Language: "go", Kind: "function", Name: caller, QualifiedName: caller, StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: callerChunk, FilePath: callerPath, Language: "go", Kind: "function", SymbolHandle: caller, SymbolName: caller, Signature: "func " + caller, StartLine: 1, EndLine: 2, Content: targetName, ContentHash: caller}},
		Relations: []model.Relation{{FromHandle: caller, UnresolvedTo: targetName, Kind: "calls", Confidence: .4, Source: "test"}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSearchBatchFallsBackBeforeSQLiteParameterLimit(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "one.go", "go", "one", "ValidateToken", "ValidateToken", "one", 1, 1, 1)
	values := make([]string, 451)
	values[0] = "ValidateToken"
	for index := 1; index < len(values); index++ {
		values[index] = fmt.Sprintf("MissingSymbol%d", index)
	}
	var queries int
	got, err := s.SearchExactSymbols(withQueryCounter(context.Background(), &queries), values, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "ValidateToken" {
		t.Fatalf("exact fallback=%+v", got)
	}
	if queries != len(values) {
		t.Fatalf("fallback query count=%d, want %d sequential queries", queries, len(values))
	}
}

func seedRelatedChild(t *testing.T, s *Store, parent, parentChunk, child, childChunk string) {
	t.Helper()
	path := parent + ".go"
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: "go", SHA256: parent}, model.Extraction{
		Symbols: []model.Symbol{
			{Handle: parent, FilePath: path, Language: "go", Kind: "function", Name: parent, QualifiedName: parent, StartLine: 1, EndLine: 2, Confidence: 1},
			{Handle: child, FilePath: path, Language: "go", Kind: "function", Name: child, QualifiedName: child, StartLine: 3, EndLine: 4, Confidence: 1},
		},
		Chunks: []model.Chunk{
			{Handle: parentChunk, FilePath: path, Language: "go", Kind: "function", SymbolHandle: parent, SymbolName: parent, Signature: "func " + parent, StartLine: 1, EndLine: 2, Content: parent, ContentHash: parent},
			{Handle: childChunk, FilePath: path, Language: "go", Kind: "function", SymbolHandle: child, SymbolName: child, Signature: "func " + child, StartLine: 3, EndLine: 4, Content: child, ContentHash: child},
		},
		Relations: []model.Relation{{FromHandle: parent, ToHandle: child, Kind: "contains", Confidence: 1, Source: "test"}},
	}); err != nil {
		t.Fatal(err)
	}
}
