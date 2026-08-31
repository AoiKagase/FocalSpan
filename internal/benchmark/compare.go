package benchmark

import (
	"fmt"
	"sort"
)

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
	base, baseKeys, err := indexQuality(baseline.Quality)
	if err != nil {
		result.Warnings = append(result.Warnings, "baseline result matrix invalid: "+err.Error())
		return result
	}
	cand, candidateKeys, err := indexQuality(candidate.Quality)
	if err != nil {
		result.Warnings = append(result.Warnings, "candidate result matrix invalid: "+err.Error())
		return result
	}
	if !sameQualityKeys(baseKeys, candidateKeys) {
		result.Warnings = append(result.Warnings, "quality result matrix incompatible")
		return result
	}
	result.Compatible = true
	for _, key := range baseKeys {
		left := base[key]
		right := cand[key]
		var worse, better []string
		compareHigher("required_path_recall", left.RequiredPathRecall, right.RequiredPathRecall, &worse, &better)
		compareHigher("required_symbol_recall", left.RequiredSymbolRecall, right.RequiredSymbolRecall, &worse, &better)
		compareHigher("intent_correct", float64(left.IntentCorrect), float64(right.IntentCorrect), &worse, &better)
		compareHigher("role_accuracy", left.RoleAccuracy, right.RoleAccuracy, &worse, &better)
		compareHigher("relation_valid", float64(left.RelationValid), float64(right.RelationValid), &worse, &better)
		compareHigher("budget_compliant", float64(left.BudgetCompliant), float64(right.BudgetCompliant), &worse, &better)
		compareHigher("deterministic", float64(left.Deterministic), float64(right.Deterministic), &worse, &better)
		compareLower("forbidden_violations", float64(left.ForbiddenViolations), float64(right.ForbiddenViolations), &worse, &better)
		compareLower("known_resend_count", float64(left.KnownResendCount), float64(right.KnownResendCount), &worse, &better)
		if left.WireTokens > 0 && right.WireTokens*100 > left.WireTokens*110 && right.RequiredPathRecall <= left.RequiredPathRecall && right.RequiredSymbolRecall <= left.RequiredSymbolRecall {
			worse = append(worse, fmt.Sprintf("wire_tokens %d -> %d (>10%% without required-recall improvement)", left.WireTokens, right.WireTokens))
		} else if right.WireTokens < left.WireTokens {
			better = append(better, fmt.Sprintf("wire_tokens %d -> %d", left.WireTokens, right.WireTokens))
		}
		if len(worse) > 0 {
			result.Regressions = append(result.Regressions, Regression{key.caseID, key.profile, key.budget, worse})
		}
		if len(better) > 0 {
			result.Improvements = append(result.Improvements, Regression{key.caseID, key.profile, key.budget, better})
		}
	}
	appendPerformanceWarnings(&result, baseKeys, baseline.Performance, candidate.Performance)
	return result
}
func indexQuality(values []QualityResult) (map[qualityKey]QualityResult, []qualityKey, error) {
	result := map[qualityKey]QualityResult{}
	keys := make([]qualityKey, 0, len(values))
	for _, value := range values {
		key := qualityKey{value.CaseID, value.Profile, value.Budget}
		if _, exists := result[key]; exists {
			return nil, nil, fmt.Errorf("duplicate result %s/%s/%d", key.caseID, key.profile, key.budget)
		}
		result[key] = value
		keys = append(keys, key)
	}
	sortQualityKeys(keys)
	return result, keys, nil
}

func sortQualityKeys(keys []qualityKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].caseID != keys[j].caseID {
			return keys[i].caseID < keys[j].caseID
		}
		if keys[i].profile != keys[j].profile {
			return keys[i].profile < keys[j].profile
		}
		return keys[i].budget < keys[j].budget
	})
}

func sameQualityKeys(left, right []qualityKey) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func appendPerformanceWarnings(result *Comparison, keys []qualityKey, baseline, candidate []PerformanceResult) {
	base := indexPerformance(baseline)
	cand := indexPerformance(candidate)
	for _, key := range keys {
		left, leftExists := base[key]
		right, rightExists := cand[key]
		if !leftExists || !rightExists {
			continue
		}
		if left.IndexMS > 0 && right.IndexMS*100 > left.IndexMS*120 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s/%s/%d index_ms %d -> %d (>20%%)", key.caseID, key.profile, key.budget, left.IndexMS, right.IndexMS))
		}
		leftQuery, rightQuery := medianInt64(left.QueryMS), medianInt64(right.QueryMS)
		if leftQuery > 0 && rightQuery*100 > leftQuery*120 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s/%s/%d query_median_ms %d -> %d (>20%%)", key.caseID, key.profile, key.budget, leftQuery, rightQuery))
		}
	}
}

func indexPerformance(values []PerformanceResult) map[qualityKey]PerformanceResult {
	result := make(map[qualityKey]PerformanceResult, len(values))
	for _, value := range values {
		key := qualityKey{value.CaseID, value.Profile, value.Budget}
		if _, exists := result[key]; !exists {
			result[key] = value
		}
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
