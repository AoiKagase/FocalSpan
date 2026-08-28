package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImpactOutsideGitReturnsEmptyBundle(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	bundle, err := service.Impact(context.Background(), "", "", 512)
	if err != nil {
		t.Fatalf("impact outside Git: %v", err)
	}
	if len(bundle.Items) != 0 {
		t.Fatalf("expected no changed candidates, got %+v", bundle.Items)
	}
	if len(bundle.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic explaining that Git impact is unavailable")
	}
}
