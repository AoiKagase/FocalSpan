package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/search"
)

type fakeQueryer struct {
	calls    int
	requests []app.QueryRequest
}

func (f *fakeQueryer) Query(_ context.Context, request app.QueryRequest) (model.ContextBundle, error) {
	f.calls++
	f.requests = append(f.requests, request)
	return model.ContextBundle{BudgetTokens: 100, EstimatedTokens: 40, Items: []model.ContextItem{{Path: "auth/service.go", Symbol: "ValidateToken", Kind: "test", Relation: "tests"}}}, nil
}

func (f *fakeQueryer) BaselineTokensForRequest(context.Context, app.QueryRequest) (int, error) {
	return 200, nil
}

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

func TestEvaluateUsesExpectedPathWhenNoExpectedSymbolIsProvided(t *testing.T) {
	queryer := &fakeQueryer{}
	report, err := Evaluate(context.Background(), queryer, []Case{{Name: "imports", Query: "imports", TokenBudget: 100, ExpectedPaths: []string{"auth/service.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Cases[0].HitAt1 != 1 || report.Cases[0].HitAt5 != 1 {
		t.Fatalf("report=%+v", report)
	}
}

func TestEvaluateModeTracksIntentRelationKindAndPassesModeToBothCalls(t *testing.T) {
	queryer := &fakeQueryer{}
	report, err := EvaluateMode(context.Background(), queryer, []Case{{
		Name: "Japanese tests", Query: "ValidateTokenを検証するテスト", TokenBudget: 100,
		ExpectedIntents: []string{"tests"}, ExpectedRelations: []string{"tests"}, ExpectedKinds: []string{"test"},
	}}, search.RetrievalFTSOnly)
	if err != nil {
		t.Fatal(err)
	}
	result := report.Cases[0]
	if result.IntentRecall != 1 || result.RelationRecall != 1 || result.KindRecall != 1 || result.RetrievalMode != string(search.RetrievalFTSOnly) {
		t.Fatalf("result=%+v report=%+v", result, report)
	}
	if len(queryer.requests) != 2 || queryer.requests[0].RetrievalMode != search.RetrievalFTSOnly || queryer.requests[1].RetrievalMode != search.RetrievalFTSOnly {
		t.Fatalf("requests=%+v", queryer.requests)
	}
}

func TestEvaluateGroupsPrimaryIntentMetricsByOriginalCase(t *testing.T) {
	queryer := &fakeQueryer{}
	report, err := Evaluate(context.Background(), queryer, []Case{
		{Name: "callers one", Query: "what calls ValidateToken?", ExpectedIntents: []string{"callers"}},
		{Name: "definition", Query: "ValidateToken", ExpectedIntents: []string{"callers"}},
		{Name: "callers two", Query: "what calls ValidateToken?", ExpectedIntents: []string{"callers"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	callers := report.ByPrimaryIntent["callers"]
	if callers.Cases != 2 || callers.IntentRecall != 1 {
		t.Fatalf("callers report=%+v, full report=%+v", callers, report)
	}
	definition := report.ByPrimaryIntent["definition"]
	if definition.Cases != 1 || definition.IntentRecall != 0 {
		t.Fatalf("definition report=%+v, full report=%+v", definition, report)
	}
}

func TestEvaluateRetrievalModesImproveJapaneseRelationRecall(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "repos", "authsample"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.IndexDirectory = ".focalspan-eval-test"
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(root, cfg.IndexDirectory)) })
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	cases := []Case{{
		Name: "Japanese callers", Query: "ValidateToken の呼び出し元はどこですか", TokenBudget: 1200,
		ExpectedPaths: []string{"http/middleware.go"}, ExpectedIntents: []string{"callers"}, ExpectedRelations: []string{"callers"},
	}}
	full, err := EvaluateMode(context.Background(), service, cases, search.RetrievalFull)
	if err != nil {
		t.Fatal(err)
	}
	ftsOnly, err := EvaluateMode(context.Background(), service, cases, search.RetrievalFTSOnly)
	if err != nil {
		t.Fatal(err)
	}
	noRelations, err := EvaluateMode(context.Background(), service, cases, search.RetrievalNoRelations)
	if err != nil {
		t.Fatal(err)
	}
	if full.Cases[0].HitAt3 < ftsOnly.Cases[0].HitAt3 || full.RelationRecall <= ftsOnly.RelationRecall || noRelations.RelationRecall != 0 {
		t.Fatalf("full=%+v fts-only=%+v no-relations=%+v", full, ftsOnly, noRelations)
	}
}

func TestEvaluateJapaneseJSTSFixtureAcrossRetrievalModes(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "repos", "jstssample"))
	if err != nil {
		t.Fatal(err)
	}
	casesPath := filepath.Join("..", "..", "testdata", "eval", "ja-jsts-cases.jsonl")
	cases, err := LoadCases(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.IndexDirectory = ".focalspan-jsts-eval-test"
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(root, cfg.IndexDirectory)) })
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	full, err := EvaluateMode(context.Background(), service, cases, search.RetrievalFull)
	if err != nil {
		t.Fatal(err)
	}
	ftsOnly, err := EvaluateMode(context.Background(), service, cases, search.RetrievalFTSOnly)
	if err != nil {
		t.Fatal(err)
	}
	noRelations, err := EvaluateMode(context.Background(), service, cases, search.RetrievalNoRelations)
	if err != nil {
		t.Fatal(err)
	}
	if full.IntentRecall != 1 || full.HitAt5 != 1 || full.RelationRecall != 1 || full.BudgetCompliance != 1 || full.DeterministicOutput != 1 || full.ForbiddenPathViolations != 0 {
		t.Fatalf("full=%+v", full)
	}
	if full.HitAt3 < ftsOnly.HitAt3 || full.RelationRecall <= ftsOnly.RelationRecall || noRelations.RelationRecall != 0 {
		t.Fatalf("full=%+v fts-only=%+v no-relations=%+v", full, ftsOnly, noRelations)
	}
}
