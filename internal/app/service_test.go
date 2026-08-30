package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/model"
)

func TestNewExtractorRegistrySelectsDedicatedExtractors(t *testing.T) {
	registry := newExtractorRegistry()
	tests := []struct {
		name, path, language, want string
	}{
		{name: "go", path: "auth.go", language: "go", want: "go-ast"},
		{name: "php", path: "src/Auth/TokenService.php", language: "php", want: "php-structural"},
		{name: "cpp", path: "src/token_service.cpp", language: "cpp", want: "cpp-structural"},
		{name: "csharp", path: "Auth/TokenService.cs", language: "csharp", want: "csharp-structural"},
		{name: "javascript", path: "src/auth/token-service.ts", language: "typescript", want: "jsts-structural"},
		{name: "template", path: "pages/login.tpl", language: "smarty", want: "template-structural"},
		{name: "rust", path: "src/lib.rs", language: "rust", want: "generic-structural"},
		{name: "python", path: "src/auth.py", language: "python", want: "generic-structural"},
		{name: "ruby", path: "lib/auth.rb", language: "ruby", want: "generic-structural"},
		{name: "lua", path: "lua/auth.lua", language: "lua", want: "generic-structural"},
		{name: "pawn", path: "addons/amxmodx/scripting/plugin.sma", language: "pawn", want: "generic-structural"},
		{name: "vb6", path: "legacy/Module1.bas", language: "vb6", want: "generic-structural"},
		{name: "vbnet", path: "src/Form.vb", language: "vbnet", want: "generic-structural"},
		{name: "xaml", path: "Views/Main.xaml", language: "xaml", want: "xaml-structural"},
		{name: "resource", path: "Properties/Resources.resx", language: "dotnet-resource", want: "resx-structural"},
		{name: "nim", path: "src/auth.nim", language: "nim", want: "generic-structural"},
		{name: "zig", path: "src/auth.zig", language: "zig", want: "generic-structural"},
		{name: "generic", path: "README.txt", language: "text", want: "generic-structural"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			extractor, ok := registry.For(test.path, test.language)
			if !ok {
				t.Fatalf("registry.For(%q, %q) did not find an extractor", test.path, test.language)
			}
			if extractor.Name() != test.want {
				t.Fatalf("registry.For(%q, %q) = %q, want %q", test.path, test.language, extractor.Name(), test.want)
			}
		})
	}
}

func TestQueryAutoIndexesAndHonorsBudget(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\n// ValidateToken rejects expired tokens.\nfunc ValidateToken(token string) error { return nil }\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	bundle, err := a.Query(context.Background(), QueryRequest{Query: "expired token ValidateToken", TokenBudget: 512, Mode: "source"})
	if err != nil || len(bundle.Items) == 0 || bundle.EstimatedTokens > 512 || bundle.Items[0].Path != "auth.go" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
	status, err := a.Status(context.Background())
	if err != nil || status.FileCount != 1 || status.ChunkCount == 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	baseline, err := a.BaselineTokens(context.Background(), "expired token ValidateToken")
	if err != nil || baseline <= 0 {
		t.Fatalf("baseline=%d err=%v", baseline, err)
	}
}

func TestQueryPrefersSpecificCSharpAndJavaScriptSpans(t *testing.T) {
	tests := []struct {
		name, path, language, content, query, symbol string
	}{
		{
			name: "csharp", path: "TokenService.cs", language: "csharp", query: "ValidateToken", symbol: "ValidateToken",
			content: "namespace Demo {\npublic class TokenService {\n    public bool ValidateToken(string token) { return token.Length > 0; }\n    public void Unrelated() {\n" + strings.Repeat("        // unrelated implementation detail\n", 30) + "    }\n}\n}\n",
		},
		{
			name: "javascript", path: "token-service.js", language: "javascript", query: "validateToken", symbol: "validateToken",
			content: "export class TokenService {\n    validateToken(token) { return token.length > 0; }\n    unrelated() {\n" + strings.Repeat("        // unrelated implementation detail\n", 30) + "    }\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeAppFile(t, filepath.Join(root, test.path), test.content)
			a, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()

			bundle, err := a.Query(context.Background(), QueryRequest{Query: test.query, TokenBudget: 512, Mode: "source"})
			if err != nil || len(bundle.Items) == 0 {
				t.Fatalf("bundle=%+v err=%v", bundle, err)
			}
			if bundle.Items[0].Symbol != test.symbol {
				t.Fatalf("first item=%+v, want symbol %q", bundle.Items[0], test.symbol)
			}
			for _, item := range bundle.Items {
				if item.Symbol == "TokenService" {
					t.Fatalf("broad class span was retained: %+v", bundle.Items)
				}
			}
			if bundle.Savings == nil || bundle.Savings.SavingsRatio <= 0 {
				t.Fatalf("savings=%+v bundle=%+v", bundle.Savings, bundle)
			}
		})
	}
}

