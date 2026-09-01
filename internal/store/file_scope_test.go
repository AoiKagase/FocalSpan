package store

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestSearchPathFilesReturnsStableDistinctPaths(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "internal/indexer/indexer.go", "go", "indexer", "Run", "Run", "index flow", 1, 2, 1)
	storeSeedFile(t, s, "docs/index.md", "markdown", "docs-index", "Index", "Index", "index flow", 1, 2, 1)
	storeSeedFile(t, s, "src/indexer.go", "go", "src-indexer", "Index", "Index", "index flow", 1, 2, 1)

	got, err := s.SearchPathFiles(context.Background(), []string{"index", "INDEX"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("paths=%v, want two distinct paths", got)
	}
	if got[0] != "docs/index.md" || got[1] != "src/indexer.go" {
		t.Fatalf("paths=%v, want deterministic final-segment order", got)
	}
}

func TestSearchFTSFilesGroupsMatchesBeforeApplyingLimit(t *testing.T) {
	s := openTestStore(t)
	noisyChunks := []model.Chunk{{Handle: "noisy-chunk", FilePath: "noisy.go", Language: "go", Kind: "function", SymbolHandle: "noisy", SymbolName: "Noisy", Signature: "Noisy", StartLine: 1, EndLine: 2, Content: "needle token", ContentHash: "noisy-chunk"}}
	for index := 0; index < 6; index++ {
		noisyChunks = append(noisyChunks, model.Chunk{Handle: "noisy-extra-" + string(rune('a'+index)), FilePath: "noisy.go", Language: "go", Kind: "window", StartLine: index + 3, EndLine: index + 3, Content: "needle token", ContentHash: "noisy-extra" + string(rune('a'+index))})
	}
	storeReplaceFile(t, s, "noisy.go", "go", []model.Symbol{{Handle: "noisy", FilePath: "noisy.go", Language: "go", Kind: "function", Name: "Noisy", QualifiedName: "Noisy", Signature: "Noisy", StartLine: 1, EndLine: 2, Confidence: 1}}, noisyChunks)
	storeSeedFile(t, s, "target.go", "go", "target", "Target", "Target", "needle token", 1, 2, 1)
	storeSeedFile(t, s, "other.go", "go", "other", "Other", "Other", "needle token", 1, 2, 1)

	got, err := s.SearchFTSFiles(context.Background(), "needle", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("files=%v, want three distinct files", got)
	}
	seen := map[string]bool{}
	for _, path := range got {
		if seen[path] {
			t.Fatalf("duplicate file in grouped results: %v", got)
		}
		seen[path] = true
	}
	if !seen["target.go"] || !seen["other.go"] {
		t.Fatalf("grouped results omitted target files: %v", got)
	}
}

func TestSearchCandidatesInFilesKeepsSymbolOwnedCandidatesAndCapsPerFile(t *testing.T) {
	s := openTestStore(t)
	storeReplaceFile(t, s, "a.go", "go", []model.Symbol{{Handle: "a", FilePath: "a.go", Language: "go", Kind: "function", Name: "ValidateToken", QualifiedName: "ValidateToken", Signature: "ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}}, []model.Chunk{{Handle: "a-chunk", FilePath: "a.go", Language: "go", Kind: "function", SymbolHandle: "a", SymbolName: "ValidateToken", Signature: "ValidateToken", StartLine: 1, EndLine: 2, Content: "needle ValidateToken", ContentHash: "a-chunk"}, {Handle: "unowned", FilePath: "a.go", Language: "go", Kind: "window", StartLine: 3, EndLine: 3, Content: "needle ValidateToken", ContentHash: "unowned"}})
	storeSeedFile(t, s, "b.go", "go", "b", "RefreshToken", "RefreshToken", "needle RefreshToken", 1, 2, 1)

	got, err := s.SearchCandidatesInFiles(context.Background(), []string{"a.go", "b.go"}, []string{"ValidateToken", "RefreshToken"}, "needle", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("candidates=%+v, want one per file", got)
	}
	for _, candidate := range got {
		if candidate.Path != "a.go" && candidate.Path != "b.go" {
			t.Fatalf("candidate escaped requested paths: %+v", candidate)
		}
		if candidate.Handle == "unowned" || candidate.Symbol == "" {
			t.Fatalf("unowned candidate returned: %+v", candidate)
		}
	}
}

func storeReplaceFile(t *testing.T, s *Store, path, language string, symbols []model.Symbol, chunks []model.Chunk) {
	t.Helper()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: language, SHA256: path}, model.Extraction{Symbols: symbols, Chunks: chunks}); err != nil {
		t.Fatal(err)
	}
}
