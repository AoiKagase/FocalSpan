package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func audit50Report(schema string, q QualityResult) RunReport {
	q.CaseID = "x"
	q.Profile = "p"
	q.Budget = 1000
	return RunReport{Schema: schema, Suite: "s", Quality: []QualityResult{q}}
}
func TestF47RankRegression(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		schema                  string
		a, b                    int
		regression, improvement bool
	}{
		{"lower", ReportSchemaV2, 1, 5, true, false}, {"miss", ReportSchemaV2, 1, 0, true, false}, {"found", ReportSchemaV2, 0, 1, false, true}, {"legacy", ReportSchemaV1, 1, 5, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := QualityResult{TargetRank: tc.a}
			b := QualityResult{TargetRank: tc.b}
			if tc.a > 0 {
				a.ReciprocalRank = 1 / float64(tc.a)
			}
			if tc.b > 0 {
				b.ReciprocalRank = 1 / float64(tc.b)
			}
			r := CompareReports(audit50Report(tc.schema, a), audit50Report(tc.schema, b))
			if !r.Compatible || (len(r.Regressions) > 0) != tc.regression || (len(r.Improvements) > 0) != tc.improvement {
				t.Fatalf("comparison=%+v", r)
			}
		})
	}
	if CompareReports(audit50Report(ReportSchemaV1, QualityResult{}), audit50Report(ReportSchemaV2, QualityResult{})).Compatible {
		t.Fatal("mixed schemas compatible")
	}
}
func TestF48ExpansionRegression(t *testing.T) {
	a := QualityResult{ExpandRequiredPathRecall: 1, ExpandRequiredSymbolRecall: 1, ExpandRelationValid: 1}
	for _, metric := range []string{"paths", "symbols", "forbidden", "relation", "initial_improves"} {
		t.Run(metric, func(t *testing.T) {
			b := a
			switch metric {
			case "paths":
				b.ExpandRequiredPathRecall = 0
			case "symbols":
				b.ExpandRequiredSymbolRecall = .5
			case "forbidden":
				b.ExpandForbiddenViolations = 1
			case "relation":
				b.ExpandRelationValid = 0
			case "initial_improves":
				b.RequiredPathRecall = 1
				b.ExpandRequiredPathRecall = 0
			}
			r := CompareReports(audit50Report(ReportSchemaV2, a), audit50Report(ReportSchemaV2, b))
			if len(r.Regressions) == 0 {
				t.Fatalf("missing regression: %+v", r)
			}
		})
	}
	if r := CompareReports(audit50Report(ReportSchemaV2, QualityResult{}), audit50Report(ReportSchemaV2, QualityResult{})); len(r.Regressions) != 0 {
		t.Fatal(r)
	}
}
func TestF49ExpansionAnchorPreserved(t *testing.T) {
	q := QualityResult{IntentCorrect: 1, TargetRank: 1, RequiredPathRecall: 1, RequiredSymbolRecall: 1, RelationValid: 1, RoleAccuracy: 1, BudgetCompliant: 1, Deterministic: 1, FailureCodes: []string{"expansion_anchor_missing", "expansion_anchor_missing"}}
	if got := strings.Join(FailureCodes(q), ","); got != "expansion_anchor_missing" {
		t.Fatalf("codes=%s", got)
	}
}