func TestExpandReturnsSelfAndEmptyForUnsupportedRelation(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	bundle, err := a.Query(context.Background(), QueryRequest{Query: "ValidateToken", TokenBudget: 512, Mode: "outline"})
	if err != nil || len(bundle.Items) == 0 {
		t.Fatalf("query=%+v err=%v", bundle, err)
	}
	expanded, err := a.Expand(context.Background(), []string{bundle.Items[0].Handle}, "self", 512)
	if err != nil || len(expanded.Items) != 1 || expanded.Savings == nil {
		t.Fatalf("expanded=%+v err=%v", expanded, err)
	}
	empty, err := a.Expand(context.Background(), []string{bundle.Items[0].Handle}, "unknown", 512)
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("unknown relation=%+v err=%v", empty, err)
	}
}

func TestImpactReturnsChangedSpanWithSyntaxOnlyDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	runAppGit(t, root, "init")
	runAppGit(t, root, "add", "auth.go")
	runAppGit(t, root, "-c", "user.name=focalspan", "-c", "user.email=focalspan@example.invalid", "commit", "-m", "initial")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return ErrExpired }\n")
	bundle, err := a.Impact(context.Background(), "", "", 512)
	if err != nil || len(bundle.Items) == 0 || bundle.Items[0].Path != "auth.go" || bundle.Savings == nil || len(bundle.Diagnostics) == 0 {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
}

func TestAppIndexesPHPAndIncFilesWithPHPExtractor(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "Service.php"), "<?php\nnamespace App;\nclass Service { public function run(): void {} }\n")
	writeAppFile(t, filepath.Join(root, "bootstrap.inc"), "<?php\nfunction bootstrap(): void {}\n")
	writeAppFile(t, filepath.Join(root, "template.phtml"), "<main><?= htmlspecialchars($title) ?></main><?php function render(): void {} ?>\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	candidates, err := a.Store.AllCandidates(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	seenPaths := map[string]bool{}
	seenSymbols := map[string]bool{}
	seenTemplate := false
	for _, candidate := range candidates {
		if candidate.Language != "php" {
			t.Fatalf("candidate language=%q: %+v", candidate.Language, candidate)
		}
		seenPaths[candidate.Path] = true
		seenSymbols[candidate.Symbol] = true
		if candidate.Path == "template.phtml" {
			seenTemplate = true
			if candidate.Language != "php" {
				t.Fatalf("mixed PHP template language=%q: %+v", candidate.Language, candidate)
			}
		}
		if candidate.StartLine < 1 || candidate.EndLine < candidate.StartLine {
			t.Fatalf("invalid candidate span=%+v", candidate)
		}
	}
	if !seenPaths["Service.php"] || !seenPaths["bootstrap.inc"] || !seenSymbols["Service"] || !seenSymbols["bootstrap"] || !seenTemplate {
		t.Fatalf("PHP index paths=%v symbols=%v candidates=%+v", seenPaths, seenSymbols, candidates)
	}
}

