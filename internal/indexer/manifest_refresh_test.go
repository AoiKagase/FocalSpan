package indexer

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"path/filepath"
	"reflect"
	"testing"
)

type manifestFixtureExtractor struct{}

func (manifestFixtureExtractor) Name() string { return "manifest-fixture" }
func (manifestFixtureExtractor) Supports(path, language string) bool {
	return filepath.Ext(path) == ".go"
}
func (manifestFixtureExtractor) Extract(ctx context.Context, f model.SourceFile) (model.Extraction, error) {
	handle := "target"
	name := "ValidateToken"
	if f.Path == "cmd/main.go" {
		handle = "caller"
		name = "Main"
	}
	e := model.Extraction{Symbols: []model.Symbol{{Handle: handle, FilePath: f.Path, Language: "go", Kind: "function", Name: name, StartLine: 1, EndLine: 1, Confidence: 1}}}
	if handle == "caller" {
		e.Relations = []model.Relation{{FromHandle: handle, UnresolvedTo: "example.com/old/auth", Kind: "imports", Confidence: .4, Source: "fixture"}}
	}
	return e, nil
}
func TestManifestOnlyUpdateRebuildsResolvedRelations(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.Default()
	write(t, filepath.Join(root, "src/auth/service.go"), "target\n")
	write(t, filepath.Join(root, "cmd/main.go"), "caller\n")
	write(t, filepath.Join(root, "src/go.mod"), "module example.com/old\n")
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(manifestFixtureExtractor{}, generic.NewExtractor()))
	if _, err = ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Relations(ctx)
	found := false
	for _, r := range before {
		if r.FromHandle == "caller" && r.ToHandle == "target" {
			found = true
		}
	}
	if !found {
		t.Fatalf("initial graph unresolved: %+v", before)
	}
	write(t, filepath.Join(root, "src/go.mod"), "module example.com/new\n")
	if _, err = ix.Run(ctx, false); err != nil {
		t.Fatal(err)
	}
	incremental, _ := s.Relations(ctx)
	if _, err = ix.Run(ctx, true); err != nil {
		t.Fatal(err)
	}
	rebuilt, _ := s.Relations(ctx)
	if !reflect.DeepEqual(incremental, rebuilt) {
		t.Fatalf("incremental=%+v full=%+v", incremental, rebuilt)
	}
}
