package benchmark

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func MarshalQuality(report RunReport) ([]byte, error) {
	value := struct {
		Schema          string           `json:"schema"`
		Suite           string           `json:"suite"`
		FocalSpanCommit string           `json:"focalspan_commit"`
		Quality         []QualityResult  `json:"quality"`
		Aggregate       AggregateQuality `json:"aggregate"`
	}{report.Schema, report.Suite, report.FocalSpanCommit, report.Quality, report.Aggregate}
	return json.MarshalIndent(value, "", "  ")
}
func MarshalFullReport(report RunReport) ([]byte, error) { return json.MarshalIndent(report, "", "  ") }
func RenderMarkdown(report RunReport) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# FocalSpan benchmark: %s\n\n- FocalSpan commit: `%s`\n\n| Case | Profile | Budget | Required path recall | Required symbol recall | Wire tokens | Failure codes |\n|---|---|---:|---:|---:|---:|---|\n", escapeTable(report.Suite), escapeTable(report.FocalSpanCommit))
	for _, r := range report.Quality {
		fmt.Fprintf(&b, "| %s | %s | %d | %.4f | %.4f | %d | %s |\n", escapeTable(r.CaseID), escapeTable(r.Profile), r.Budget, r.RequiredPathRecall, r.RequiredSymbolRecall, r.WireTokens, escapeTable(strings.Join(r.FailureCodes, ", ")))
	}
	if len(report.Performance) > 0 {
		b.WriteString("\n## Performance context\n\n| Case | Profile | Budget | Snapshot ms | Index ms | Query median ms | Files | Symbols | Chunks | Relations |\n|---|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
		for _, r := range report.Performance {
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d | %d | %d |\n", escapeTable(r.CaseID), escapeTable(r.Profile), r.Budget, r.SnapshotMS, r.IndexMS, medianInt64(r.QueryMS), r.Files, r.Symbols, r.Chunks, r.Relations)
		}
	}
	if report.Efficiency != nil {
		b.WriteString("\n## Development efficiency\n\n| Useful evidence | Estimated wire tokens | Per 1,000 tokens |\n|---:|---:|---:|\n")
		fmt.Fprintf(&b, "| %d | %d | %.4f |\n", report.Efficiency.UsefulEvidence, report.Efficiency.EstimatedTokens, report.Efficiency.Per1000Tokens)
	}
	return b.String(), nil
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]int64(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 0 {
		return (ordered[middle-1] + ordered[middle]) / 2
	}
	return ordered[middle]
}
func escapeTable(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v, "|", "\\|"), "\n", " ")
}
