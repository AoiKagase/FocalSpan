package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/model"
)

func TestResidual11RefreshesPreviousExtractorIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, e := os.ReadFile("../../testdata/regressions/residual11/jsts-shadow.ts")
	if e != nil {
		t.Fatal(e)
	}
	writeAppFile(t, filepath.Join(root, "a.ts"), string(source))
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Index(ctx, true); e != nil {
		t.Fatal(e)
	}
	hash, _, e := s.Store.FileHash(ctx, "a.ts")
	if e != nil {
		t.Fatal(e)
	}
	file := model.SourceFile{Path: "a.ts", Language: "typescript", Content: source, SHA256: hash, SizeBytes: int64(len(source))}
	x, e := jsts.NewExtractor().Extract(ctx, file)
	if e != nil {
		t.Fatal(e)
	}
	caller, target := "", ""
	for _, symbol := range x.Symbols {
		if symbol.Name == "caller" {
			caller = symbol.Handle
		}
		if symbol.Name == "target" {
			target = symbol.Handle
		}
	}
	if caller == "" || target == "" {
		t.Fatal("fixture symbols missing")
	}
	x.Relations = append(x.Relations, model.Relation{FromHandle: caller, ToHandle: target, Kind: "calls", Source: "jsts-call", Confidence: .95})
	if e = s.Store.ReplaceFile(ctx, file, x); e != nil {
		t.Fatal(e)
	}
	if e = s.Store.SetMeta(ctx, "extractor_version", "extractors-v18-audit50-extraction-linking"); e != nil {
		t.Fatal(e)
	}
	status, e := s.Status(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !status.Stale {
		t.Error("previous extractor version accepted as fresh")
	}
	if _, e = s.Index(ctx, false); e != nil {
		t.Fatal(e)
	}
	rels, e := s.Store.Relations(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rels {
		if r.FromHandle == caller && r.ToHandle == target && r.Kind == "calls" {
			t.Error("previous false relation survived regular incremental update")
		}
	}
	run, e := s.Index(ctx, false)
	if e != nil {
		t.Fatal(e)
	}
	if run.FilesUnchanged != 1 || run.FilesChanged != 0 {
		t.Errorf("version refresh repeated: %+v", run)
	}
}
