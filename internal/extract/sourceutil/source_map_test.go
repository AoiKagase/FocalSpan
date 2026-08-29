package sourceutil

import "testing"

func TestSourceMapUsesOriginalBytesAndCRLFLines(t *testing.T) {
	content := []byte("一行\r\nsecond\n三行")
	m := NewSourceMap(content)
	if got := m.LineCount(); got != 3 {
		t.Fatalf("line count=%d", got)
	}
	start := len([]byte("一行\r\n"))
	span, ok := m.Span(start, len(content))
	if !ok || span.StartLine != 2 || span.EndLine != 3 {
		t.Fatalf("span=%+v ok=%v", span, ok)
	}
	text, ok := m.Slice(span)
	if !ok || text != "second\n三行" {
		t.Fatalf("slice=%q ok=%v", text, ok)
	}
}

func TestIntervalsMergeAndSubtract(t *testing.T) {
	merged := MergeIntervals([]Interval{{Start: 8, End: 12}, {Start: 1, End: 5}, {Start: 4, End: 9}})
	want := []Interval{{Start: 1, End: 12}}
	if len(merged) != 1 || merged[0] != want[0] {
		t.Fatalf("merged=%+v", merged)
	}
	remaining := SubtractIntervals([]Interval{{Start: 0, End: 20}}, []Interval{{Start: 4, End: 7}, {Start: 12, End: 15}})
	got := []Interval{{Start: 0, End: 4}, {Start: 7, End: 12}, {Start: 15, End: 20}}
	if len(remaining) != len(got) {
		t.Fatalf("remaining=%+v", remaining)
	}
	for i := range got {
		if remaining[i] != got[i] {
			t.Fatalf("remaining=%+v", remaining)
		}
	}
}

func TestBoundedLineWindows(t *testing.T) {
	got := BoundedLineWindows(170, 80, 10)
	if len(got) != 3 || got[0] != [2]int{1, 80} || got[1] != [2]int{71, 150} || got[2] != [2]int{141, 170} {
		t.Fatalf("windows=%v", got)
	}
}
