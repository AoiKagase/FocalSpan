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
