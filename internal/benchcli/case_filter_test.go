package benchcli

import (
	"testing"

	"github.com/focalspan/focalspan/internal/benchmark"
)

func TestSelectSuiteCasesKeepsRequestedOrderAndRejectsUnknownIDs(t *testing.T) {
	suite := benchmark.Suite{Name: "suite", Cases: []benchmark.Case{{ID: "first"}, {ID: "second"}, {ID: "third"}}}

	selected, err := selectSuiteCases(suite, []string{"third", "first"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Cases) != 2 || selected.Cases[0].ID != "third" || selected.Cases[1].ID != "first" {
		t.Fatalf("selected cases = %+v", selected.Cases)
	}
	if _, err := selectSuiteCases(suite, []string{"missing"}); err == nil {
		t.Fatal("expected an unknown case ID error")
	}
}

func TestSelectReportCasesUsesTheSameQualityMatrixSubset(t *testing.T) {
	report := benchmark.RunReport{
		Quality:     []benchmark.QualityResult{{CaseID: "first", Profile: "p", Budget: 100}, {CaseID: "second", Profile: "p", Budget: 100}},
		Performance: []benchmark.PerformanceResult{{CaseID: "first", Profile: "p", Budget: 100}, {CaseID: "second", Profile: "p", Budget: 100}},
	}

	selected, err := selectReportCases(report, []string{"second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Quality) != 1 || selected.Quality[0].CaseID != "second" || len(selected.Performance) != 1 || selected.Performance[0].CaseID != "second" {
		t.Fatalf("selected report = %+v", selected)
	}
	if _, err := selectReportCases(report, []string{"missing"}); err == nil {
		t.Fatal("expected an unknown case ID error")
	}
}
