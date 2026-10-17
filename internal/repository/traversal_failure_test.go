package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestScannerRejectsMissingRoot(t *testing.T) {
	_, _, err := NewScanner(filepath.Join(t.TempDir(), "missing"), testConfig()).Scan(context.Background())
	if err == nil {
		t.Fatal("missing root accepted as an empty successful scan")
	}
}

func TestScannerCancelsEmptyWalk(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := NewScanner(t.TempDir(), testConfig()).Scan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}
