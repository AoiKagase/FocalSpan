package benchmark

import (
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestMetricResultFromPacket(t *testing.T) {
	packet := evidence.Packet{Intent: "definition", Budget: evidence.Budget{Limit: 1000, Used: 400}, Evidence: []evidence.Item{{ID: "e1", Role: evidence.RoleTarget, Location: evidence.Location{Path: "a.go"}, Symbol: "Target", Source: "same"}, {ID: "e2", Location: evidence.Location{Path: "b.go"}, Source: "same"}}}
	result := MeasurePacket(Case{ID: "c", ExpectedIntent: "definition", RequiredPaths: []string{"a.go", "missing.go"}, OptionalPaths: []string{}, ForbiddenPaths: []string{"b.go"}, RequiredSymbols: []SymbolExpectation{{Path: "a.go", Name: "Target", Role: "target"}}}, "p", 1000, packet, true, nil)
	if result.TargetRank != 1 || result.ReciprocalRank != 1 || result.RequiredPathRecall != .5 || result.OptionalPathRecall != 1 || result.RequiredSymbolRecall != 1 || result.IntentCorrect != 1 || result.BudgetCompliant != 1 || result.ForbiddenViolations != 1 || result.DuplicateSourceRatio <= 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAggregateQuality(t *testing.T) {
	aggregate := AggregateResults([]QualityResult{{Profile: "p", Budget: 100, TargetRank: 1, RequiredPathRecall: 1, BudgetCompliant: 1, Deterministic: 1}, {Profile: "p", Budget: 100, TargetRank: 0, RequiredPathRecall: 0, BudgetCompliant: 1, Deterministic: 1}})
	if len(aggregate.Groups) != 1 || aggregate.Groups[0].HitAt1 != .5 || aggregate.Groups[0].MedianRequiredPathRecall != .5 {
		t.Fatalf("aggregate = %+v", aggregate)
	}
}
