package budget

import (
	"encoding/json"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestPackerIncludesIndexRevisionInFinalBudget(t *testing.T) {
	content := "func ValidateToken() error { return ErrExpired }\n"
	candidate := model.RankedCandidate{
		Handle:    "sym_validate",
		Path:      "auth/service.go",
		Language:  "go",
		Kind:      "function",
		Symbol:    "ValidateToken",
		Signature: "func ValidateToken() error",
		StartLine: 1,
		EndLine:   1,
		Content:   content,
	}
	bundle := NewPacker(NewEstimator()).Pack(model.PackRequest{
		Query:         "expired token",
		IndexRevision: "0123456789abcdef",
		TokenBudget:   256,
		Mode:          "source",
		Candidates:    []model.RankedCandidate{candidate},
	})
	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if got := NewEstimator().Estimate(string(payload)); got > bundle.BudgetTokens {
		t.Fatalf("final payload exceeds budget: %d > %d; bundle=%+v", got, bundle.BudgetTokens, bundle)
	}
	if bundle.IndexRevision != "0123456789abcdef" {
		t.Fatalf("index revision=%q", bundle.IndexRevision)
	}
}
