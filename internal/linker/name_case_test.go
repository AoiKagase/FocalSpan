package linker

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestGoCallResolutionRespectsCase(t *testing.T) {
	s := openLinkerStore(t)
	seedLinkerFile(t, s, "a.go", "caller", "Caller", "func Caller", []model.Relation{{FromHandle: "caller", UnresolvedTo: "validate", Kind: "calls", Confidence: .8}})
	seedLinkerFile(t, s, "b.go", "target", "Validate", "func Validate", nil)
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	rs, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.FromHandle == "caller" && r.ToHandle == "target" {
			t.Fatalf("case mismatch resolved: %+v", r)
		}
	}
}
