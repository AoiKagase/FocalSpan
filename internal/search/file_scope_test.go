package search

import (
	"reflect"
	"testing"
)

func TestIdentifierStyleVariantsPreserveStableCodeNames(t *testing.T) {
	got := identifierStyleVariants("code_context")
	want := []string{"code_context", "context", "codeContext", "CodeContext"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("variants=%v, want %v", got, want)
	}
}

func TestFuseFileScopesUsesRRFAndStableTieBreaks(t *testing.T) {
	got := fuseFileScopes([]fileScopeList{
		{signal: "symbol", paths: []string{"b.go", "a.go"}},
		{signal: "fts", paths: []string{"a.go", "c.go"}},
	}, 8)
	want := []string{"a.go", "b.go", "c.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scope=%v, want %v", got, want)
	}
}

func TestFilterFileScopesHonorsExplicitPathFilters(t *testing.T) {
	got := filterFileScopes([]string{"src/allowed.go", "src/other.go", "docs/readme.md", "src/ALLOWED.go"}, []string{"src/allowed.go"})
	want := []string{"src/allowed.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered scope=%v, want %v", got, want)
	}
}
