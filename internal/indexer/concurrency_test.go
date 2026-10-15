package indexer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/focalspan/focalspan/internal/store"
)

// A probe context acknowledges the wait loop without timing-based sleeps.
type waitProbe struct {
	context.Context
	entered chan struct{}
}

func (p *waitProbe) Deadline() (deadline time.Time, ok bool) {
	select {
	case p.entered <- struct{}{}:
	default:
	}
	return p.Context.Deadline()
}

func TestIndexerWaitsBeforeScanningAndRescans(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	st, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ix := New(root, cfg, st, extract.NewRegistry(goast.NewExtractor()))
	held, err := indexlock.Acquire(context.Background(), filepath.Join(root, cfg.IndexDirectory), indexlock.Writer, indexlock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probe := &waitProbe{Context: ctx, entered: make(chan struct{}, 1)}
	done := make(chan error, 1)
	scanned := make(chan struct{}, 1)
	go func() {
		_, err := ix.RunWithProgress(probe, false, func(p Progress) {
			if p.Phase == PhaseScanning {
				scanned <- struct{}{}
			}
		})
		done <- err
	}()
	select {
	case <-scanned:
		t.Fatal("indexer scanned before acquiring the writer lock")
	case err := <-done:
		t.Fatalf("indexer did not wait: %v", err)
	case <-probe.entered:
	}
	write(t, filepath.Join(root, "after.go"), "package after\nfunc AddedWhileWaiting() {}\n")
	held.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	paths, err := st.Paths(context.Background())
	if _, found := paths["after.go"]; err != nil || !found {
		t.Fatalf("waiting run did not see new source: %v, %v", paths, err)
	}
}

func TestIndexerCancellationDoesNotScanOrWrite(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	st, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ix := New(root, cfg, st, extract.NewRegistry(goast.NewExtractor()))
	held, err := indexlock.Acquire(context.Background(), filepath.Join(root, cfg.IndexDirectory), indexlock.Writer, indexlock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanned := false
	_, err = ix.RunWithProgress(ctx, false, func(Progress) { scanned = true })
	if !errors.Is(err, context.Canceled) || scanned {
		t.Fatalf("cancelled run emitted progress=%v, err=%v", scanned, err)
	}
	held.Close()
	// Cancel after acquisition, during the scan phase, and verify the deferred
	// writer release permits the next independent run.
	during, cancelDuring := context.WithCancel(context.Background())
	_, err = ix.RunWithProgress(during, false, func(p Progress) {
		if p.Phase == PhaseScanning {
			cancelDuring()
		}
	})
	cancelDuring()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("scan cancellation: %v", err)
	}
	_, err = ix.Run(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
}
