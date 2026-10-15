package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/store"
)

// Pause exactly at FinalizeUpgrade's context check, after Indexer has released
// its writer lock. This exposes the otherwise tiny finalization gap.
type pausedFinalizeContext struct {
	context.Context
	armed   atomic.Bool
	once    sync.Once
	entered chan struct{}
	resume  chan struct{}
}

func (p *pausedFinalizeContext) Err() error {
	if p.armed.Load() {
		p.once.Do(func() { close(p.entered); <-p.resume })
	}
	return p.Context.Err()
}

func TestSameServiceWaiterCancelsDuringUpgradeFinalization(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	old, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetMeta(context.Background(), "schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	old.Close()
	s, err := NewWithConfigForUpdate(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	paused := &pausedFinalizeContext{Context: context.Background(), entered: make(chan struct{}), resume: make(chan struct{})}
	var release sync.Once
	resume := func() { release.Do(func() { close(paused.resume) }) }
	defer resume()
	first := make(chan error, 1)
	go func() {
		_, err := s.IndexWithProgress(paused, false, func(p IndexProgress) {
			if p.Phase == IndexPhaseComplete {
				paused.armed.Store(true)
			}
		})
		first <- err
	}()
	<-paused.entered
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var events atomic.Int32
	second := make(chan error, 1)
	go func() {
		_, err := s.IndexWithProgress(ctx, false, func(IndexProgress) { events.Add(1) })
		second <- err
	}()
	var secondErr error
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case secondErr = <-second:
	case <-timer.C:
		// Unblock both calls before failing, so cleanup never races a live run.
		resume()
		secondErr = <-second
		t.Error("waiter ignored its deadline while another call finalized the upgrade")
	}
	resume()
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(secondErr, context.DeadlineExceeded) || events.Load() != 0 {
		t.Fatalf("waiter must cancel before scanning: events=%d, err=%v", events.Load(), secondErr)
	}
}
