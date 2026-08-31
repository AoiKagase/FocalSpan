package benchmark

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

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

func TestCompareReportsSortsKeysAndRequiresExactResultMatrix(t *testing.T) {
	base := RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{
		{CaseID: "z", Profile: "p", Budget: 200, RequiredPathRecall: 1},
		{CaseID: "a", Profile: "q", Budget: 100, RequiredPathRecall: 1},
		{CaseID: "a", Profile: "p", Budget: 200, RequiredPathRecall: 1},
		{CaseID: "a", Profile: "p", Budget: 100, RequiredPathRecall: 1},
	}}
	candidate := base
	candidate.Quality = append([]QualityResult(nil), base.Quality...)
	for index := range candidate.Quality {
		candidate.Quality[index].RequiredPathRecall = 0
	}
	want := []string{"a/p/100", "a/p/200", "a/q/100", "z/p/200"}
	for attempt := 0; attempt < 20; attempt++ {
		comparison := CompareReports(base, candidate)
		got := make([]string, 0, len(comparison.Regressions))
		for _, regression := range comparison.Regressions {
			got = append(got, regression.CaseID+"/"+regression.Profile+"/"+strconv.Itoa(regression.Budget))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("regression order=%v want=%v", got, want)
		}
	}
	extra := candidate
	extra.Quality = append(extra.Quality, QualityResult{CaseID: "extra", Profile: "p", Budget: 100})
	if comparison := CompareReports(base, extra); comparison.Compatible || !strings.Contains(strings.Join(comparison.Warnings, " "), "matrix") {
		t.Fatalf("extra result comparison=%+v", comparison)
	}
}

func TestCompareReportsCoversQualityContractAndWireThreshold(t *testing.T) {
	left := QualityResult{CaseID: "c", Profile: "p", Budget: 100, RequiredPathRecall: 1, RequiredSymbolRecall: 1, IntentCorrect: 1, RoleAccuracy: 1, RelationValid: 1, BudgetCompliant: 1, Deterministic: 1, WireTokens: 100}
	right := left
	right.IntentCorrect = 0
	right.RoleAccuracy = .5
	right.RelationValid = 0
	right.WireTokens = 111
	comparison := CompareReports(RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{left}}, RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{right}})
	if len(comparison.Regressions) != 1 {
		t.Fatalf("comparison=%+v", comparison)
	}
	details := strings.Join(comparison.Regressions[0].Details, " ")
	for _, want := range []string{"intent_correct", "role_accuracy", "relation_valid", "wire_tokens"} {
		if !strings.Contains(details, want) {
			t.Fatalf("details %q missing %q", details, want)
		}
	}
	right.RequiredPathRecall = 1.1
	comparison = CompareReports(RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{left}}, RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: []QualityResult{right}})
	if strings.Contains(strings.Join(comparison.Regressions[0].Details, " "), "wire_tokens") {
		t.Fatalf("wire increase should be allowed with recall improvement: %+v", comparison)
	}
}

func TestCompareReportsPerformanceSlowdownIsWarningOnly(t *testing.T) {
	quality := []QualityResult{{CaseID: "c", Profile: "p", Budget: 100}}
	base := RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: quality, Performance: []PerformanceResult{{CaseID: "c", Profile: "p", Budget: 100, IndexMS: 100, QueryMS: []int64{10, 20, 30}}}}
	candidate := RunReport{Schema: ReportSchemaV1, Suite: "s", Quality: quality, Performance: []PerformanceResult{{CaseID: "c", Profile: "p", Budget: 100, IndexMS: 121, QueryMS: []int64{25, 25, 25}}}}
	comparison := CompareReports(base, candidate)
	if !comparison.Compatible || len(comparison.Regressions) != 0 || len(comparison.Warnings) != 2 {
		t.Fatalf("comparison=%+v", comparison)
	}
}
