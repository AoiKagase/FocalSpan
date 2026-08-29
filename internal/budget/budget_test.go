package budget

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestEstimatorIsConservativeForCodeAndUnicode(t *testing.T) {
	e := NewEstimator()
	ascii := e.Estimate("func ValidateToken(token string) error { return nil }")
	nonascii := e.Estimate("トークンの有効期限を検証します")
	if ascii <= len("func ValidateToken(token string) error { return nil }")/4 || nonascii <= 0 || nonascii == ascii {
		t.Fatalf("ascii=%d nonascii=%d", ascii, nonascii)
	}
}

func TestPackerKeepsFinalPayloadWithinBudgetAndCutsWholeLines(t *testing.T) {
	content := strings.Repeat("line with ValidateToken and expired token\n", 120)
	candidates := []model.RankedCandidate{{Handle: "one", Path: "auth/service.go", Language: "go", Kind: "function", Symbol: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 120, Score: 120, Confidence: 1, Content: content}}
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{Query: "expired token", TokenBudget: 256, Mode: "source", Candidates: candidates})
	if bundle.BudgetTokens != 256 || bundle.EstimatedTokens > bundle.BudgetTokens || len(bundle.Items) != 1 || !bundle.Items[0].Elided {
		t.Fatalf("bundle=%+v", bundle)
	}
	if strings.Contains(bundle.Items[0].Content, "\n\n") || (bundle.Items[0].Content != "" && !strings.HasSuffix(bundle.Items[0].Content, "\n") && !strings.HasSuffix(bundle.Items[0].Content, "token")) {
		t.Fatalf("content cut at unsafe boundary: %q", bundle.Items[0].Content)
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if NewEstimator().Estimate(string(payload)) > bundle.BudgetTokens {
		t.Fatalf("final payload exceeds budget: %d", NewEstimator().Estimate(string(payload)))
	}
}

func TestPackerOmitsContentOnlyNoiseWhenAConcreteStructuralMatchExists(t *testing.T) {
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{
		Query:       "where is an expired authentication token rejected?",
		TokenBudget: 1200,
		Mode:        "source",
		Candidates: []model.RankedCandidate{
			{Handle: "target", Path: "auth/service.go", Language: "go", Kind: "function", Symbol: "ValidateToken", Score: 80, Reasons: []model.ScoreReason{{Code: "symbol-prefix"}}, Content: "func ValidateToken() error { return ErrExpired }"},
			{Handle: "noise", Path: "unrelated/report.go", Language: "go", Kind: "function", Symbol: "BuildReport", Score: 55, Reasons: []model.ScoreReason{{Code: "lexical"}}, Content: "func BuildReport() {}"},
		},
	})
	for _, item := range bundle.Items {
		if item.Path == "unrelated/report.go" {
			t.Fatalf("content-only noise was retained: %+v", bundle.Items)
		}
	}
}

func TestOutlineModeOmitsSourceBody(t *testing.T) {
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{Query: "token", TokenBudget: 512, Mode: "outline", Candidates: []model.RankedCandidate{{Handle: "h", Path: "a.go", Symbol: "Run", Signature: "func Run", StartLine: 2, EndLine: 4, Content: "secret source"}}})
	if len(bundle.Items) != 1 || bundle.Items[0].Content != "" {
		t.Fatalf("outline=%+v", bundle)
	}
}

func TestPackerOmitsLowUtilityCandidateWhenBudgetIsTight(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "top", Path: "auth/service.go", Symbol: "ValidateToken", Score: 100, Content: strings.Repeat("important validation line\n", 20)},
		{Handle: "low", Path: "unrelated/report.go", Symbol: "BuildReport", Score: 10, Content: strings.Repeat("unrelated report line\n", 10)},
		{Handle: "second", Path: "auth/service_test.go", Symbol: "TestValidateToken", Score: 90, Content: strings.Repeat("important regression line\n", 20)},
	}
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{Query: "expired token", TokenBudget: 512, Mode: "source", Candidates: candidates})
	foundSecond := false
	for _, item := range bundle.Items {
		if item.Path == "unrelated/report.go" {
			t.Fatalf("low utility candidate crowded out the top result: %+v", bundle)
		}
		if item.Path == "auth/service_test.go" {
			foundSecond = true
		}
	}
	if !foundSecond {
		t.Fatalf("higher utility candidate was omitted: %+v", bundle)
	}
}

