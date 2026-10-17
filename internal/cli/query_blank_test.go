package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestExplicitBlankQueryCannotUsePositionalText(t *testing.T) {
	var out, errout bytes.Buffer
	code := Run(context.Background(), []string{"--query", "", "extra", "--root", t.TempDir()}, &out, &errout)
	if code == 0 {
		t.Fatal("blank explicit query silently replaced by positional text")
	}
}
