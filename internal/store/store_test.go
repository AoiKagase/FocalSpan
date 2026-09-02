package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestSearchExactSymbolsDoesNotDependOnFTSContent(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "src/Auth/TokenService.php", "php", "target", "ValidateToken", `App\Auth\TokenService::ValidateToken`, "this chunk has no searchable token", 1, 5, 1)
	storeSeedFile(t, s, "docs/auth.md", "markdown", "docs", "", "", "ValidateToken is only mentioned in documentation", 1, 2, .5)

	got, err := s.SearchExactSymbols(context.Background(), []string{"ValidateToken"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Handle != "target-chunk" {
		t.Fatalf("exact=%+v, want symbol-backed chunk without FTS content", got)
	}
}

func TestSearchQualifiedSymbolsUsesExactQualifiedName(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "src/Auth/TokenService.php", "php", "target", "ValidateToken", `App\Auth\TokenService::ValidateToken`, "implementation", 2, 8, 1)

	got, err := s.SearchQualifiedSymbols(context.Background(), []string{`App\Auth\TokenService::ValidateToken`}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "ValidateToken" {
		t.Fatalf("qualified=%+v", got)
	}
}

func TestSearchSymbolPrefixesIsCaseInsensitiveAndBounded(t *testing.T) {
	s := openTestStore(t)
	for index := 0; index < 250; index++ {
		name := fmt.Sprintf("ValidateToken%03d", index)
		storeSeedFile(t, s, fmt.Sprintf("src/%03d.go", index), "go", fmt.Sprintf("symbol-%03d", index), name, name, "body", 1, 2, 1)
	}

	got, err := s.SearchSymbolPrefixes(context.Background(), []string{"validatetoken"}, 37)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 37 {
		t.Fatalf("prefix count=%d, want explicit limit 37", len(got))
	}
	if got[0].Path != "src/000.go" || got[len(got)-1].Path != "src/036.go" {
		t.Fatalf("prefix order is not stable: first=%+v last=%+v", got[0], got[len(got)-1])
	}
}

func TestSearchPathsMatchesNormalizedPathHints(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "src/Auth/TokenService.php", "php", "target", "TokenService", `App\Auth\TokenService`, "path content", 1, 3, 1)

	got, err := s.SearchPaths(context.Background(), []string{`src\Auth\TokenService.php`}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "src/Auth/TokenService.php" {
		t.Fatalf("paths=%+v", got)
	}
}

func TestSearchMethodsUseStableTieBreaks(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "z.go", "go", "z", "ValidateToken", "ValidateToken", "body", 10, 12, 1)
	storeSeedFile(t, s, "a.go", "go", "a", "ValidateToken", "ValidateToken", "body", 10, 12, 1)

	got, err := s.SearchExactSymbols(context.Background(), []string{"ValidateToken"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "a.go" || got[1].Path != "z.go" {
		t.Fatalf("stable order=%+v", got)
	}
}

func TestSearchMethodsClampLimits(t *testing.T) {
	if got := retrievalLimit(0, 100, 500); got != 100 {
		t.Fatalf("fallback limit=%d", got)
	}
	if got := retrievalLimit(-1, 100, 500); got != 100 {
		t.Fatalf("negative limit=%d", got)
	}
	if got := retrievalLimit(9999, 100, 500); got != 500 {
		t.Fatalf("maximum limit=%d", got)
	}

	s := openTestStore(t)
	storeSeedFile(t, s, "one.go", "go", "one", "ValidateToken", "ValidateToken", "body", 1, 1, 1)
	got, err := s.SearchExactSymbols(context.Background(), []string{"ValidateToken"}, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("default limit results=%+v err=%v", got, err)
	}
}

func TestSearchMethodsHandleCancellationAndMalformedInputs(t *testing.T) {
	s := openTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.SearchExactSymbols(ctx, []string{"ValidateToken"}, 10); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("canceled exact err=%v", err)
	}
	for _, values := range [][]string{nil, {}, {""}} {
		got, err := s.SearchPaths(context.Background(), values, 10)
		if err != nil || len(got) != 0 {
			t.Fatalf("empty path values=%v results=%+v err=%v", values, got, err)
		}
	}
	got, err := s.SearchPaths(context.Background(), []string{"%_\\'\x00"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("malformed path hint results=%+v err=%v", got, err)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func storeSeedFile(t *testing.T, s *Store, path, language, handle, name, qualified, content string, startLine, endLine int, confidence float64) {
	t.Helper()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: language, SHA256: handle}, model.Extraction{
		Symbols: []model.Symbol{{Handle: handle, FilePath: path, Language: language, Kind: "function", Name: name, QualifiedName: qualified, StartLine: startLine, EndLine: endLine, StartByte: 0, EndByte: len(content), Confidence: confidence}},
		Chunks:  []model.Chunk{{Handle: handle + "-chunk", FilePath: path, Language: language, Kind: "function", SymbolHandle: handle, SymbolName: name, Signature: name, StartLine: startLine, EndLine: endLine, StartByte: 0, EndByte: len(content), Content: content, ContentHash: handle}},
	}); err != nil {
		t.Fatal(err)
	}
}

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
	results, err := s.SearchFTS(context.Background(), "expired", 100)
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
	old, err := s.SearchFTS(context.Background(), "expired", 100)
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