func TestPackerKeepsStructuredAnchorAlongsideRelationCandidate(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "caller", Path: "src/http/middleware.php", Language: "php", Kind: "method", Symbol: "handle", Score: 524, Relation: "imports", Content: "require 'includes/bootstrap.inc';"},
		{Handle: "anchor", Path: "includes/bootstrap.inc", Language: "php", Kind: "function", Symbol: "bootstrap", Score: 36, Reasons: []model.ScoreReason{{Code: "symbol-prefix", Weight: 24}}, Content: "function bootstrap(): void {}"},
	}
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{Query: "which include file bootstraps authentication?", TokenBudget: 1200, Mode: "source", Candidates: candidates})
	for _, item := range bundle.Items {
		if item.Symbol == "bootstrap" && item.Path == "includes/bootstrap.inc" {
			return
		}
	}
	t.Fatalf("structured anchor was omitted: %+v", bundle)
}

func TestPackerUsesJapaneseTestIntentHint(t *testing.T) {
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{
		Query:       "ValidateTokenを検証するテスト",
		TokenBudget: 1200,
		Mode:        "source",
		IntentHints: []string{"tests"},
		Candidates: []model.RankedCandidate{
			{Handle: "production", Path: "auth/token.go", Language: "go", Kind: "function", Symbol: "ValidateToken", Score: 100, Content: "return nil"},
			{Handle: "test", Path: "tests/token_test.go", Language: "go", Kind: "test", Symbol: "TestValidateToken", Relation: "tests", Score: 90, Content: "ValidateToken test"},
		},
	})
	for _, item := range bundle.Items {
		if item.Kind == "test" {
			return
		}
	}
	t.Fatalf("test candidate was dropped despite tests intent: %+v", bundle)
}

func TestPackerLimitsTestIntentToFocusedTestChunks(t *testing.T) {
	candidates := []model.RankedCandidate{
		{Handle: "one", Path: "tests/one.test.ts", Kind: "test", Symbol: "expired token is rejected", Score: 100, Content: "test(\"expired token is rejected\", () => {});"},
		{Handle: "docs", Path: "README.md", Language: "markdown", Kind: "heading", Score: 90, Content: "tests and TypeScript"},
		{Handle: "two", Path: "tests/two.test.ts", Kind: "test", Symbol: "accepts live token", Score: 80, Content: "test(\"accepts live token\", () => {});"},
		{Handle: "three", Path: "tests/three.test.ts", Kind: "test", Symbol: "rejects empty token", Score: 70, Content: "test(\"rejects empty token\", () => {});"},
	}
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{Query: "what tests cover expired tokens?", TokenBudget: 512, Mode: "source", Candidates: candidates})
	if len(bundle.Items) != 2 || bundle.Items[0].Path != "tests/one.test.ts" || bundle.Items[1].Path != "tests/two.test.ts" {
		t.Fatalf("bundle=%+v", bundle)
	}
}

func TestPackerPreservesRelationMetadata(t *testing.T) {
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{
		Query:       "what calls Handle?",
		TokenBudget: 512,
		Mode:        "source",
		Candidates: []model.RankedCandidate{{
			Handle:   "caller",
			Path:     "http/middleware.go",
			Language: "go",
			Kind:     "function",
			Symbol:   "Handle",
			Relation: "callers",
			Content:  "func Handle() {}",
		}},
	})
	if len(bundle.Items) != 1 {
		t.Fatalf("items=%+v", bundle.Items)
	}
	if bundle.Items[0].Relation != "callers" {
		t.Fatalf("relation=%q, want callers", bundle.Items[0].Relation)
	}
}
