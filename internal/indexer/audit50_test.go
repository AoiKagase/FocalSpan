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

func TestAudit50ReindexesV17Metadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), "package p\nfunc Current(){}\n")
	cfg := config.Default()
	st, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ix := New(root, cfg, st, extract.NewRegistry(goast.NewExtractor()))
	if _, err := ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMeta(ctx, "extractor_version", "extractors-v17-python-go-fidelity"); err != nil {
		t.Fatal(err)
	}
	run, err := ix.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.FilesUnchanged != 0 || run.FilesChanged != 1 {
		t.Fatalf("v17 extraction not refreshed: %+v", run)
	}
	version, err := st.Meta(ctx, "extractor_version")
	if err != nil || version != extractorVersion {
		t.Fatalf("version=%q err=%v", version, err)
	}
	again, err := ix.Run(ctx, false)
	if err != nil || again.FilesUnchanged != 1 || again.FilesChanged != 0 {
		t.Fatalf("refresh must happen once: %+v %v", again, err)
	}
}