func TestPHPFixtureQueriesReturnRelevantBoundedContext(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "repos", "phpsample"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name           string
		query          string
		expectedSymbol string
		expectedPath   string
	}{
		{name: "expired token", query: "where is an expired PHP authentication token rejected?", expectedSymbol: "validateToken", expectedPath: "src/Auth/TokenService.php"},
		{name: "token callers", query: "what calls TokenService validateToken?", expectedSymbol: "validateToken", expectedPath: "src/Http/AuthMiddleware.php"},
		{name: "expired token tests", query: "what tests cover expired PHP tokens?", expectedSymbol: "testExpiredTokenIsRejected", expectedPath: "tests/TokenServiceTest.php"},
		{name: "bootstrap include", query: "which include file bootstraps authentication?", expectedSymbol: "bootstrap", expectedPath: "includes/bootstrap.inc"},
	}
	cfg := config.Default()
	cfg.IndexDirectory = ".focalspan-php-test"
	a, err := NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(root, cfg.IndexDirectory)) })
	if _, err := a.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			bundle, err := a.Query(context.Background(), QueryRequest{Query: testCase.query, TokenBudget: 1200, NoUpdate: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(bundle.Items) == 0 {
				t.Fatalf("items=%d bundle=%+v", len(bundle.Items), bundle)
			}
			items := bundle.Items
			if len(items) > 5 {
				items = items[:5]
			}
			foundSymbol, foundPath := false, false
			for _, item := range items {
				if item.Path == "unrelated/Report.php" {
					t.Fatalf("forbidden unrelated item=%+v", item)
				}
				fullPath := filepath.Join(root, filepath.FromSlash(item.Path))
				content, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("item path=%q: %v", item.Path, err)
				}
				lineCount := 1 + strings.Count(string(content), "\n")
				if item.StartLine < 1 || item.EndLine < item.StartLine || item.EndLine > lineCount {
					t.Fatalf("invalid line range item=%+v lines=%d", item, lineCount)
				}
				if item.Symbol == testCase.expectedSymbol {
					foundSymbol = true
				}
				if item.Path == testCase.expectedPath {
					foundPath = true
				}
			}
			if !foundSymbol || !foundPath {
				t.Fatalf("expected symbol=%q path=%q in top five: %+v", testCase.expectedSymbol, testCase.expectedPath, bundle.Items)
			}
		})
	}
}

func TestQueryReportsTokenSavings(t *testing.T) {
	root := t.TempDir()
	content := "package auth\n\n// ValidateToken rejects expired tokens.\nfunc ValidateToken(token string) error { return nil }\n" + strings.Repeat("// additional context\n", 30)
	writeAppFile(t, filepath.Join(root, "auth.go"), content)
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	bundle, err := a.Query(context.Background(), QueryRequest{Query: "ValidateToken", TokenBudget: 512, Mode: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Savings == nil {
		t.Fatalf("expected token savings, bundle=%+v", bundle)
	}
	baseline := budget.NewEstimator().Estimate(content)
	if bundle.Savings.BaselineTokens != baseline {
		t.Fatalf("baseline=%d, want %d", bundle.Savings.BaselineTokens, baseline)
	}
	wantSaved := baseline - bundle.EstimatedTokens
	if bundle.Savings.SavedTokens != wantSaved {
		t.Fatalf("saved=%d, want %d", bundle.Savings.SavedTokens, wantSaved)
	}
	wantRatio := float64(wantSaved) / float64(baseline)
	if bundle.Savings.SavingsRatio != wantRatio {
		t.Fatalf("ratio=%v, want %v", bundle.Savings.SavingsRatio, wantRatio)
	}
}

func TestBaselineTokensForCandidatesDeduplicatesPaths(t *testing.T) {
	root := t.TempDir()
	content := "package auth\n\nfunc ValidateToken() error { return nil }\n"
	writeAppFile(t, filepath.Join(root, "auth.go"), content)
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	got, err := a.baselineTokensForCandidates(context.Background(), []model.RankedCandidate{{Path: "auth.go"}, {Path: "auth.go"}})
	if err != nil {
		t.Fatal(err)
	}
	want := budget.NewEstimator().Estimate(content)
	if got != want {
		t.Fatalf("baseline=%d, want %d", got, want)
	}
}

func TestPackWithSavingsOmitsUnreadableBaseline(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	bundle, err := a.packWithSavings(context.Background(), model.PackRequest{Query: "missing", TokenBudget: 512, Candidates: []model.RankedCandidate{{Path: "missing.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Savings != nil {
		t.Fatalf("expected savings to be omitted, bundle=%+v", bundle)
	}
	if len(bundle.Diagnostics) != 1 || !strings.Contains(bundle.Diagnostics[0], "token savings unavailable") {
		t.Fatalf("diagnostics=%v", bundle.Diagnostics)
	}
}

func runAppGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}

func writeAppFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
