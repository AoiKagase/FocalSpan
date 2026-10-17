package benchmark

import (
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestTargetRankUsesRequiredLabels(t *testing.T) {
	packet := evidence.Packet{Evidence: []evidence.Item{
		{ID: "e1", Role: evidence.RoleTarget, Location: evidence.Location{Path: "wrong.go"}, Symbol: "Wrong", Kind: "function"},
		{ID: "e2", Role: evidence.RoleTarget, Location: evidence.Location{Path: "correct.go"}, Symbol: "Other", Kind: "function"},
		{ID: "e3", Role: evidence.RoleCaller, Location: evidence.Location{Path: "correct.go"}, Symbol: "Wanted", Kind: "method"},
	}}
	for _, tc := range []struct {
		name string
		c    Case
		rank int
	}{
		{"unrelated", Case{RequiredPaths: []string{"missing.go"}}, 0},
		{"path", Case{RequiredPaths: []string{"correct.go"}}, 2},
		{"symbol priority", Case{RequiredPaths: []string{"correct.go"}, RequiredSymbols: []SymbolExpectation{{Path: "correct.go", Name: "Wanted"}}}, 3},
		{"role mismatch", Case{RequiredSymbols: []SymbolExpectation{{Path: "correct.go", Name: "Wanted", Role: "target"}}}, 0},
		{"kind mismatch", Case{RequiredSymbols: []SymbolExpectation{{Path: "correct.go", Name: "Wanted", Kind: "function"}}}, 0},
		{"full symbol", Case{RequiredSymbols: []SymbolExpectation{{Path: "correct.go", Name: "Wanted", Kind: "method", Role: "caller"}}}, 3},
		{"no labels", Case{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := MeasurePacket(tc.c, "full", 1000, packet, true, nil)
			wantRR := 0.0
			if tc.rank > 0 {
				wantRR = 1 / float64(tc.rank)
			}
			if got.TargetRank != tc.rank || got.ReciprocalRank != wantRR {
				t.Fatalf("rank=%d RR=%v want=%d/%v", got.TargetRank, got.ReciprocalRank, tc.rank, wantRR)
			}
		})
	}
}
func TestReportComparisonVersions(t *testing.T) {
	row := QualityResult{CaseID: "case", Profile: "full", Budget: 1000}
	for _, version := range []string{"focalspan.benchmark-report.v1", "focalspan.benchmark-report.v2"} {
		report := RunReport{Schema: version, Suite: "suite", Quality: []QualityResult{row}}
		if !CompareReports(report, report).Compatible {
			t.Errorf("matching %s reports rejected", version)
		}
	}
	first := RunReport{Schema: "focalspan.benchmark-report.v1", Suite: "suite", Quality: []QualityResult{row}}
	second := first
	second.Schema = "focalspan.benchmark-report.v2"
	if CompareReports(first, second).Compatible {
		t.Fatal("mixed metric definitions accepted")
	}
}
