package rank

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func TestProfileForUsesPrimaryIntentAndCopiesRelationWeights(t *testing.T) {
	callers := ProfileFor(query.Plan{PrimaryIntent: query.IntentCallers})
	if callers.Name != string(query.IntentCallers) || callers.RelationWeights["callers"] <= callers.RelationWeights["tests"] {
		t.Fatalf("callers profile=%+v", callers)
	}
	callers.RelationWeights["callers"] = -1
	again := ProfileFor(query.Plan{PrimaryIntent: query.IntentCallers})
	if again.RelationWeights["callers"] <= 0 {
		t.Fatal("profile relation weights share mutable state")
	}
}

func TestRankWithPlanUsesIntentSpecificOrdering(t *testing.T) {
	tests := []struct {
		name       string
		plan       query.Plan
		candidates []model.RankedCandidate
		want       string
	}{
		{
			name: "definition prefers implementation",
			plan: query.Plan{PrimaryIntent: query.IntentDefinition, Terms: query.Terms{Words: []string{"ValidateToken"}, Identifiers: []string{"ValidateToken"}}},
			candidates: []model.RankedCandidate{
				{Handle: "caller", Symbol: "Authenticate", Kind: "method", Relation: "callers", Content: "service.ValidateToken()"},
				{Handle: "definition", Symbol: "ValidateToken", Kind: "function", Content: "reject expired token"},
			},
			want: "definition",
		},
		{
			name: "callers prefers relation",
			plan: query.Plan{PrimaryIntent: query.IntentCallers, Terms: query.Terms{Words: []string{"ValidateToken"}, Identifiers: []string{"ValidateToken"}}},
			candidates: []model.RankedCandidate{
				{Handle: "definition", Symbol: "ValidateToken", Kind: "function", Content: "reject expired token"},
				{Handle: "caller", Symbol: "Authenticate", Kind: "method", Relation: "callers", Content: "service.ValidateToken()"},
			},
			want: "caller",
		},
		{
			name: "callees prefers relation",
			plan: query.Plan{PrimaryIntent: query.IntentCallees, Terms: query.Terms{Words: []string{"ValidateToken"}, Identifiers: []string{"ValidateToken"}}},
			candidates: []model.RankedCandidate{
				{Handle: "noise", Symbol: "ValidateToken", Kind: "function", Content: "text match"},
				{Handle: "callee", Symbol: "loadClock", Kind: "function", Relation: "callees", Content: "clock.Now()"},
			},
			want: "callee",
		},
		{
			name: "tests prefers relation",
			plan: query.Plan{PrimaryIntent: query.IntentTests, Terms: query.Terms{Words: []string{"ValidateToken"}, Identifiers: []string{"ValidateToken"}}},
			candidates: []model.RankedCandidate{
				{Handle: "production", Symbol: "ValidateToken", Kind: "function", Content: "reject expired token"},
				{Handle: "test", Symbol: "TestValidateToken", Kind: "test", Relation: "tests", Content: "ValidateToken rejects expired token"},
			},
			want: "test",
		},
		{
			name: "imports prefers relation",
			plan: query.Plan{PrimaryIntent: query.IntentImports, Terms: query.Terms{Words: []string{"auth"}}},
			candidates: []model.RankedCandidate{
				{Handle: "docs", Symbol: "import guide", Kind: "heading", Language: "markdown", Content: "import auth"},
				{Handle: "importer", Symbol: "handle", Kind: "method", Relation: "imports", Content: "require auth.php"},
			},
			want: "importer",
		},
		{
			name: "references prefers relation",
			plan: query.Plan{PrimaryIntent: query.IntentReferences, Terms: query.Terms{Words: []string{"Token"}}},
			candidates: []model.RankedCandidate{
				{Handle: "noise", Symbol: "Report", Kind: "function", Content: "Token appears in a report"},
				{Handle: "reference", Symbol: "Handler", Kind: "class", Relation: "references", Content: "Token token"},
			},
			want: "reference",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RankWithPlan(test.candidates, test.plan)
			if len(got) < 2 || got[0].Handle != test.want {
				t.Fatalf("ranked=%+v, want first %q", got, test.want)
			}
			if test.plan.PrimaryIntent == query.IntentCallers && len(got) < 2 {
				t.Fatal("definition anchor was not retained")
			}
		})
	}
}

func TestRankWithPlanUsesJapaneseTestIntentAndFusionReason(t *testing.T) {
	plan := query.PlanQuery("期限切れtokenを検証するテスト")
	got := RankWithPlan([]model.RankedCandidate{{
		Handle: "test", Path: "tests/token_test.go", Symbol: "TestValidateToken", Kind: "test", Relation: "tests", RetrievalScore: .05,
	}}, plan)
	if len(got) != 1 || got[0].Score <= 0 {
		t.Fatalf("ranked=%+v", got)
	}
	for _, reason := range got[0].Reasons {
		if reason.Code == "test-context-penalty" {
			t.Fatalf("Japanese test intent was penalized: %+v", got[0].Reasons)
		}
	}
	counts := map[string]int{}
	for _, reason := range got[0].Reasons {
		counts[reason.Code]++
	}
	for code, count := range counts {
		if count > 1 {
			t.Fatalf("reason %q repeated in %+v", code, got[0].Reasons)
		}
	}
	if !strings.Contains(reasonCodes(got[0].Reasons), "retrieval-fusion") {
		t.Fatalf("fusion reason missing: %+v", got[0].Reasons)
	}
}

func TestRankWithPlanPenalizesUnrelatedTestsForNonTestQuery(t *testing.T) {
	plan := query.PlanQuery("ValidateToken implementation")
	got := RankWithPlan([]model.RankedCandidate{
		{Handle: "test", Path: "tests/token_test.go", Symbol: "TestValidateToken", Kind: "test", Content: "ValidateToken"},
		{Handle: "production", Path: "auth/token.go", Symbol: "ValidateToken", Kind: "function", Content: "implementation"},
	}, plan)
	if len(got) != 2 || got[0].Handle != "production" {
		t.Fatalf("ranked=%+v", got)
	}
}

func TestRankWithPlanRecognizesSimpleInflectedSymbol(t *testing.T) {
	plan := query.Plan{PrimaryIntent: query.IntentImports, Terms: query.Terms{Words: []string{"bootstraps"}}}
	got := RankWithPlan([]model.RankedCandidate{{Handle: "anchor", Symbol: "bootstrap", Kind: "function", Signature: "function bootstrap()", Content: "authentication"}}, plan)
	if len(got) != 1 || !hasReason(got[0], "symbol-prefix") {
		t.Fatalf("ranked=%+v", got)
	}
}

func reasonCodes(reasons []model.ScoreReason) string {
	values := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		values = append(values, reason.Code)
	}
	return strings.Join(values, ",")
}
