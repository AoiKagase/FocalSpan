package store

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestStoreMigrationAndFTSSynchronization(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	file := model.SourceFile{Path: "auth.go", Language: "go", Content: []byte("package auth"), SHA256: "one", SizeBytes: 12}
	first := model.Symbol{Handle: "sym_one", FilePath: file.Path, Language: "go", Kind: "function", Name: "ValidateToken", QualifiedName: "ValidateToken", Signature: "func ValidateToken() error", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 12, Confidence: 1}
	extraction := model.Extraction{Symbols: []model.Symbol{first}, Chunks: []model.Chunk{{Handle: "chunk_one", FilePath: file.Path, Language: "go", Kind: "function", SymbolHandle: first.Handle, SymbolName: first.Name, Signature: first.Signature, StartLine: 1, EndLine: 1, Content: "expired token", ContentHash: "hash-one"}}}
	if err := s.ReplaceFile(context.Background(), file, extraction); err != nil {
		t.Fatal(err)
	}
	results, err := s.SearchFTS(context.Background(), "expired")
	if err != nil || len(results) != 1 || results[0].Symbol != "ValidateToken" {
		t.Fatalf("first search=%+v err=%v", results, err)
	}
	file.SHA256 = "two"
	second := first
	second.Handle = "sym_two"
	second.Name = "RefreshToken"
	extraction.Symbols = []model.Symbol{second}
	extraction.Chunks = []model.Chunk{{Handle: "chunk_two", FilePath: file.Path, Language: "go", Kind: "function", SymbolHandle: second.Handle, SymbolName: second.Name, Signature: "func RefreshToken() error", StartLine: 1, EndLine: 1, Content: "refresh token", ContentHash: "hash-two"}}
	if err := s.ReplaceFile(context.Background(), file, extraction); err != nil {
		t.Fatal(err)
	}
	old, err := s.SearchFTS(context.Background(), "expired")
	if err != nil || len(old) != 0 {
		t.Fatalf("old FTS rows=%+v err=%v", old, err)
	}
	if err := s.DeleteFile(context.Background(), file.Path); err != nil {
		t.Fatal(err)
	}
	status, err := s.Status(context.Background(), root)
	if err != nil || status.FileCount != 0 || status.ChunkCount != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStoreRejectsMissingSchemaVersion(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetMeta(context.Background(), "schema_version", "999"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Meta(context.Background(), "schema_version"); err != nil || got != "999" {
		t.Fatalf("meta=%q err=%v", got, err)
	}
}
