package benchmark

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarshalQualityExcludesPerformanceAndIsDeterministic(t *testing.T) {
	report := RunReport{Schema: ReportSchemaV1, Suite: "suite", FocalSpanCommit: "abc", Quality: []QualityResult{{CaseID: "c", Profile: "p", Budget: 100}}, Performance: []PerformanceResult{{CaseID: "c", SnapshotMS: 99}}}
	first, err := MarshalQuality(report)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := MarshalQuality(report)
	if !bytes.Equal(first, second) || bytes.Contains(first, []byte("snapshot_ms")) {
		t.Fatalf("quality = %s", first)
	}
}

func TestRenderMarkdownRejectsSourceAndAbsolutePaths(t *testing.T) {
	text, err := RenderMarkdown(RunReport{Schema: ReportSchemaV1, Suite: "suite", Quality: []QualityResult{{CaseID: "case", Profile: "p", Budget: 100, RequiredPathRecall: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "C:\\") || strings.Contains(text, "source") {
		t.Fatalf("markdown = %s", text)
	}
}
