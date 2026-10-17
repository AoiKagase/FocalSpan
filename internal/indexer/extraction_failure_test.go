package indexer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
)

func TestFailedExtractionEvictsStaleSourceAndRetries(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "auth.go")
	write(t, path, "package auth\n\nfunc OldToken() {}\n")
	write(t, filepath.Join(root, "stable.go"), "package auth\n\nfunc Stable() {}\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(failingGoExtractor{}))
	first, err := ix.Run(ctx, true)
	if err != nil || first.ParseFailures != 0 {
		t.Fatalf("initial run=%+v err=%v", first, err)
	}
	assertSource := func(path, want string) {
		t.Helper()
		got, err := s.CandidatesForPath(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(got) != 0 {
				t.Fatalf("stale candidates for %s: %+v", path, got)
			}
			return
		}
		for _, candidate := range got {
			if strings.Contains(candidate.Content, want) {
				return
			}
		}
		t.Fatalf("missing source %q in %s: %+v", want, path, got)
	}
	assertSource("auth.go", "OldToken")
	broken := "package auth\n// fail extraction\n"
	write(t, path, broken)
	for attempt := 0; attempt < 2; attempt++ {
		run, err := ix.Run(ctx, false)
		if err != nil || run.ParseFailures != 1 || run.FilesUnchanged != 1 || run.FilesDeleted != 0 {
			t.Fatalf("attempt %d: run=%+v err=%v", attempt, run, err)
		}
		if run.Revision == first.Revision {
			t.Fatal("changed repository retained previous revision")
		}
		assertSource("auth.go", "")
		assertSource("stable.go", "Stable")
		if _, found, err := s.FileHash(ctx, "auth.go"); err != nil || found {
			t.Fatalf("failed path must remain retryable: found=%v err=%v", found, err)
		}
	}
	write(t, path, "package auth\n\nfunc NewToken() {}\n")
	repaired, err := ix.Run(ctx, false)
	if err != nil || repaired.ParseFailures != 0 || repaired.FilesAdded != 1 || repaired.FilesUnchanged != 1 {
		t.Fatalf("repair run=%+v err=%v", repaired, err)
	}
	assertSource("auth.go", "NewToken")
	if _, found, err := s.FileHash(ctx, "auth.go"); err != nil || !found {
		t.Fatalf("repaired path not indexed: found=%v err=%v", found, err)
	}
}

// Inject a hard failure rather than relying on malformed Go: the Go parser
// intentionally returns a partial AST for ordinary syntax errors.
type failingGoExtractor struct{ goast.Extractor }

func (e failingGoExtractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if strings.Contains(string(file.Content), "// fail extraction") {
		return model.Extraction{}, errors.New("injected extraction failure")
	}
	return e.Extractor.Extract(ctx, file)
}
