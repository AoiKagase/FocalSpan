package benchmark

import "testing"

func TestMeasureUsefulEvidenceCountsPackedLabelsAndUsesCumulativeWireTokens(t *testing.T) {
	caseValue := Case{
		ID:              "case",
		RequiredPaths:   []string{"internal/app/service.go"},
		RequiredSymbols: []SymbolExpectation{{Path: "internal/app/service.go", Name: "Run"}},
		Expand:          []ExpandExpectation{{RequiredPaths: []string{"internal/mcpserver/server.go"}, RequiredSymbols: []SymbolExpectation{{Path: "internal/mcpserver/server.go", Name: "codeContext"}}}},
	}
	result := QualityResult{CaseID: "case", RequiredPathRecall: 1, RequiredSymbolRecall: 1, ExpandRequiredPathRecall: 1, ExpandRequiredSymbolRecall: 1, WireTokens: 80, CumulativeWireTokens: 120}
	got := MeasureUsefulEvidence(caseValue, result)
	if got != 4 {
		t.Fatalf("useful evidence=%d, want 4", got)
	}
	summary := AggregateEvidenceEfficiency([]Case{caseValue}, []QualityResult{result})
	if summary.UsefulEvidence != 4 || summary.EstimatedTokens != 120 || summary.Per1000Tokens != 1000.0/30.0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestAggregateEvidenceEfficiencyHandlesZeroTokens(t *testing.T) {
	summary := AggregateEvidenceEfficiency([]Case{{ID: "case"}}, []QualityResult{{CaseID: "case", RequiredPathRecall: 1}})
	if summary.Per1000Tokens != 0 || summary.EstimatedTokens != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestMeasureUsefulEvidenceDeduplicatesPackedLabels(t *testing.T) {
	caseValue := Case{
		ID:            "case",
		RequiredPaths: []string{"a.go", "a.go"},
		RequiredSymbols: []SymbolExpectation{
			{Path: "a.go", Name: "Target"},
			{Path: "a.go", Name: "Target"},
		},
	}
	result := QualityResult{CaseID: "case", RequiredPathRecall: 1, RequiredSymbolRecall: 1}
	if got := MeasureUsefulEvidence(caseValue, result); got != 2 {
		t.Fatalf("useful evidence=%d, want two unique labels", got)
	}
}
