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
