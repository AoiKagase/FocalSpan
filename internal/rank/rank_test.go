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
