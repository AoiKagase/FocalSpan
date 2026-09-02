package evidence

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/query"
)

type indexedLine struct {
	start int
	end   int
	text  string
}

type sourceWindow struct {
	start int
	end   int
	score int
}

func focusedSegments(candidate ClassifiedCandidate, plan query.Plan) []Segment {
	return focusedSegmentsWithMargins(candidate, plan, 2, 4, 0, 3)
}

func adaptiveFocusedSegments(candidate ClassifiedCandidate, plan query.Plan) []Segment {
	return focusedSegmentsWithMargins(candidate, plan, 0, 1, 2, 4)
}

func focusedSegmentsWithMargins(candidate ClassifiedCandidate, plan query.Plan, contextBefore, contextAfter, prefixLineLimit, maxWindows int) []Segment {
	content := candidate.Candidate.Content
	if content == "" || !utf8.ValidString(content) {
		return nil
	}
	lines := indexLines(content)
	if len(lines) == 0 {
		return nil
	}
	terms := focusedTerms(plan)
	hits := make([]sourceWindow, 0)
	for index, line := range lines {
		score := distinctMatches(line.text, terms)
		if score == 0 {
			continue
		}
		start := index - contextBefore
		if start < 0 {
			start = 0
		}
		end := index + contextAfter
		if end >= len(lines) {
			end = len(lines) - 1
		}
		hits = append(hits, sourceWindow{start: start, end: end, score: score})
	}
	hits = mergeWindows(hits)
	prefixEnd := declarationPrefixEnd(lines)
	if prefixLineLimit > 0 && prefixEnd >= prefixLineLimit {
		prefixEnd = prefixLineLimit - 1
	}
	prefix := sourceWindow{start: 0, end: prefixEnd, score: distinctMatches(joinLines(lines, 0, prefixEnd), terms)}
	selected := []sourceWindow{prefix}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].start < hits[j].start
	})
	for _, hit := range hits {
		selected = addOrMergeWindow(selected, hit)
		if len(selected) == maxWindows {
			break
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].start < selected[j].start })
	selected = mergeWindows(selected)

	segments := make([]Segment, 0, len(selected)*2+1)
	cursor := 0
	for _, window := range selected {
		if window.start > cursor {
			segments = append(segments, Segment{Kind: SegmentOmitted, Lines: absoluteLines(candidate, cursor, window.start-1)})
		}
		segments = append(segments, Segment{Kind: SegmentSource, Lines: absoluteLines(candidate, window.start, window.end), Text: joinLines(lines, window.start, window.end)})
		cursor = window.end + 1
	}
	if cursor < len(lines) {
		segments = append(segments, Segment{Kind: SegmentOmitted, Lines: absoluteLines(candidate, cursor, len(lines)-1)})
	}
	return segments
}

func indexLines(content string) []indexedLine {
	if content == "" {
		return nil
	}
	result := make([]indexedLine, 0, strings.Count(content, "\n")+1)
	start := 0
	for index := 0; index < len(content); index++ {
		if content[index] != '\n' {
			continue
		}
		end := index + 1
		result = append(result, indexedLine{start: start, end: end, text: content[start:end]})
		start = end
	}
	if start < len(content) {
		result = append(result, indexedLine{start: start, end: len(content), text: content[start:]})
	}
	return result
}

func focusedTerms(plan query.Plan) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	for _, value := range plan.Terms.Identifiers {
		add(value)
	}
	for _, value := range plan.Terms.Symbols {
		add(value)
	}
	for _, value := range plan.Terms.Phrases {
		add(value)
	}
	for _, value := range plan.Terms.UnicodeRuns {
		if utf8.RuneCountInString(value) >= 2 {
			add(value)
		}
	}
	for _, value := range plan.Terms.Words {
		if isUsefulFocusedTerm(value) {
			add(value)
		}
	}
	for _, anchor := range plan.Anchors {
		add(terminalName(anchor))
	}
	return result
}

func isUsefulFocusedTerm(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r > 127 {
			return utf8.RuneCountInString(value) >= 2
		}
	}
	if len(value) < 3 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func terminalName(value string) string {
	value = strings.TrimSpace(value)
	for _, separator := range []string{"->", "::", ".", `\`} {
		if index := strings.LastIndex(value, separator); index >= 0 {
			value = value[index+len(separator):]
		}
	}
	return value
}

func distinctMatches(line string, terms []string) int {
	count := 0
	lower := strings.ToLower(line)
	for _, term := range terms {
		if strings.Contains(line, term) || strings.Contains(lower, strings.ToLower(term)) {
			count++
		}
	}
	return count
}

func declarationPrefixEnd(lines []indexedLine) int {
	limit := len(lines) - 1
	if limit > 5 {
		limit = 5
	}
	for index := 0; index <= limit; index++ {
		if strings.ContainsAny(lines[index].text, "{:") {
			return index
		}
	}
	return limit
}

func mergeWindows(windows []sourceWindow) []sourceWindow {
	if len(windows) == 0 {
		return nil
	}
	ordered := append([]sourceWindow(nil), windows...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].start < ordered[j].start })
	result := []sourceWindow{ordered[0]}
	for _, window := range ordered[1:] {
		last := &result[len(result)-1]
		if window.start-last.end <= 3 {
			if window.end > last.end {
				last.end = window.end
			}
			if window.score > last.score {
				last.score = window.score
			}
			continue
		}
		result = append(result, window)
	}
	return result
}

func addOrMergeWindow(selected []sourceWindow, candidate sourceWindow) []sourceWindow {
	for index := range selected {
		if candidate.start-selected[index].end <= 3 && selected[index].start-candidate.end <= 3 {
			if candidate.start < selected[index].start {
				selected[index].start = candidate.start
			}
			if candidate.end > selected[index].end {
				selected[index].end = candidate.end
			}
			if candidate.score > selected[index].score {
				selected[index].score = candidate.score
			}
			return selected
		}
	}
	return append(selected, candidate)
}

func joinLines(lines []indexedLine, start, end int) string {
	if start < 0 || end < start || start >= len(lines) {
		return ""
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	var result strings.Builder
	for index := start; index <= end; index++ {
		result.WriteString(lines[index].text)
	}
	return result.String()
}

func absoluteLines(candidate ClassifiedCandidate, start, end int) [2]int {
	return [2]int{candidate.Candidate.StartLine + start, candidate.Candidate.StartLine + end}
}
