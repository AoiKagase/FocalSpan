package benchmark

import "testing"

func TestCompareReportsIgnoresPerformanceAndFindsRegression(t *testing.T) {
	base := RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{{CaseID: "c", Profile: "p", Budget: 100, RequiredPathRecall: 1, BudgetCompliant: 1, Deterministic: 1}}, Performance: []PerformanceResult{{SnapshotMS: 1}}}
	candidate := base
	candidate.Performance = []PerformanceResult{{SnapshotMS: 999}}
	comparison := CompareReports(base, candidate)
	if !comparison.Compatible || len(comparison.Regressions) != 0 {
		t.Fatalf("timing comparison=%+v", comparison)
	}
	candidate.Quality = []QualityResult{{CaseID: "c", Profile: "p", Budget: 100, RequiredPathRecall: .5, BudgetCompliant: 1, Deterministic: 1}}
	comparison = CompareReports(base, candidate)
	if !comparison.Compatible || len(comparison.Regressions) != 1 {
		t.Fatalf("quality comparison=%+v", comparison)
	}
}
func TestCompareReportsRejectsSchemaMismatch(t *testing.T) {
	comparison := CompareReports(RunReport{Schema: ReportSchemaV1, Suite: "s"}, RunReport{Schema: "other", Suite: "s"})
	if comparison.Compatible {
		t.Fatalf("comparison=%+v", comparison)
	}
}
