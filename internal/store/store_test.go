package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
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

func TestSearchSymbolsInPathsFindsLateOwnedSymbol(t *testing.T) {
	s := openTestStore(t)
	path := "internal/indexer/indexer.go"
	extraction := model.Extraction{}
	for index := 0; index < 55; index++ {
		handle := fmt.Sprintf("helper-%02d", index)
		name := fmt.Sprintf("Helper%02d", index)
		extraction.Symbols = append(extraction.Symbols, model.Symbol{
			Handle: handle, FilePath: path, Language: "go", Kind: "function",
			Name: name, QualifiedName: name, Signature: "func " + name + "()",
			StartLine: index + 1, EndLine: index + 1, Confidence: 1,
		})
		extraction.Chunks = append(extraction.Chunks, model.Chunk{
			Handle: handle + "-chunk", FilePath: path, Language: "go", Kind: "function",
			SymbolHandle: handle, SymbolName: name, Signature: "func " + name + "()",
			StartLine: index + 1, EndLine: index + 1, Content: "short helper", ContentHash: handle,
		})
	}
	extraction.Symbols = append(extraction.Symbols, model.Symbol{
		Handle: "run", FilePath: path, Language: "go", Kind: "function", Name: "Run",
		QualifiedName: "Indexer.Run", Signature: "func Run() error", StartLine: 100, EndLine: 120, Confidence: 1,
	})
	extraction.Chunks = append(extraction.Chunks,
		model.Chunk{Handle: "run-body", FilePath: path, Language: "go", Kind: "function", SymbolHandle: "run", SymbolName: "Run", Signature: "func Run() error", StartLine: 100, EndLine: 120, Content: "extract index store metadata", ContentHash: "run-body"},
		model.Chunk{Handle: "run-outline", FilePath: path, Language: "go", Kind: "go-outline", SymbolHandle: "run", SymbolName: "Run", Signature: "func Run() error", StartLine: 99, EndLine: 121, Content: "Run outline extract index store metadata", ContentHash: "run-outline"},
		model.Chunk{Handle: "generic-noise", FilePath: path, Language: "go", Kind: "window", StartLine: 90, EndLine: 90, Content: strings.Repeat("extract index store metadata ", 20), ContentHash: "generic-noise"},
	)
	storeReplaceExtraction(t, s, path, extraction)

	old, err := s.SearchPaths(context.Background(), []string{path}, 50)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(old, func(candidate model.RankedCandidate) bool { return candidate.Symbol == "Run" }) {
		t.Fatalf("generic path search unexpectedly guaranteed Run in first 50: %+v", old)
	}

	fts := query.BuildFTS(query.Terms{Words: []string{"extract", "index", "store", "metadata"}})
	got, err := s.SearchSymbolsInPaths(context.Background(), []string{path}, []string{"Run"}, fts, 8, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0].Handle != "run-body" || got[1].Handle != "run-outline" {
		t.Fatalf("scoped Run order=%+v", got)
	}
	if slices.ContainsFunc(got, func(candidate model.RankedCandidate) bool { return candidate.Handle == "generic-noise" }) {
		t.Fatalf("unowned generic chunk entered scoped symbols: %+v", got)
	}
	seen := make(map[string]bool)
	for _, candidate := range got {
		if seen[candidate.Handle] {
			t.Fatalf("duplicate scoped handle %q in %+v", candidate.Handle, got)
		}
		seen[candidate.Handle] = true
	}
}

func TestSearchSymbolsInPathsMatchesVariantsAndOrdersPassStrength(t *testing.T) {
	s := openTestStore(t)
	path := "internal/mcpserver/server.go"
	extraction := model.Extraction{
		Symbols: []model.Symbol{
			{Handle: "code-context", FilePath: path, Language: "go", Kind: "method", Name: "codeContext", QualifiedName: "server.codeContext", Signature: "func codeContext()", StartLine: 1, EndLine: 2, Confidence: 1},
			{Handle: "search", FilePath: path, Language: "go", Kind: "method", Name: "Search", QualifiedName: "server.Search", Signature: "func Search()", StartLine: 3, EndLine: 4, Confidence: 1},
			{Handle: "qualified", FilePath: path, Language: "go", Kind: "method", Name: "Run", QualifiedName: "Service.Run", Signature: "func Run()", StartLine: 5, EndLine: 6, Confidence: 1},
			{Handle: "simple", FilePath: path, Language: "go", Kind: "method", Name: "Service.Run", QualifiedName: "Other.ServiceRun", Signature: "func ServiceRun()", StartLine: 7, EndLine: 8, Confidence: 1},
			{Handle: "prefix", FilePath: path, Language: "go", Kind: "method", Name: "Service.Runner", QualifiedName: "Other.ServiceRunner", Signature: "func ServiceRunner()", StartLine: 9, EndLine: 10, Confidence: 1},
			{Handle: "fts-only", FilePath: path, Language: "go", Kind: "method", Name: "Execute", QualifiedName: "Other.Execute", Signature: "func Execute()", StartLine: 11, EndLine: 12, Confidence: 1},
		},
	}
	for _, symbol := range extraction.Symbols {
		content := "ordinary body"
		if symbol.Handle == "fts-only" {
			content = "lexicalonly body"
		}
		extraction.Chunks = append(extraction.Chunks, model.Chunk{Handle: symbol.Handle + "-chunk", FilePath: path, Language: "go", Kind: symbol.Kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: symbol.StartLine, EndLine: symbol.EndLine, Content: content, ContentHash: symbol.Handle})
	}
	storeReplaceExtraction(t, s, path, extraction)

	got, err := s.SearchSymbolsInPaths(context.Background(), []string{path}, []string{"code_context", "codeContext", "CodeContext", "search", "Service.Run"}, query.BuildFTS(query.Terms{Words: []string{"lexicalonly"}}), 8, 40)
	if err != nil {
		t.Fatal(err)
	}
	for _, handle := range []string{"code-context-chunk", "search-chunk"} {
		if !slices.ContainsFunc(got, func(candidate model.RankedCandidate) bool { return candidate.Handle == handle }) {
			t.Fatalf("missing variant/case handle %q in %+v", handle, got)
		}
	}
	positions := candidatePositions(got)
	if !(positions["qualified-chunk"] < positions["simple-chunk"] && positions["simple-chunk"] < positions["prefix-chunk"] && positions["prefix-chunk"] < positions["fts-only-chunk"]) {
		t.Fatalf("pass strength positions=%v candidates=%+v", positions, got)
	}
}

