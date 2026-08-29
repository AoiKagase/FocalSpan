package rank

import (
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestRankPrefersExactSymbolAndKeepsReasons(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "chunk-test", Path: "auth/service_test.go", Symbol: "TestValidateExpiredToken", Kind: "test", Content: "expired token"},
		{Handle: "chunk-prod", Path: "auth/service.go", Symbol: "ValidateToken", Kind: "function", Signature: "func ValidateToken", Content: "reject expired token"},
	}
	got := Rank(candidates, []string{"where", "expired", "token", "ValidateToken"})
	if len(got) != 2 || got[0].Symbol != "ValidateToken" || len(got[0].Reasons) == 0 || got[0].Score <= got[1].Score {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankDoesNotUseNaturalLanguageWordsAsSymbolPrefixes(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "test", Path: "auth_test.php", Language: "php", Symbol: "testExpiredTokenIsRejected", Kind: "test", Content: "a test"},
		{Handle: "production", Path: "auth.php", Language: "php", Symbol: "validateToken", Kind: "method", Content: "rejects expired authentication tokens"},
	}
	got := Rank(candidates, []string{"where", "expired", "token", "rejected"})
	if len(got) != 2 || got[0].Symbol != "validateToken" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankRetainsIdentifierPrefixesForGoCandidates(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "unrelated", Path: "unrelated/report.go", Language: "go", Symbol: "BuildReport", Content: "rejects expired authentication token records"},
		{Handle: "production", Path: "auth/service.go", Language: "go", Symbol: "ValidateToken", Kind: "method", Content: "rejects expired authentication tokens"},
	}
	got := Rank(candidates, []string{"expired", "token"})
	if len(got) != 2 || got[0].Symbol != "ValidateToken" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankUsesExplicitPHPIdentifierTermsForSymbolPrefixes(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "unrelated", Path: "unrelated/report.php", Language: "php", Symbol: "BuildReport", Content: "rejects expired authentication token records"},
		{Handle: "production", Path: "src/Auth/TokenService.php", Language: "php", Symbol: "validateToken", Kind: "method", Content: "rejects expired authentication tokens"},
	}
	got := RankWithIdentifiers(candidates, []string{"validate", "token"}, []string{"validateToken"})
	if len(got) != 2 || got[0].Symbol != "validateToken" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankIgnoresSingleLetterLanguageNameAsSymbolPrefix(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "noise", Path: "noise.cpp", Symbol: "C", Content: ""},
		{Handle: "target", Path: "auth.cpp", Symbol: "ValidateToken", Content: "expired"},
	}
	got := RankWithIdentifiers(candidates, []string{"where", "c", "expired"}, []string{"C"})
	if len(got) == 0 || got[0].Handle != "target" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankDoesNotBoostTestsForDefinitionQuestion(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "test", Path: "auth_test.cpp", Symbol: "RejectsExpiredToken", Kind: "test", Content: "expired token rejected"},
		{Handle: "production", Path: "auth.cpp", Symbol: "ValidateToken", Kind: "method", Content: "expired token rejected"},
	}
	got := Rank(candidates, []string{"where", "expired", "token", "rejected"})
	if len(got) == 0 || got[0].Handle != "production" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankDoesNotUseDocumentationHeadingAsCodeSymbolPrefix(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "docs", Path: "README.md", Language: "markdown", Kind: "heading", Symbol: "JavaScript and TypeScript", Content: "token"},
		{Handle: "source", Path: "auth.ts", Language: "typescript", Kind: "function", Symbol: "validateToken", Content: "expired token"},
	}
	got := Rank(candidates, []string{"tests", "expired", "TypeScript", "token"})
	if len(got) == 0 || got[0].Handle != "source" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankKeepsExactOutlineWhenSpecificChunkSharesPath(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "outline", Path: "Auth/TokenService.Partial.cs", Kind: "class-outline", Symbol: "TokenService", Reasons: []model.ScoreReason{{Code: "symbol-exact"}}},
		{Handle: "method", Path: "Auth/TokenService.Partial.cs", Kind: "method", Symbol: "ValidateForHeader"},
	}
	got := Deduplicate(candidates)
	if len(got) != 2 {
		t.Fatalf("deduped=%+v", got)
	}
}

func TestRankDeduplicatesContentAndContainedLowerScore(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "whole", Path: "a.go", Symbol: "Run", StartLine: 1, EndLine: 20, ContentHash: "same", Score: 10, Confidence: .8},
		{Handle: "duplicate", Path: "a.go", Symbol: "Run", StartLine: 1, EndLine: 20, ContentHash: "same", Score: 9, Confidence: .8},
		{Handle: "inner", Path: "a.go", Symbol: "Run", StartLine: 3, EndLine: 5, ContentHash: "inner", Score: 1, Confidence: .8},
	}
	got := Deduplicate(candidates)
	if len(got) != 1 || got[0].Handle != "whole" {
		t.Fatalf("deduped=%+v", got)
	}
}

func TestRankDropsNonRelationOutlineWhenSpecificChunkExists(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "outline", Path: "auth.cpp", Kind: "translation-unit-outline", StartLine: 1, EndLine: 1, Score: 20},
		{Handle: "method", Path: "auth.cpp", Kind: "method", Symbol: "ValidateToken", StartLine: 4, EndLine: 8, Score: 10},
	}
	got := Deduplicate(candidates)
	if len(got) != 1 || got[0].Handle != "method" {
		t.Fatalf("deduped=%+v", got)
	}
}

func TestRankDropsLowerScoreContainerWhenSpecificChildIsPreferred(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "method", Path: "TokenService.cs", Symbol: "Validate", StartLine: 10, EndLine: 16, ContentHash: "method", Score: 100, Confidence: .55},
		{Handle: "class", Path: "TokenService.cs", Symbol: "TokenService", StartLine: 1, EndLine: 30, ContentHash: "class", Score: 8, Confidence: .55},
	}
	got := Deduplicate(candidates)
	if len(got) != 1 || got[0].Handle != "method" {
		t.Fatalf("deduped=%+v", got)
	}
}

func TestRankTieBreaksBySpanThenPathThenHandle(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "b", Path: "z.go", StartLine: 1, EndLine: 2, Confidence: .5},
		{Handle: "a", Path: "a.go", StartLine: 3, EndLine: 4, Confidence: .5},
	}
	got := Rank(candidates, nil)
	if got[0].Path != "a.go" {
		t.Fatalf("tie order=%+v", got)
	}
}
