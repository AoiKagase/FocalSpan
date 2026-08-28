package store

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestRelatedCandidatesRespectRelationAndHandles(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	extraction := model.Extraction{
		Symbols:   []model.Symbol{{Handle: "source", FilePath: "a.go", Language: "go", Kind: "function", Name: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}, {Handle: "test", FilePath: "a_test.go", Language: "go", Kind: "test", Name: "TestValidateToken", Signature: "func TestValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "chunk-source", FilePath: "a.go", Language: "go", Kind: "function", SymbolHandle: "source", SymbolName: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Content: "return nil", ContentHash: "source"}, {Handle: "chunk-test", FilePath: "a_test.go", Language: "go", Kind: "test", SymbolHandle: "test", SymbolName: "TestValidateToken", Signature: "func TestValidateToken", StartLine: 1, EndLine: 2, Content: "ValidateToken", ContentHash: "test"}},
		Relations: []model.Relation{{FromHandle: "test", ToHandle: "source", Kind: "tests", Confidence: .8, Source: "test"}},
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "a.go", Language: "go", SHA256: "a"}, model.Extraction{Symbols: extraction.Symbols[:1], Chunks: extraction.Chunks[:1]}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "a_test.go", Language: "go", SHA256: "b"}, model.Extraction{Symbols: extraction.Symbols[1:], Chunks: extraction.Chunks[1:], Relations: extraction.Relations}); err != nil {
		t.Fatal(err)
	}
	got, err := s.RelatedCandidates(context.Background(), []string{"test"}, "tests")
	if err != nil || len(got) != 1 || got[0].Symbol != "ValidateToken" {
		t.Fatalf("related=%+v err=%v", got, err)
	}
	missing, err := s.RelatedCandidates(context.Background(), []string{"test"}, "does-not-exist")
	if err != nil || len(missing) != 0 {
		t.Fatalf("unsupported relation=%+v err=%v", missing, err)
	}
}

func TestRelatedCandidatesResolveUnresolvedCallByTargetSymbolName(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "auth.go", Language: "go", SHA256: "auth"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "target", FilePath: "auth.go", Language: "go", Kind: "method", Name: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: "target-chunk", FilePath: "auth.go", Language: "go", Kind: "method", SymbolHandle: "target", SymbolName: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Content: "return nil", ContentHash: "target"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "http.go", Language: "go", SHA256: "caller"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "caller", FilePath: "http.go", Language: "go", Kind: "function", Name: "Authenticate", Signature: "func Authenticate", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "caller-chunk", FilePath: "http.go", Language: "go", Kind: "function", SymbolHandle: "caller", SymbolName: "Authenticate", Signature: "func Authenticate", StartLine: 1, EndLine: 2, Content: "service.ValidateToken()", ContentHash: "caller"}},
		Relations: []model.Relation{{FromHandle: "caller", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .3, Source: "go-ast"}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.RelatedCandidates(context.Background(), []string{"target-chunk"}, "callers")
	if err != nil || len(got) != 1 || got[0].Symbol != "Authenticate" {
		t.Fatalf("callers=%+v err=%v", got, err)
	}
}
