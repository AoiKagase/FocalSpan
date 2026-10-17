package goast

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestAuditGoShadowedFunctionCall(t *testing.T) {
	e, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "p.go", Language: "go", Content: []byte("package p\nfunc Target() {}\nfunc Caller(){ Target := func(){}; Target() }\n")})
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, s := range e.Symbols {
		if s.Name == "Target" {
			target = s.Handle
		}
	}
	for _, r := range e.Relations {
		if r.Kind == "calls" && r.ToHandle == target {
			t.Fatalf("local variable call falsely resolved to global function: %+v", r)
		}
	}
}
