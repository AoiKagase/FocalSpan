package render

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCompactRenderContainsPathSymbolAndReasons(t *testing.T) {
	bundle := model.ContextBundle{Query: "token", BudgetTokens: 512, EstimatedTokens: 20, Items: []model.ContextItem{{Handle: "h", Path: "auth.go", Symbol: "ValidateToken", StartLine: 3, EndLine: 5, Reasons: []model.ScoreReason{{Code: "symbol-exact", Weight: 100}}, Content: "func ValidateToken() {}"}}}
	text := Compact(bundle)
	for _, part := range []string{"auth.go:3-5", "ValidateToken", "symbol-exact", "func ValidateToken"} {
		if !strings.Contains(text, part) {
			t.Fatalf("compact output missing %q: %s", part, text)
		}
	}
}

func TestCompactRenderShowsTokenSavings(t *testing.T) {
	bundle := model.ContextBundle{
		BudgetTokens:    512,
		EstimatedTokens: 320,
		Savings:         &model.TokenSavings{BaselineTokens: 1280, SavedTokens: 960, SavingsRatio: 0.75},
	}
	output := Compact(bundle)
	for _, want := range []string{"estimated: 320", "baseline: 1280", "saved: 960 tokens (75.0%)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output=%q missing %q", output, want)
		}
	}
}
