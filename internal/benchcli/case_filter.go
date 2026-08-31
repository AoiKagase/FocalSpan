package benchcli

import (
	"fmt"

	"github.com/focalspan/focalspan/internal/benchmark"
)

func selectSuiteCases(suite benchmark.Suite, requested []string) (benchmark.Suite, error) {
	if len(requested) == 0 {
		return suite, nil
	}
	available := make(map[string]benchmark.Case, len(suite.Cases))
	for _, benchmarkCase := range suite.Cases {
		available[benchmarkCase.ID] = benchmarkCase
	}
	selected := make([]benchmark.Case, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, id := range requested {
		if _, duplicate := seen[id]; duplicate {
			return benchmark.Suite{}, fmt.Errorf("duplicate --case %q", id)
		}
		benchmarkCase, exists := available[id]
		if !exists {
			return benchmark.Suite{}, fmt.Errorf("unknown benchmark case %q", id)
		}
		seen[id] = struct{}{}
		selected = append(selected, benchmarkCase)
	}
	suite.Cases = selected
	return suite, nil
}

func selectReportCases(report benchmark.RunReport, requested []string) (benchmark.RunReport, error) {
	if len(requested) == 0 {
		return report, nil
	}
	requestedSet := make(map[string]struct{}, len(requested))
	available := make(map[string]struct{})
	for _, quality := range report.Quality {
		available[quality.CaseID] = struct{}{}
	}
	for _, id := range requested {
		if _, duplicate := requestedSet[id]; duplicate {
			return benchmark.RunReport{}, fmt.Errorf("duplicate --case %q", id)
		}
		if _, exists := available[id]; !exists {
			return benchmark.RunReport{}, fmt.Errorf("unknown benchmark case %q", id)
		}
		requestedSet[id] = struct{}{}
	}
	quality := make([]benchmark.QualityResult, 0, len(report.Quality))
	for _, result := range report.Quality {
		if _, selected := requestedSet[result.CaseID]; selected {
			quality = append(quality, result)
		}
	}
	performance := make([]benchmark.PerformanceResult, 0, len(report.Performance))
	for _, result := range report.Performance {
		if _, selected := requestedSet[result.CaseID]; selected {
			performance = append(performance, result)
		}
	}
	report.Quality = quality
	report.Performance = performance
	report.Aggregate = benchmark.AggregateResults(quality)
	return report, nil
}
