package goast

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestLineDirectiveKeepsPhysicalLocations(t *testing.T) {
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.go", Language: "go", Content: []byte("package p\n//line virtual.go:900\nfunc Redirected() {}\n")})
	if err != nil {
		t.Fatal(err)
	}
	s := findSymbol(got.Symbols, "Redirected")
	if s == nil || s.StartLine != 3 || s.EndLine != 3 {
		t.Fatalf("physical line 3 expected: %+v", s)
	}
}
