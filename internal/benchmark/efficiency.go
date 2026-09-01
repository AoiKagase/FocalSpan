package benchmark

import (
	"math"
	"strconv"
)

// EvidenceEfficiency summarizes useful packed labels per estimated wire tokens.
// It is development-only and is not serialized in normal quality output.
type EvidenceEfficiency struct {
	UsefulEvidence  int     `json:"useful_evidence"`
	EstimatedTokens int     `json:"estimated_tokens"`
	Per1000Tokens   float64 `json:"per_1000_tokens"`
}

func MeasureUsefulEvidence(benchmarkCase Case, result QualityResult) int {
	seen := make(map[string]struct{})
	useful := proportionalCount(uniquePathLabels(benchmarkCase.RequiredPaths, seen), result.RequiredPathRecall)
	useful += proportionalCount(uniqueSymbolLabels(benchmarkCase.RequiredSymbols, seen), result.RequiredSymbolRecall)
	for index, expand := range benchmarkCase.Expand {
		// Include the expansion index in the key so repeated labels from
		// separate relation expectations remain distinct evidence obligations.
		prefix := "expand:" + strconv.Itoa(index) + ":"
		useful += proportionalCount(uniquePathLabelsWithPrefix(expand.RequiredPaths, seen, prefix), result.ExpandRequiredPathRecall)
		useful += proportionalCount(uniqueSymbolLabelsWithPrefix(expand.RequiredSymbols, seen, prefix), result.ExpandRequiredSymbolRecall)
	}
	return useful
}

func uniquePathLabels(values []string, seen map[string]struct{}) int {
	return uniquePathLabelsWithPrefix(values, seen, "initial:")
}

func uniquePathLabelsWithPrefix(values []string, seen map[string]struct{}, prefix string) int {
	count := 0
	for _, value := range values {
		key := prefix + "path:" + value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		count++
	}
	return count
}

func uniqueSymbolLabels(values []SymbolExpectation, seen map[string]struct{}) int {
	return uniqueSymbolLabelsWithPrefix(values, seen, "initial:")
}

func uniqueSymbolLabelsWithPrefix(values []SymbolExpectation, seen map[string]struct{}, prefix string) int {
	count := 0
	for _, value := range values {
		key := prefix + "symbol:" + value.Path + "\x00" + value.Name + "\x00" + value.Kind
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		count++
	}
	return count
}

func AggregateEvidenceEfficiency(cases []Case, results []QualityResult) EvidenceEfficiency {
	byID := make(map[string]Case, len(cases))
	for _, benchmarkCase := range cases {
		byID[benchmarkCase.ID] = benchmarkCase
	}
	summary := EvidenceEfficiency{}
	for _, result := range results {
		benchmarkCase, ok := byID[result.CaseID]
		if !ok {
			continue
		}
		summary.UsefulEvidence += MeasureUsefulEvidence(benchmarkCase, result)
		if result.CumulativeWireTokens > 0 {
			summary.EstimatedTokens += result.CumulativeWireTokens
		} else {
			summary.EstimatedTokens += result.WireTokens
		}
	}
	if summary.EstimatedTokens > 0 {
		summary.Per1000Tokens = float64(summary.UsefulEvidence) * 1000 / float64(summary.EstimatedTokens)
	}
	return summary
}

func proportionalCount(total int, recall float64) int {
	if total <= 0 || recall <= 0 {
		return 0
	}
	if recall >= 1 {
		return total
	}
	return int(math.Round(float64(total) * recall))
}