func TestSearchSymbolsInPathsConstrainsFTSToExactPaths(t *testing.T) {
	s := openTestStore(t)
	for _, path := range []string{"scoped/run.go", "other/run.go"} {
		extraction := model.Extraction{
			Symbols: []model.Symbol{{Handle: path + "-run", FilePath: path, Language: "go", Kind: "function", Name: "Run", QualifiedName: path + ".Run", Signature: "func Run()", StartLine: 1, EndLine: 4, Confidence: 1}},
			Chunks:  []model.Chunk{{Handle: path + "-chunk", FilePath: path, Language: "go", Kind: "function", SymbolHandle: path + "-run", SymbolName: "Run", Signature: "func Run()", StartLine: 1, EndLine: 4, Content: "extract index store metadata", ContentHash: path}},
		}
		storeReplaceExtraction(t, s, path, extraction)
	}

	safeFTS := query.BuildFTS(query.Terms{Words: []string{"extract", `bad" OR *`}})
	got, err := s.SearchSymbolsInPaths(context.Background(), []string{"scoped/run.go"}, nil, safeFTS, 8, 40)
	if err != nil || len(got) != 1 || got[0].Path != "scoped/run.go" {
		t.Fatalf("path-constrained FTS=%+v err=%v expression=%q", got, err, safeFTS)
	}

	empty, err := s.SearchSymbolsInPaths(context.Background(), []string{"scoped/run.go"}, nil, "", 8, 40)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty FTS results=%v nil=%v err=%v", empty, empty == nil, err)
	}
}

func TestSearchSymbolsInPathsEnforcesFairnessCapsAndStableOrder(t *testing.T) {
	s := openTestStore(t)
	counts := []int{12, 3, 6, 6, 6, 6, 6, 6}
	paths := make([]string, 0, len(counts))
	for fileIndex, count := range counts {
		path := fmt.Sprintf("scope/%c.go", 'a'+rune(fileIndex))
		paths = append(paths, path)
		extraction := model.Extraction{}
		for symbolIndex := 0; symbolIndex < count; symbolIndex++ {
			handle := fmt.Sprintf("worker-%d-%02d", fileIndex, symbolIndex)
			name := fmt.Sprintf("Worker%d%02d", fileIndex, symbolIndex)
			extraction.Symbols = append(extraction.Symbols, model.Symbol{Handle: handle, FilePath: path, Language: "go", Kind: "function", Name: name, QualifiedName: name, Signature: "func " + name + "()", StartLine: symbolIndex + 1, EndLine: symbolIndex + 1, Confidence: 1})
			extraction.Chunks = append(extraction.Chunks, model.Chunk{Handle: handle + "-chunk", FilePath: path, Language: "go", Kind: "function", SymbolHandle: handle, SymbolName: name, Signature: "func " + name + "()", StartLine: symbolIndex + 1, EndLine: symbolIndex + 1, Content: "worker body", ContentHash: handle})
		}
		storeReplaceExtraction(t, s, path, extraction)
	}

	first, err := s.SearchSymbolsInPaths(context.Background(), paths, []string{"Worker"}, query.BuildFTS(query.Terms{Words: []string{"worker"}}), 8, 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SearchSymbolsInPaths(context.Background(), paths, []string{"Worker"}, query.BuildFTS(query.Terms{Words: []string{"worker"}}), 8, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 40 || !slices.EqualFunc(first, second, func(left, right model.RankedCandidate) bool { return left.Handle == right.Handle }) {
		t.Fatalf("total/stability count=%d stable=%v", len(first), slices.EqualFunc(first, second, func(left, right model.RankedCandidate) bool { return left.Handle == right.Handle }))
	}
	perPath := make(map[string]int)
	seen := make(map[string]bool)
	for _, candidate := range first {
		perPath[candidate.Path]++
		if perPath[candidate.Path] > 8 {
			t.Fatalf("per-path cap exceeded: %v", perPath)
		}
		if seen[candidate.Handle] {
			t.Fatalf("duplicate handle %q", candidate.Handle)
		}
		seen[candidate.Handle] = true
	}
	if perPath["scope/a.go"] != 8 || perPath["scope/b.go"] != 3 {
		t.Fatalf("fairness counts=%v", perPath)
	}
}

func candidatePositions(candidates []model.RankedCandidate) map[string]int {
	positions := make(map[string]int, len(candidates))
	for index, candidate := range candidates {
		positions[candidate.Handle] = index + 1
	}
	return positions
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

func storeReplaceExtraction(t *testing.T, s *Store, path string, extraction model.Extraction) {
	t.Helper()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: "go", SHA256: path}, extraction); err != nil {
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
