package benchmark

import "fmt"

type Comparison struct {
	Compatible   bool         `json:"compatible"`
	Regressions  []Regression `json:"regressions,omitempty"`
	Improvements []Regression `json:"improvements,omitempty"`
	Warnings     []string     `json:"warnings,omitempty"`
}
type Regression struct {
	CaseID  string   `json:"case_id"`
	Profile string   `json:"profile"`
	Budget  int      `json:"budget"`
	Details []string `json:"details"`
}
type qualityKey struct {
	caseID, profile string
	budget          int
}

func CompareReports(baseline, candidate RunReport) Comparison {
	result := Comparison{}
	if baseline.Schema != candidate.Schema || baseline.Schema != ReportSchemaV1 {
		result.Warnings = append(result.Warnings, "report schema incompatible")
		return result
	}
	if baseline.Suite != candidate.Suite {
		result.Warnings = append(result.Warnings, "suite incompatible")
		return result
	}
	result.Compatible = true
	base := indexQuality(baseline.Quality)
	cand := indexQuality(candidate.Quality)
	for key, left := range base {
		right, ok := cand[key]
		if !ok {
			result.Regressions = append(result.Regressions, Regression{key.caseID, key.profile, key.budget, []string{"candidate result missing"}})
			continue
		}
		var worse, better []string
		compareHigher("required_path_recall", left.RequiredPathRecall, right.RequiredPathRecall, &worse, &better)
		compareHigher("required_symbol_recall", left.RequiredSymbolRecall, right.RequiredSymbolRecall, &worse, &better)
		compareHigher("budget_compliant", float64(left.BudgetCompliant), float64(right.BudgetCompliant), &worse, &better)
		compareHigher("deterministic", float64(left.Deterministic), float64(right.Deterministic), &worse, &better)
		compareLower("forbidden_violations", float64(left.ForbiddenViolations), float64(right.ForbiddenViolations), &worse, &better)
		compareLower("known_resend_count", float64(left.KnownResendCount), float64(right.KnownResendCount), &worse, &better)
		if len(worse) > 0 {
			result.Regressions = append(result.Regressions, Regression{key.caseID, key.profile, key.budget, worse})
		}
		if len(better) > 0 {
			result.Improvements = append(result.Improvements, Regression{key.caseID, key.profile, key.budget, better})
		}
	}
	return result
}
func indexQuality(values []QualityResult) map[qualityKey]QualityResult {
	result := map[qualityKey]QualityResult{}
	for _, value := range values {
		result[qualityKey{value.CaseID, value.Profile, value.Budget}] = value
	}
	return result
}
func compareHigher(name string, left, right float64, worse, better *[]string) {
	if right < left {
		*worse = append(*worse, fmt.Sprintf("%s %.4f -> %.4f", name, left, right))
	} else if right > left {
		*better = append(*better, fmt.Sprintf("%s %.4f -> %.4f", name, left, right))
	}
}
func compareLower(name string, left, right float64, worse, better *[]string) {
	if right > left {
		*worse = append(*worse, fmt.Sprintf("%s %.4f -> %.4f", name, left, right))
	} else if right < left {
		*better = append(*better, fmt.Sprintf("%s %.4f -> %.4f", name, left, right))
	}
}
