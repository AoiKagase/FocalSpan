package jsts

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestR01ShadowedCalls(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"parameter", "function caller(target: () => void) { target(); }"},
		{"default", "function caller(target = () => {}) { target(); }"},
		{"destructure", "function caller({target}) { target(); }"},
		{"local", "function caller() { const target = getCallback(); target(); }"},
		{"var", "function caller() { if (true) { var target = getCallback(); } target(); }"},
		{"catch", "function caller() { try {} catch (target) { target(); } }"},
		{"receiver", "function caller(obj) { obj.target(); }"},
		{"nested", "function caller(target) { function inner() { target(); } inner(); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte("function target() {}\n" + tc.body + "\n")})
			if e != nil {
				t.Fatal(e)
			}
			names := map[string]string{}
			for _, s := range x.Symbols {
				names[s.Handle] = s.Name
			}
			for _, r := range x.Relations {
				if names[r.FromHandle] != "target" && r.Kind != "contains" && (names[r.ToHandle] == "target" || r.UnresolvedTo == "target") {
					t.Errorf("dynamic binding leaked global relation: %+v", r)
				}
			}
		})
	}
}

func TestR01LexicalControls(t *testing.T) {
	for _, source := range []string{
		"function target() {} function caller() { target(); }",
		"function target() {} function caller() { { let target = external; } target(); }",
		"function target() {} function caller() { function target() {} target(); }",
		"function caller() { const target = () => {}; target(); }",
	} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte(source)})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		names := map[string]string{}
		for _, s := range x.Symbols {
			names[s.Handle] = s.Name
		}
		for _, r := range x.Relations {
			if names[r.FromHandle] == "caller" && r.Kind == "calls" && names[r.ToHandle] == "target" {
				found = true
			}
		}
		if !found {
			t.Errorf("known target not resolved: %s; %+v", source, x.Relations)
		}
	}
}