func TestF49ExpansionFailureAccounting(t *testing.T) {
	q := QualityResult{IntentCorrect: 1, TargetRank: 1, RequiredPathRecall: 1, RequiredSymbolRecall: 1, RelationValid: 1, RoleAccuracy: 1, BudgetCompliant: 1, Deterministic: 1}
	for _, tc := range []struct {
		name string
		a    ExpansionAssessment
		want bool
	}{
		{"none", ExpansionAssessment{}, false},
		{"requested_zero", ExpansionAssessment{Requested: 1, RequiredPaths: 1}, true},
		{"missing_anchor", ExpansionAssessment{Requested: 1, MissingAnchors: 1}, true},
		{"partial", ExpansionAssessment{Requested: 2, Completed: 2, RequiredPaths: 3, MatchedPaths: 1}, true},
		{"symbol_only", ExpansionAssessment{Requested: 1, Completed: 1, RequiredSymbols: 1}, true},
		{"all", ExpansionAssessment{Requested: 1, Completed: 1, RequiredPaths: 1, MatchedPaths: 1, RequiredSymbols: 1, MatchedSymbols: 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(FailureCodesWithExpansion(q, tc.a), ",")
			if strings.Contains(got, "expansion_required_missing") != tc.want {
				t.Fatalf("codes=%s assessment=%+v", got, tc.a)
			}
		})
	}
	for _, reverse := range []bool{false, true} {
		a := ExpansionAssessment{Requested: 2, RequiredPaths: 3, RequiredSymbols: 3}
		metrics := []ExpansionMetrics{{MatchedPaths: 1, MatchedSymbols: 1}, {MatchedPaths: 0, MatchedSymbols: 0}}
		if reverse {
			metrics[0], metrics[1] = metrics[1], metrics[0]
		}
		for _, m := range metrics {
			a.add(m)
		}
		actual := q
		a.apply(&actual)
		if actual.ExpandRequiredPathRecall != 1.0/3 || actual.ExpandRequiredSymbolRecall != 1.0/3 {
			t.Fatalf("weighted recall=%+v", actual)
		}
	}
}

func TestF04CIBaselineCompatibility(t *testing.T) {
	data, err := os.ReadFile("../../docs/benchmarks/results-whole-audit-baseline-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline RunReport
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Schema != ReportSchemaV2 || !strings.HasPrefix(baseline.FocalSpanCommit, "9400ffb") {
		t.Fatalf("invalid provenance: schema=%s commit=%s", baseline.Schema, baseline.FocalSpanCommit)
	}
	suite, err := LoadSuite("../../testdata/benchmark/focalspan-history.json")
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Suite != suite.Name {
		t.Fatal("suite mismatch")
	}
	var expected []QualityResult
	for _, c := range suite.Cases {
		for _, p := range DefaultProfiles {
			for _, b := range p.Budgets {
				expected = append(expected, QualityResult{CaseID: c.ID, Profile: p.Name, Budget: b})
			}
		}
	}
	if got := CompareReports(baseline, RunReport{Schema: ReportSchemaV2, Suite: suite.Name, Quality: expected}); !got.Compatible {
		t.Fatalf("matrix incompatible: %+v", got)
	}
	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(workflow), "--baseline docs/benchmarks/results-whole-audit-baseline-v2.json") != 2 {
		t.Fatal("smoke and full must use v2 baseline")
	}
}

func TestF47IndependentRankMetrics(t *testing.T) {
	for _, metric := range []string{"target_rank", "reciprocal_rank"} {
		t.Run(metric, func(t *testing.T) {
			a := QualityResult{TargetRank: 1, ReciprocalRank: 1}
			b := a
			if metric == "target_rank" {
				b.TargetRank = 5
			} else {
				b.ReciprocalRank = .2
			}
			r := CompareReports(audit50Report(ReportSchemaV2, a), audit50Report(ReportSchemaV2, b))
			if len(r.Regressions) != 1 || !strings.Contains(strings.Join(r.Regressions[0].Details, ","), metric) {
				t.Fatalf("comparison=%+v", r)
			}
		})
	}
}
func TestF49RunnerMissingAnchor(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"anchor.go", "related.go"} {
		if err := os.WriteFile(root+"/"+p, []byte("package fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runner := Runner{Snapshotter: &fakeSnapshotter{root: root}, EngineFactory: &fakeEngineFactory{}}
	r, err := runner.Run(context.Background(), RunRequest{Suite: Suite{Name: "s", Cases: []Case{{ID: "c", Repository: "self", BaseRef: "base", TargetRef: "target", Query: "Anchor", Expand: []ExpandExpectation{{From: SymbolExpectation{Path: "anchor.go", Name: "Anchor"}, RequiredPaths: []string{"related.go"}}}}}}, Repositories: map[string]string{"self": root}, Profiles: []Profile{{Name: "p", Contract: "evidence", Budgets: []int{1024}, RunExpansion: true}}, Repeat: 1, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Quality) != 1 {
		t.Fatal(r)
	}
	codes := strings.Join(r.Quality[0].FailureCodes, ",")
	if !strings.Contains(codes, "expansion_anchor_missing") || !strings.Contains(codes, "expansion_required_missing") || r.Quality[0].ExpandRequiredPathRecall != 0 {
		t.Fatalf("quality=%+v", r.Quality[0])
	}
}
