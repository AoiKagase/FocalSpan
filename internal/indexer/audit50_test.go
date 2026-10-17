package indexer

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestF07AddedTargetMatchesRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.Default()
	put := func(path, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("caller.go", "package p\nfunc Caller(){ Foo() }\n")
	put("a.go", "package p\nfunc Foo(){}\n")
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor()))
	if _, err := ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	before, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, r := range before {
		if r.Kind == "calls" && r.ToHandle != "" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatalf("positive control initial call unresolved: %+v", before)
	}
	put("b.go", "package p\nfunc Foo(){}\n")
	if _, err := ix.Run(ctx, false); err != nil {
		t.Fatal(err)
	}
	incremental, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(incremental, rebuilt) {
		t.Fatalf("addition leaves incremental graph different from rebuild:\nincremental=%+v\nrebuilt=%+v", incremental, rebuilt)
	}
}
