package indexer

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/php"
	"github.com/focalspan/focalspan/internal/store"
	"path/filepath"
	"testing"
)

func TestPHPFunctionalVersionRefreshesUnchangedSource(t *testing.T) {
	if ExtractorVersion != "extractors-v22-php-functional" {
		t.Fatalf("PHP refresh version not advanced: %s", ExtractorVersion)
	}
	ctx := context.Background()
	root := t.TempDir()
	write(t, filepath.Join(root, "source.inc"), "<?php class Settings { public const string NAME = 'x'; }")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old := New(root, cfg, s, extract.NewRegistry(generic.NewExtractor()))
	if _, err = old.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err = s.SetMeta(ctx, "extractor_version", "extractors-v21-csharp-functional"); err != nil {
		t.Fatal(err)
	}
	current := New(root, cfg, s, extract.NewRegistry(php.NewExtractor()))
	run, err := current.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.FilesChanged != 1 || run.FilesUnchanged != 0 {
		t.Fatalf("refresh=%+v", run)
	}
	symbols, err := s.Symbols(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, symbol := range symbols {
		if symbol.QualifiedName == "Settings::NAME" && symbol.Kind == "constant" {
			found = true
		}
	}
	if !found {
		t.Fatal("unchanged INC not reextracted with PHP constant fix")
	}
	run, err = current.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.FilesChanged != 0 || run.FilesUnchanged != 1 {
		t.Fatalf("repeat refresh=%+v", run)
	}
}
