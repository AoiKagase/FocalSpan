package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

func TestSearchFilePathsUsesFrozenOrdering(t *testing.T) {
	s := openTestStore(t)
	for _, path := range []string{
		"internal/indexer/indexer.go",
		"internal/indexer/config.go",
		"internal/mcpserver/server.go",
		"internal/search/search.go",
		"internal/searchable/helpers.go",
		"docs/index.md",
		"testdata/repos/sample/indexer.go",
	} {
		storeSeedPath(t, s, path)
	}

	got, err := s.SearchFilePaths(context.Background(), []string{"index"}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0] != "docs/index.md" || !slices.Contains(got, "internal/indexer/indexer.go") {
		t.Fatalf("index paths=%v, want shorter final-prefix docs path first and internal indexer in scope", got)
	}

	got, err = s.SearchFilePaths(context.Background(), []string{"internal/indexer/indexer.go"}, 8)
	if err != nil || len(got) == 0 || got[0] != "internal/indexer/indexer.go" {
		t.Fatalf("exact path=%v err=%v", got, err)
	}

	got, err = s.SearchFilePaths(context.Background(), []string{"indexer.go"}, 8)
	wantExactFinal := []string{"internal/indexer/indexer.go", "testdata/repos/sample/indexer.go"}
	if err != nil || !slices.Equal(got, wantExactFinal) {
		t.Fatalf("exact final paths=%v err=%v, want %v", got, err, wantExactFinal)
	}

	got, err = s.SearchFilePaths(context.Background(), []string{"search"}, 8)
	if err != nil || len(got) < 2 || got[0] != "internal/search/search.go" || got[1] != "internal/searchable/helpers.go" {
		t.Fatalf("final-prefix before path-segment-prefix paths=%v err=%v", got, err)
	}

	got, err = s.SearchFilePaths(context.Background(), []string{"mcp"}, 1)
	if err != nil || !slices.Equal(got, []string{"internal/mcpserver/server.go"}) {
		t.Fatalf("mcp paths=%v err=%v", got, err)
	}
}

func TestSearchFilePathsNormalizesDeduplicatesAndLimits(t *testing.T) {
	s := openTestStore(t)
	for _, path := range []string{
		"internal/indexer/indexer.go",
		"internal/indexer/config.go",
		"docs/index.md",
	} {
		storeSeedPath(t, s, path)
	}

	first, err := s.SearchFilePaths(context.Background(), []string{`internal\indexer`, "INTERNAL/indexer", " internal/indexer "}, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SearchFilePaths(context.Background(), []string{`internal\indexer`, "INTERNAL/indexer"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/indexer/config.go", "internal/indexer/indexer.go"}
	if !slices.Equal(first, want) || !slices.Equal(second, want) {
		t.Fatalf("normalized paths first=%v second=%v, want %v", first, second, want)
	}

	empty, err := s.SearchFilePaths(context.Background(), []string{"", "  "}, 2)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty paths=%v nil=%v err=%v", empty, empty == nil, err)
	}
}

func TestSearchFilePathsWrapsCancellationAndSQLErrors(t *testing.T) {
	s := openTestStore(t)
	storeSeedPath(t, s, "internal/indexer/indexer.go")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := s.SearchFilePaths(ctx, []string{"index"}, 8)
	if err == nil || !errors.Is(err, context.Canceled) || got != nil || !strings.Contains(err.Error(), "file path search") {
		t.Fatalf("canceled paths=%v err=%v", got, err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = s.SearchFilePaths(context.Background(), []string{"index"}, 8)
	if err == nil || got != nil || !strings.Contains(err.Error(), "file path search") || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed database paths=%v err=%v", got, err)
	}
}

func TestSearchFilePathsIsBoundedAndDeterministicAcrossManyFiles(t *testing.T) {
	s := openTestStore(t)
	for index := 0; index < 500; index++ {
		storeSeedPath(t, s, fmt.Sprintf("src/generated/%03d.go", index))
	}

	first, err := s.SearchFilePaths(context.Background(), []string{"generated"}, 13)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SearchFilePaths(context.Background(), []string{"generated"}, 13)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 13 || !slices.Equal(first, second) {
		t.Fatalf("bounded paths count=%d stable=%v first=%v second=%v", len(first), slices.Equal(first, second), first, second)
	}
	if first[0] != "src/generated/000.go" || first[12] != "src/generated/012.go" {
		t.Fatalf("bounded lexical paths first=%q last=%q", first[0], first[12])
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

func storeSeedPath(t *testing.T, s *Store, path string) {
	t.Helper()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{
		Path:     path,
		Language: "fixture",
		SHA256:   path,
	}, model.Extraction{}); err != nil {
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
