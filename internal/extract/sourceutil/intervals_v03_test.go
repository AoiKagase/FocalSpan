package sourceutil

import (
	"bytes"
	"testing"
)

func TestSpanHelpersMergeSubtractAndWindowByLines(t *testing.T) {
	merged := Merge([]Span{
		{StartByte: 8, EndByte: 12, StartLine: 2, EndLine: 2},
		{StartByte: 1, EndByte: 5, StartLine: 1, EndLine: 1},
		{StartByte: 4, EndByte: 9, StartLine: 1, EndLine: 2},
		{StartByte: 20, EndByte: 20, StartLine: 4, EndLine: 4},
	})
	wantMerged := []Span{{StartByte: 1, EndByte: 12, StartLine: 1, EndLine: 2}}
	if len(merged) != len(wantMerged) || merged[0] != wantMerged[0] {
		t.Fatalf("merged=%+v want=%+v", merged, wantMerged)
	}

	whole := Span{StartByte: 0, EndByte: 20, StartLine: 1, EndLine: 4}
	remaining := Subtract(whole, []Span{{StartByte: 4, EndByte: 7}, {StartByte: 12, EndByte: 15}})
	wantRemaining := []Span{{StartByte: 0, EndByte: 4}, {StartByte: 7, EndByte: 12}, {StartByte: 15, EndByte: 20}}
	if len(remaining) != len(wantRemaining) {
		t.Fatalf("remaining=%+v want=%+v", remaining, wantRemaining)
	}
	for index := range wantRemaining {
		if remaining[index].StartByte != wantRemaining[index].StartByte || remaining[index].EndByte != wantRemaining[index].EndByte {
			t.Fatalf("remaining=%+v want=%+v", remaining, wantRemaining)
		}
	}

	content := []byte("one\ntwo\nthree\nfour\nfive\nsix\n")
	source := NewSourceMap(content)
	span, ok := source.Span(bytes.Index(content, []byte("three")), bytes.Index(content, []byte("five"))+len("five"))
	if !ok {
		t.Fatal("failed to make source span")
	}
	windows := WindowByLines(source, span, 2, 1)
	if len(windows) != 2 || windows[0].StartLine != 3 || windows[0].EndLine != 4 || windows[1].StartLine != 4 || windows[1].EndLine != 5 {
		t.Fatalf("windows=%+v", windows)
	}
	if got, _ := source.Slice(windows[0]); got != "three\nfour\n" {
		t.Fatalf("window source=%q", got)
	}
}

func TestValidUTF8BoundaryRejectsContinuationByte(t *testing.T) {
	content := []byte("日本語")
	if ValidUTF8Boundary(content, 1) || ValidUTF8Boundary(content, 2) || !ValidUTF8Boundary(content, len(content)) {
		t.Fatalf("unexpected UTF-8 boundaries for %v", content)
	}
}
