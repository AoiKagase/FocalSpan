package eval

import (
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestF50LateHitPreservation(t *testing.T) {
	c := EvidenceCase{Mode: evidence.ModeFocused, Expected: []EvidenceExpectation{{Path: "a.go", Contains: []string{"late_branch"}}}}
	if got := focusedLateHit(c, evidence.Packet{}); got != 0 {
		t.Fatalf("missing late hit=%d", got)
	}
}

func TestF50LateHitAllExpectations(t *testing.T) {
	c := EvidenceCase{Mode: evidence.ModeFocused, Expected: []EvidenceExpectation{{Path: "a.go", Symbol: "A", Contains: []string{"late", "branch"}}, {Path: "b.go", Contains: []string{"other"}}}}
	a := evidence.Item{Symbol: "A", Source: "late branch"}
	a.Location.Path = "a.go"
	b := evidence.Item{Source: "other"}
	b.Location.Path = "b.go"
	for _, tc := range []struct {
		name  string
		items []evidence.Item
		want  int
	}{{"all", []evidence.Item{a, b}, 1}, {"partial", []evidence.Item{a}, 0}, {"empty", nil, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := focusedLateHit(c, evidence.Packet{Evidence: tc.items}); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
	wrong := a
	wrong.Location.Path = "wrong.go"
	if focusedLateHit(c, evidence.Packet{Evidence: []evidence.Item{wrong, b}}) != 0 {
		t.Fatal("wrong file accepted")
	}
	wrong = a
	wrong.Symbol = "Wrong"
	if focusedLateHit(c, evidence.Packet{Evidence: []evidence.Item{wrong, b}}) != 0 {
		t.Fatal("wrong symbol accepted")
	}
	c.Mode = evidence.ModeSource
	if focusedLateHit(c, evidence.Packet{}) != 1 {
		t.Fatal("nonfocused changed")
	}
	c.Mode = evidence.ModeFocused
	c.Expected = []EvidenceExpectation{{Path: "a.go"}}
	if focusedLateHit(c, evidence.Packet{}) != 1 {
		t.Fatal("no contains changed")
	}
}

func TestF50LateHitInExcerpt(t *testing.T) {
	c := EvidenceCase{Mode: evidence.ModeFocused, Expected: []EvidenceExpectation{{Path: "a.go", Contains: []string{"late_branch"}}}}
	item := evidence.Item{Location: evidence.Location{Path: "a.go"}, Segments: []evidence.Segment{{Text: "late_branch"}}}
	if focusedLateHit(c, evidence.Packet{Evidence: []evidence.Item{item}}) != 1 {
		t.Fatal("excerpt late hit lost")
	}
	item.Segments = nil
	if focusedLateHit(c, evidence.Packet{Evidence: []evidence.Item{item}}) != 0 {
		t.Fatal("missing excerpt accepted")
	}
}
