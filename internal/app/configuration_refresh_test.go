package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"path/filepath"
	"testing"
)

func TestConfigurationReextractsSameBytes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "logic.txt"), "package p\nfunc ValidateToken() bool { return true }\n")
	cfg := config.Default()
	s, err := NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	s.Close()
	cfg.LanguageOverrides = map[string]string{"*.txt": "go"}
	s, err = NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(ctx, false); err != nil {
		t.Fatal(err)
	}
	cs, err := s.Store.CandidatesForPath(ctx, "logic.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.Language == "go" && c.Symbol == "ValidateToken" {
			return
		}
	}
	t.Fatalf("override not applied: %+v", cs)
}
