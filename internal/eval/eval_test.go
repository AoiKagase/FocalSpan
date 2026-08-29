package eval

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/model"
)

type fakeQueryer struct{ calls int }

func (f *fakeQueryer) Query(context.Context, app.QueryRequest) (model.ContextBundle, error) {
	f.calls++
	return model.ContextBundle{BudgetTokens: 100, EstimatedTokens: 40, Items: []model.ContextItem{{Path: "auth/service.go", Symbol: "ValidateToken"}}}, nil
}

func (f *fakeQueryer) BaselineTokens(context.Context, string) (int, error) { return 200, nil }

func TestEvaluateReportsHitsBudgetReductionAndDeterminism(t *testing.T) {
	queryer := &fakeQueryer{}
	report, err := Evaluate(context.Background(), queryer, []Case{{Name: "expired", Query: "expired token", TokenBudget: 100, ExpectedSymbols: []string{"ValidateToken"}, ExpectedPaths: []string{"auth/service.go"}, ForbiddenPaths: []string{"unrelated/report.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Cases[0].HitAt1 != 1 || report.Cases[0].BudgetCompliant != 1 || report.Cases[0].Deterministic != 1 || report.Cases[0].ReductionRatio != .2 {
		t.Fatalf("report=%+v", report)
	}
	if queryer.calls != 2 {
		t.Fatalf("query calls=%d, want repeated run", queryer.calls)
	}
}

func TestEvaluateUsesExpectedPathsForPathOnlyHitMetrics(t *testing.T) {
	queryer := &fakeQueryer{}
	report, err := Evaluate(context.Background(), queryer, []Case{{Name: "path-only", Query: "expired", TokenBudget: 100, ExpectedPaths: []string{"auth/service.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Cases[0].HitAt1 != 1 || report.Cases[0].HitAt5 != 1 {
		t.Fatalf("path-only hit metrics=%+v", report.Cases[0])
	}
}
