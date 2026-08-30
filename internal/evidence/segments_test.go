package evidence

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func TestFocusedSegmentsPreserveLateHitAndOriginalBytes(t *testing.T) {
	content := strings.Join([]string{
		"func ValidateToken(token Token) error {",
		"\tnormalized := strings.TrimSpace(token.Raw)",
		"\t_ = normalized",
		strings.Repeat("\tlogValidationStep()\n", 80),
		"\t// 日本語の期限コメント",
		"\tif token.ExpiresAt.Before(now) {",
		"\t\treturn ErrExpiredToken",
		"\t}",
		"\treturn nil",
		"}",
	}, "\n")
	candidate := ClassifiedCandidate{
		Candidate: model.RankedCandidate{Handle: "target", Path: "auth/service.go", Language: "go", Kind: "method", Symbol: "Service.ValidateToken", Signature: "func (s *Service) ValidateToken(token Token) error", StartLine: 30, EndLine: 120, Content: content},
		Role:      RoleTarget,
	}
	plan := query.Plan{RawQuery: "where is an expired authentication token rejected?", Terms: query.Terms{Words: []string{"expired", "authentication", "token", "rejected"}, Identifiers: []string{"ErrExpiredToken"}}, Anchors: []string{"ValidateToken"}}
	segments := focusedSegments(candidate, plan)
	if len(segments) == 0 {
		t.Fatal("focusedSegments returned no segments")
	}
	foundLate := false
	for _, segment := range segments {
		if segment.Kind != SegmentSource {
			if segment.Text != "" {
				t.Fatalf("omitted segment contains text: %+v", segment)
			}
			continue
		}
		want := testSourceLines(content, segment.Lines, candidate.Candidate.StartLine)
		if segment.Text != want {
			t.Fatalf("segment differs from indexed source\n got: %q\nwant: %q", segment.Text, want)
		}
		if strings.Contains(segment.Text, "ErrExpiredToken") {
			foundLate = true
		}
		if strings.Contains(segment.Text, "[...") || strings.Contains(segment.Text, "1: ") {
			t.Fatalf("generated marker or line prefix leaked into source: %q", segment.Text)
		}
	}
	if !foundLate {
		t.Fatalf("late ErrExpiredToken branch missing: %+v", segments)
	}
}

func TestFocusedSegmentsHandleCRLFUTF8AndBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   query.Plan
	}{
		{name: "single line", content: "func Run() {}", query: query.Plan{Terms: query.Terms{Identifiers: []string{"Run"}}}},
		{name: "no trailing newline", content: "func Run() {\n\treturn ErrLast\n}", query: query.Plan{Terms: query.Terms{Identifiers: []string{"ErrLast"}}}},
		{name: "crlf", content: "func Run() {\r\n\t// 日本語\r\n\treturn ErrExpired\r\n}\r\n", query: query.Plan{Terms: query.Terms{Identifiers: []string{"ErrExpired"}}}},
		{name: "utf8 identifier", content: "func 検証() {\n\treturn 期限切れ\n}\n", query: query.Plan{Terms: query.Terms{UnicodeRuns: []string{"期限切れ"}}}},
		{name: "first and last", content: "HitFirst\nnone\nnone\nHitLast", query: query.Plan{Terms: query.Terms{Identifiers: []string{"HitFirst", "HitLast"}}}},
		{name: "no lexical hit", content: "func Run() {\n\treturn nil\n}\n", query: query.Plan{Terms: query.Terms{Words: []string{"missing"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "h", Path: "a.go", Kind: "function", Symbol: "Run", StartLine: 7, EndLine: 7 + strings.Count(tt.content, "\n"), Content: tt.content}, Role: RoleTarget}
			segments := focusedSegments(candidate, tt.query)
			for _, segment := range segments {
				if segment.Kind == SegmentSource && segment.Text != testSourceLines(tt.content, segment.Lines, 7) {
					t.Fatalf("source fidelity mismatch: %+v", segment)
				}
			}
		})
	}
}

func TestFocusedSegmentsSelectAtMostThreeDistantWindows(t *testing.T) {
	content := "func Run() {\n" +
		"HitOne\n" + strings.Repeat("filler\n", 10) +
		"HitTwo\n" + strings.Repeat("filler\n", 10) +
		"Decisive Extra\n" + strings.Repeat("filler\n", 10) +
		"HitFour\n}\n"
	candidate := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "h", Path: "a.go", Kind: "function", StartLine: 1, EndLine: 38, Content: content}, Role: RoleTarget}
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"HitOne", "HitTwo", "Decisive", "Extra", "HitFour"}}}
	segments := focusedSegments(candidate, plan)
	sourceCount := 0
	foundBest := false
	for _, segment := range segments {
		if segment.Kind == SegmentSource {
			sourceCount++
			foundBest = foundBest || strings.Contains(segment.Text, "Decisive Extra")
		}
	}
	if sourceCount > 3 || !foundBest {
		t.Fatalf("source windows=%d foundBest=%v segments=%+v", sourceCount, foundBest, segments)
	}
}

func testSourceLines(content string, absolute [2]int, candidateStart int) string {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	start := absolute[0] - candidateStart
	end := absolute[1] - candidateStart
	if start < 0 || end < start || end >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:end+1], "")
}
