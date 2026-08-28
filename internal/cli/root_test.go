package cli

import (
	"context"
	"path/filepath"
	"testing"
)

func TestExplicitRootDoesNotExpandToParentGitRepository(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "repos", "authsample")
	want, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := resolveRoot(context.Background(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("explicit root=%q, want %q", got, filepath.Clean(want))
	}
}
