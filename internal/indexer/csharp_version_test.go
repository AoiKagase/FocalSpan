package indexer

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSharpFunctionalVersionRefreshesUnchangedSource(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := "class Service { public required string Name { get; init; } public T Echo<T>(T value) { return value; } }\n"
	write(t, filepath.Join(root, "Service.cs"), source)
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old := New(root, cfg, s, extract.NewRegistry(generic.NewExtractor()))
	if _, err := old.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(ctx, "extractor_version", "extractors-v20-cpp-functional"); err != nil {
		t.Fatal(err)
	}
	current := New(root, cfg, s, extract.NewRegistry(csharp.NewExtractor()))
	result, err := current.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesChanged != 1 || result.FilesUnchanged != 0 {
		t.Fatalf("unchanged source was not refreshed: %+v", result)
	}
	symbols, err := s.Symbols(ctx)
	if err != nil {
		t.Fatal(err)
	}
	echo, property := false, false
	for _, symbol := range symbols {
		if symbol.Name == "Echo" && symbol.Kind == "method" {
			echo = true
		}
		if symbol.Name == "Name" && symbol.Kind == "property" {
			property = strings.HasPrefix(source[symbol.StartByte:symbol.EndByte], "public required string Name")
		}
	}
	if !echo || !property {
		t.Fatalf("refreshed Echo=%v full property=%v", echo, property)
	}
	next, err := current.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if next.FilesChanged != 0 || next.FilesUnchanged != 1 {
		t.Fatalf("repeat refresh=%+v", next)
	}
}
