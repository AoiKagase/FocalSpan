package app

import (
	"context"
	"database/sql"
	"errors"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/search"
	"github.com/focalspan/focalspan/internal/store"
	"path/filepath"
	"sync"
	"testing"
)

func TestRevisionReadErrorsPropagate(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.revision(context.Background()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing revision err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.revision(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled revision err=%v", err)
	}
}

type cancelAfterPaths struct {
	*store.Store
	once          sync.Once
	entered, done chan struct{}
}

func (r *cancelAfterPaths) ScopedCandidates(model.CandidateScope) model.CandidateReader { return r }
func (r *cancelAfterPaths) SearchPaths(ctx context.Context, q []string, n int) ([]model.RankedCandidate, error) {
	got, err := r.Store.SearchPaths(ctx, q, n)
	r.once.Do(func() { close(r.entered); <-r.done })
	return got, err
}
func TestEvidenceHonorsCancellationAfterCandidates(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "notes.txt"), "context cancellation evidence\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &cancelAfterPaths{Store: s.Store, entered: make(chan struct{}), done: make(chan struct{})}
	s.searcher = search.New(reader)
	go func() { <-reader.entered; cancel(); close(reader.done) }()
	got, err := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "notes", NoUpdate: true, TokenBudget: 1200})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation ignored: err=%v packet=%+v", err, got.Packet)
	}
}
func TestReadIndexHonorsLateCancellationIncludingNestedRead(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, nested := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if nested {
			ctx = context.WithValue(ctx, indexReadKey{}, s)
		}
		got, err := readIndex(s, ctx, false, false, func(context.Context) (string, error) { cancel(); return "should not escape", nil })
		if !errors.Is(err, context.Canceled) || got != "" {
			t.Fatalf("nested=%t got=%q err=%v", nested, got, err)
		}
	}
}
