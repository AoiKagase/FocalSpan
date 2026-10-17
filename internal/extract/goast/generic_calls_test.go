package goast

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestAuditGoGenericCall(t *testing.T) {
	e, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "p.go", Language: "go", Content: []byte("package p\nfunc Target[T any]() {}\nfunc Caller(){ Target[int]() }\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range e.Relations {
		if r.Kind == "calls" {
			return
		}
	}
	t.Fatal("generic function invocation missing from calls")
}
