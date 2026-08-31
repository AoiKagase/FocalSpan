package benchmark

import (
	"encoding/json"
	"fmt"
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
	fmt.Fprintf(&b, "# FocalSpan benchmark: %s\n\n| Case | Profile | Budget | Required path recall |\n|---|---|---:|---:|\n", escapeTable(report.Suite))
	for _, r := range report.Quality {
		fmt.Fprintf(&b, "| %s | %s | %d | %.4f |\n", escapeTable(r.CaseID), escapeTable(r.Profile), r.Budget, r.RequiredPathRecall)
	}
	return b.String(), nil
}
func escapeTable(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v, "|", "\\|"), "\n", " ")
}
