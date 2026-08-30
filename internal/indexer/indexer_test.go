package indexer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	templateextract "github.com/focalspan/focalspan/internal/extract/template"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
)

func TestIndexerIncrementalStatsAndDeletion(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() {}\n")
	write(t, filepath.Join(root, "README.md"), "# Auth\n\nexpired token\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor(), generic.NewExtractor()))
	first, err := ix.Run(context.Background(), true)
	if err != nil || first.FilesSeen != 2 || first.FilesAdded != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := ix.Run(context.Background(), false)
	if err != nil || second.FilesUnchanged != 2 || second.FilesChanged != 0 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	third, err := ix.Run(context.Background(), false)
	if err != nil || third.FilesChanged != 1 || third.FilesUnchanged != 1 {
		t.Fatalf("third=%+v err=%v", third, err)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	fourth, err := ix.Run(context.Background(), false)
	if err != nil || fourth.FilesDeleted != 1 {
		t.Fatalf("fourth=%+v err=%v", fourth, err)
	}
}

func TestIndexerReportsProgressDuringInitialIndex(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() {}\n")
	write(t, filepath.Join(root, "README.md"), "# Auth\n\nexpired token\n")
	cfg := config.Default()
	cfg.Workers = 2
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor(), generic.NewExtractor()))
	var events []Progress
	_, err = ix.RunWithProgress(context.Background(), true, func(event Progress) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[0].Phase != PhaseScanning {
		t.Fatalf("events=%+v, want scanning first", events)
	}
	seen := map[string]bool{}
	lastByPhase := map[string]int{}
	for _, event := range events {
		seen[event.Phase] = true
		if event.Phase == PhaseChecking || event.Phase == PhaseParsing {
			if event.Completed < 0 || event.Completed > event.Total || event.Completed < lastByPhase[event.Phase] {
				t.Fatalf("non-monotonic progress event=%+v events=%+v", event, events)
			}
			lastByPhase[event.Phase] = event.Completed
		}
	}
	for _, phase := range []string{PhaseChecking, PhaseParsing, PhaseWriting, PhaseComplete} {
		if !seen[phase] {
			t.Fatalf("events=%+v missing phase %q", events, phase)
		}
	}
	last := events[len(events)-1]
	if last.Phase != PhaseComplete || last.Completed != 1 || last.Total != 1 {
		t.Fatalf("last event=%+v", last)
	}

	var updateEvents []Progress
	if _, err := ix.RunWithProgress(context.Background(), false, func(event Progress) {
		updateEvents = append(updateEvents, event)
	}); err != nil {
		t.Fatal(err)
	}
	checkingComplete := false
	for _, event := range updateEvents {
		if event.Phase == PhaseChecking && event.Completed == 2 && event.Total == 2 {
			checkingComplete = true
		}
	}
	if !checkingComplete {
		t.Fatalf("update events=%+v missing checking 2/2", updateEvents)
	}
}

func TestIndexerReindexesWhenExtractorVersionChanges(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "Service.cs"), "public class Service { public bool ValidateToken(string token) { return token.Length > 0; } }\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(generic.NewExtractor()))
	if _, err := ix.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(context.Background(), "extractor_version", "outdated"); err != nil {
		t.Fatal(err)
	}

	run, err := ix.Run(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if run.FilesUnchanged != 0 || run.FilesChanged != 1 {
		t.Fatalf("run=%+v, want unchanged=0 changed=1 after extractor update", run)
	}
}

func TestIndexerReindexesOldTemplateWindowsAfterExtractorVersionChanges(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "view.tpl"), `{block name="content"}Hello{/block}`)
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old := New(root, cfg, s, extract.NewRegistry(generic.NewExtractor()))
	if _, err := old.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(context.Background(), "extractor_version", "generic-structured-v2"); err != nil {
		t.Fatal(err)
	}
	current := New(root, cfg, s, extract.NewRegistry(templateextract.NewExtractor(), generic.NewExtractor()))
	run, err := current.Run(context.Background(), false)
	if err != nil || run.FilesChanged != 1 || run.FilesUnchanged != 0 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	candidates, err := s.SearchFTS(context.Background(), `"content"`, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Path == "view.tpl" && candidate.Kind == "window" {
			t.Fatalf("old generic window remained: %+v", candidates)
		}
	}
}

func TestIndexerRefreshesAllFilesOnceForPolyglotExtractorVersion(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "view.tpl"), `{block name="content"}Hello{/block}`)
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(templateextract.NewExtractor(), generic.NewExtractor()))
	if _, err := ix.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(context.Background(), "extractor_version", "extractors-v4"); err != nil {
		t.Fatal(err)
	}
	first, err := ix.Run(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if first.FilesChanged != 1 || first.FilesUnchanged != 0 {
		t.Fatalf("first version refresh run=%+v", first)
	}
	second, err := ix.Run(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if second.FilesChanged != 0 || second.FilesUnchanged != 1 {
		t.Fatalf("second version refresh run=%+v", second)
	}
	candidates, err := s.SearchFTS(context.Background(), `"content"`, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Path == "view.tpl" && candidate.Kind == "window" {
			t.Fatalf("old generic window remained after version refresh: %+v", candidates)
		}
	}
}

func TestIndexerLinksUniqueCrossFileCallAfterApplyIndex(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() bool { return true }\n")
	write(t, filepath.Join(root, "http.go"), "package auth\n\nfunc Authenticate() bool { return ValidateToken() }\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor(), generic.NewExtractor()))
	if _, err := ix.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	linked := false
	for _, relation := range relations {
		if relation.Kind == "calls" && relation.ToHandle != "" && relation.UnresolvedTo == "" {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("cross-file call was not linked: %+v", relations)
	}
}

func TestIndexerCancellationRollsBackPendingChanges(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "old.go"), "package old\n\nfunc Old() {}\n")
	cfg := config.Default()
	s, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor(), generic.NewExtractor()))
	if _, err := ix.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "new.go"), "package new\n\nfunc New() {}\n")
	ctx, cancel := context.WithCancel(context.Background())
	ix.registry = extract.NewRegistry(cancellingExtractor{cancel: cancel})
	_, err = ix.Run(ctx, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context.Canceled", err)
	}
	if _, found, err := s.FileHash(context.Background(), "old.go"); err != nil || !found {
		t.Fatalf("old file after cancellation found=%v err=%v", found, err)
	}
	if _, found, err := s.FileHash(context.Background(), "new.go"); err != nil || found {
		t.Fatalf("new file after cancellation found=%v err=%v", found, err)
	}
}

type cancellingExtractor struct {
	cancel context.CancelFunc
}

func (cancellingExtractor) Name() string { return "cancelling" }

func (cancellingExtractor) Supports(string, string) bool { return true }

func (e cancellingExtractor) Extract(context.Context, model.SourceFile) (model.Extraction, error) {
	e.cancel()
	return model.Extraction{}, context.Canceled
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
