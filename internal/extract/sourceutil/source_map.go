package sourceutil

import "sort"

// Span is a half-open byte range in the original source and its corresponding
// one-based line range.
type Span struct {
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

// SourceMap keeps the original bytes and an index of line starts. It makes
// span conversion independent of the source language and O(log n) per lookup.
type SourceMap struct {
	Content    []byte
	LineStarts []int
}

func NewSourceMap(content []byte) SourceMap {
	starts := []int{0}
	for index, value := range content {
		if value == '\n' && index+1 <= len(content) {
			starts = append(starts, index+1)
		}
	}
	return SourceMap{Content: content, LineStarts: starts}
}

func (m SourceMap) LineCount() int {
	if len(m.LineStarts) == 0 {
		return 1
	}
	return len(m.LineStarts)
}

func (m SourceMap) LineAt(offset int) int {
	if offset <= 0 || len(m.LineStarts) == 0 {
		return 1
	}
	if offset >= len(m.Content) {
		return m.LineCount()
	}
	return sort.Search(len(m.LineStarts), func(index int) bool { return m.LineStarts[index] > offset })
}

func (m SourceMap) Span(start, end int) (Span, bool) {
	if start < 0 || end < start || end > len(m.Content) {
		return Span{}, false
	}
	lineEnd := start
	if end > start {
		lineEnd = end - 1
	}
	return Span{StartByte: start, EndByte: end, StartLine: m.LineAt(start), EndLine: m.LineAt(lineEnd)}, true
}

func (m SourceMap) Slice(span Span) (string, bool) {
	if span.StartByte < 0 || span.EndByte < span.StartByte || span.EndByte > len(m.Content) {
		return "", false
	}
	return string(m.Content[span.StartByte:span.EndByte]), true
}

// WindowByLines returns deterministic windows, each at most lines long, that
// cover span's source lines. Overlap applies between adjacent windows.
func WindowByLines(source SourceMap, span Span, lines, overlap int) []Span {
	if source.LineCount() == 0 || span.StartLine < 1 || span.EndLine < span.StartLine {
		return nil
	}
	if lines <= 0 {
		lines = 80
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= lines {
		overlap = lines - 1
	}
	startLine := span.StartLine
	endLine := span.EndLine
	if startLine > source.LineCount() {
		return nil
	}
	if endLine > source.LineCount() {
		endLine = source.LineCount()
	}
	step := lines - overlap
	result := make([]Span, 0, (endLine-startLine+step)/step)
	for windowStart := startLine; windowStart <= endLine; windowStart += step {
		windowEnd := windowStart + lines - 1
		if windowEnd > endLine {
			windowEnd = endLine
		}
		startByte := source.LineStarts[windowStart-1]
		endByte := len(source.Content)
		if windowEnd < source.LineCount() {
			endByte = source.LineStarts[windowEnd]
		}
		window, ok := source.Span(startByte, endByte)
		if !ok {
			return nil
		}
		result = append(result, window)
		if windowEnd == endLine {
			break
		}
	}
	return result
}

// ValidUTF8Boundary reports whether offset is at a UTF-8 code point boundary.
func ValidUTF8Boundary(content []byte, offset int) bool {
	if offset < 0 || offset > len(content) {
		return false
	}
	return offset == 0 || offset == len(content) || content[offset]&0xc0 != 0x80
}
