package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/search"
	"github.com/focalspan/focalspan/internal/store"
)

type pausedCandidateReader struct {
	*store.Store
	paused          atomic.Bool
	entered, resume chan struct{}
}

func (p *pausedCandidateReader) SearchFTS(ctx context.Context, q string, n int) ([]model.RankedCandidate, error) {
	got, err := p.Store.SearchFTS(ctx, q, n)
	if p.paused.CompareAndSwap(false, true) {
		close(p.entered)
		select {
		case <-p.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return got, err
}

func TestReadLeaseKeepsPacketRevisionAndAllowsParallelReaders(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() bool { return false }\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Index(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	reader := &pausedCandidateReader{Store: s.Store, entered: make(chan struct{}), resume: make(chan struct{})}
	s.searcher = search.New(reader)
	result := make(chan error, 1)
	go func() {
		got, err := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken", NoUpdate: true, TokenBudget: 1200, Mode: evidence.ModeSource})
		if err == nil && (got.Packet.Revision != first.Revision || len(got.Packet.Evidence) != 1) {
			err = fmt.Errorf("inconsistent packet %+v", got.Packet)
		}
		result <- err
	}()
	<-reader.entered
	defer func() {
		if !reader.paused.Load() {
			return
		}
		select {
		case <-reader.resume:
		default:
			close(reader.resume)
		}
	}()
	parallel := make(chan error, 1)
	go func() { _, err := s.Query(ctx, QueryRequest{Query: "ValidateToken", NoUpdate: true}); parallel <- err }()
	select {
	case err := <-parallel:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel readers blocked each other")
	}
	wait, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	lease, err := indexlock.Acquire(wait, filepath.Dir(s.Store.DBPath()), indexlock.Writer, indexlock.Exclusive)
	if lease != nil {
		lease.Close()
	}
	if !errors.Is(err, indexlock.ErrBusy) {
		t.Errorf("writer entered during packet read: %v", err)
	}
	close(reader.resume)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() bool { return true }\n")
	if _, err = s.Index(ctx, false); err != nil {
		t.Fatal(err)
	}
}

func TestReadLeaseWaitsForOtherProcess(t *testing.T) {
	if dir := os.Getenv("FOCALSPAN_READ_LEASE_HELPER"); dir != "" {
		lease, err := indexlock.Acquire(context.Background(), dir, indexlock.Writer, indexlock.Exclusive)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		fmt.Println("locked")
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
		return
	}
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() {}\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestReadLeaseWaitsForOtherProcess$")
	command.Env = append(os.Environ(), "FOCALSPAN_READ_LEASE_HELPER="+filepath.Dir(s.Store.DBPath()))
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); _ = command.Wait() }()
	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || lines.Text() != "locked" {
		t.Fatal("helper did not acquire writer lease")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken", NoUpdate: true, TokenBudget: 1200})
	if !errors.Is(err, indexlock.ErrBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read entered during other process write: %v", err)
	}
}

func TestAllPublicReadsHonorWriterWaitCancellation(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() {}\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Config.AutoUpdateBeforeQuery = false
	if _, err = s.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	writer, err := indexlock.Acquire(context.Background(), filepath.Dir(s.Store.DBPath()), indexlock.Writer, indexlock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reads := []struct {
		name string
		call func(context.Context) error
	}{
		{"query", func(ctx context.Context) error {
			_, err := s.Query(ctx, QueryRequest{Query: "ValidateToken", NoUpdate: true})
			return err
		}},
		{"query evidence", func(ctx context.Context) error {
			_, err := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken", NoUpdate: true})
			return err
		}},
		{"expand", func(ctx context.Context) error { _, err := s.Expand(ctx, []string{"handle"}, "self", 1200); return err }},
		{"expand evidence", func(ctx context.Context) error {
			_, err := s.ExpandEvidence(ctx, EvidenceExpandRequest{Handles: []string{"handle"}})
			return err
		}},
		{"impact", func(ctx context.Context) error { _, err := s.Impact(ctx, "", "", 1200); return err }},
		{"impact evidence", func(ctx context.Context) error { _, err := s.ImpactEvidence(ctx, EvidenceImpactRequest{}); return err }},
	}
	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			err := read.call(ctx)
			if !errors.Is(err, indexlock.ErrBusy) || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
