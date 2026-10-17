package indexer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/store"
)

func TestIndexerRecoversInterruptedLinkingAfterReopen(t *testing.T) {
	for _, tc := range []struct {
		name         string
		initialIndex bool
		interrupts   int
	}{
		{"initial index", true, 1},
		{"incremental update", false, 1},
		{"repeated interruption", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			write(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() bool { return true }\n")
			caller := filepath.Join(root, "http.go")
			write(t, caller, "package auth\nfunc Authenticate() bool { return false }\n")
			cfg := config.Default()
			var s *store.Store
			var ix *Indexer
			open := func() {
				t.Helper()
				var err error
				s, err = store.Open(root, cfg.IndexDirectory)
				if err != nil {
					t.Fatal(err)
				}
				ix = New(root, cfg, s, extract.NewRegistry(goast.NewExtractor()))
			}
			open()
			t.Cleanup(func() { _ = s.Close() })
			if !tc.initialIndex {
				if _, err := ix.Run(ctx, true); err != nil {
					t.Fatal(err)
				}
			}
			write(t, caller, "package auth\nfunc Authenticate() bool { return ValidateToken() }\n")
			for attempt := 0; attempt < tc.interrupts; attempt++ {
				cancelCtx, cancel := context.WithCancel(ctx)
				_, err := ix.RunWithProgress(cancelCtx, false, func(event Progress) {
					if event.Phase == PhaseLinking && event.Completed == 0 {
						cancel()
					}
				})
				cancel()
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("interruption %d: got %v, want cancellation during linking", attempt, err)
				}
				// File writes committed before cancellation; reopening must not
				// lose the outstanding relation work when file hashes now match.
				if _, found, err := s.FileHash(ctx, "http.go"); err != nil || !found {
					t.Fatalf("committed caller found=%v err=%v", found, err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				open()
			}
			run, err := ix.Run(ctx, false)
			if err != nil || run.FilesUnchanged != 2 || run.FilesChanged != 0 || run.FilesAdded != 0 {
				t.Fatalf("unchanged recovery run=%+v err=%v", run, err)
			}
			relations, err := s.Relations(ctx)
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
				t.Fatalf("unchanged recovery left cross-file call unresolved: %+v", relations)
			}
			linkProgress := false
			if _, err := ix.RunWithProgress(ctx, false, func(event Progress) {
				linkProgress = linkProgress || event.Phase == PhaseLinking
			}); err != nil {
				t.Fatal(err)
			}
			if linkProgress {
				t.Fatal("successful recovery must return to scoped incremental linking")
			}
		})
	}
}
