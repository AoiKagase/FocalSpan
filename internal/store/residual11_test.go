package store

import (
	"context"
	"errors"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestR10IdentityOrderingAndCancellation(t *testing.T) {
	ctx := context.Background()
	s, e := Open(t.TempDir(), ".focalspan")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	symbols := []model.Symbol{{Handle: "later", Name: "Target", Kind: "function", StartLine: 5, EndLine: 5}, {Handle: "first", Name: "Target", Kind: "function", StartLine: 1, EndLine: 1}, {Handle: "method", Name: "Target", Kind: "method", StartLine: 2, EndLine: 2}}
	if e = s.ReplaceFile(ctx, model.SourceFile{Path: "a.go", SHA256: "fixture"}, model.Extraction{Symbols: symbols}); e != nil {
		t.Fatal(e)
	}
	got, e := s.SymbolsForIdentity(ctx, "a.go", "Target", "function")
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != 2 || got[0].Handle != "first" || got[1].Handle != "later" {
		t.Errorf("identity query lost exact kind or order: %+v", got)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.SymbolsForIdentity(cancelled, "a.go", "Target", ""); !errors.Is(e, context.Canceled) {
		t.Errorf("cancel error=%v", e)
	}
}
