package benchmark

import (
	"bytes"
	"os"
	"path/filepath"
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
	text, err := RenderMarkdown(RunReport{Schema: ReportSchemaV1, Suite: "suite", FocalSpanCommit: "abc123", Quality: []QualityResult{{CaseID: "case", Profile: "p", Budget: 100, RequiredPathRecall: 1, FailureCodes: []string{"required_symbol_missing"}}}, Performance: []PerformanceResult{{CaseID: "case", Profile: "p", Budget: 100, SnapshotMS: 7, IndexMS: 11, QueryMS: []int64{2, 3, 4}}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "C:\\") || strings.Contains(text, "source") {
		t.Fatalf("markdown = %s", text)
	}
	for _, want := range []string{"abc123", "required_symbol_missing", "Snapshot ms", "Query median ms", "| 7 | 11 | 3 |"} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown missing %q:\n%s", want, text)
		}
	}
}

func TestGoldenReportsMatchCheckedInFiles(t *testing.T) {
	report := goldenReportFixture()
	jsonBytes, err := MarshalQuality(report)
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := RenderMarkdown(report)
	if err != nil {
		t.Fatal(err)
	}
	assertGoldenBytes(t, filepath.Join("..", "..", "testdata", "benchmark", "golden", "quality-report.json"), append(jsonBytes, '\n'))
	assertGoldenBytes(t, filepath.Join("..", "..", "testdata", "benchmark", "golden", "quality-report.md"), []byte(markdown))
}

func goldenReportFixture() RunReport {
	quality := QualityResult{CaseID: "case-a", Profile: "full-evidence-focused", Budget: 1024, TargetRank: 1, ReciprocalRank: 1, RequiredPathRecall: 1, OptionalPathRecall: 1, RequiredSymbolRecall: 1, IntentCorrect: 1, RoleAccuracy: 1, RelationValid: 1, BudgetCompliant: 1, Deterministic: 1, WireTokens: 320, EvidenceTokens: 200, MetadataOverheadRatio: .375, ChangedPathRecall: .5}
	return RunReport{Schema: ReportSchemaV1, Suite: "golden", FocalSpanCommit: "0000000000000000000000000000000000000000", Quality: []QualityResult{quality}, Aggregate: AggregateResults([]QualityResult{quality}), Performance: []PerformanceResult{{CaseID: "case-a", Profile: "full-evidence-focused", Budget: 1024, SnapshotMS: 5, IndexMS: 8, QueryMS: []int64{2, 3, 4}, Files: 10, Symbols: 20, Chunks: 30, Relations: 4}}}
}

func assertGoldenBytes(t *testing.T, path string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
	if bytes.Contains(got, []byte("\r\n")) {
		t.Fatalf("golden %s contains CRLF", path)
	}
}
