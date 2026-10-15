package indexlock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func mustAcquire(t *testing.T, dir string, kind Kind, mode Mode) *Lock {
	t.Helper()
	l, err := Acquire(context.Background(), dir, kind, mode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Error(err)
		}
	})
	return l
}

func expectBusy(t *testing.T, dir string, kind Kind, mode Mode) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	l, err := Acquire(ctx, dir, kind, mode)
	if l != nil {
		l.Close()
	}
	if !errors.Is(err, ErrBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want typed busy error and caller deadline, got %v", err)
	}
}

func TestSharedReadersAndExclusiveWriters(t *testing.T) {
	dir := t.TempDir()
	first := mustAcquire(t, dir, Lifecycle, Shared)
	second := mustAcquire(t, dir, Lifecycle, Shared)
	expectBusy(t, dir, Lifecycle, Exclusive)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	expectBusy(t, dir, Lifecycle, Exclusive)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	writer := mustAcquire(t, dir, Lifecycle, Exclusive)
	expectBusy(t, dir, Lifecycle, Shared)
	expectBusy(t, dir, Lifecycle, Exclusive)
	writer.Close()
	mustAcquire(t, dir, Lifecycle, Shared)
}

func TestWriterScopeAndIndependentIndexes(t *testing.T) {
	dir := t.TempDir()
	mustAcquire(t, dir, Lifecycle, Shared)
	mustAcquire(t, dir, Writer, Exclusive)
	expectBusy(t, dir, Writer, Exclusive)
	mustAcquire(t, t.TempDir(), Writer, Exclusive)
}

func TestCancellationAndDefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	mustAcquire(t, dir, Writer, Exclusive)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if l, err := Acquire(ctx, dir, Writer, Exclusive); l != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, %v", l, err)
	}
	l, err := Acquire(context.Background(), dir, Writer, Exclusive)
	if l != nil {
		l.Close()
	}
	var busy *BusyError
	if !errors.As(err, &busy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("default wait must expire: %v", err)
	}
}

func TestWaitDeadlineIsSharedWithoutCancellingWork(t *testing.T) {
	// Exhaust the acquisition budget without sleeping or cancelling the caller.
	ctx := WithWaitDeadline(context.Background(), time.Now().Add(-time.Second))
	// A second stage must not reset the budget granted to the first stage.
	ctx = WithWaitDeadline(ctx, time.Now().Add(time.Minute))
	l, err := Acquire(ctx, t.TempDir(), Writer, Exclusive)
	if l != nil {
		l.Close()
	}
	if !errors.Is(err, ErrBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("exhausted shared wait budget: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("acquisition budget cancelled operation context: %v", ctx.Err())
	}
}

func TestWaiterAcquiresAfterRelease(t *testing.T) {
	dir := t.TempDir()
	held := mustAcquire(t, dir, Writer, Exclusive)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		l, err := Acquire(context.Background(), dir, Writer, Exclusive)
		if err == nil {
			err = l.Close()
		}
		done <- err
	}()
	<-started
	held.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatalf("release must be idempotent: %v", err)
	}
}

func TestLockReleasedWhenProcessDies(t *testing.T) {
	for _, mode := range []Mode{Shared, Exclusive} {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockHelper$")
			cmd.Env = append(os.Environ(), "FOCALSPAN_LOCK_HELPER="+dir, "FOCALSPAN_LOCK_MODE="+string(mode))
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); cmd.Wait() })
			reader := bufio.NewReader(pipe)
			ready, err := reader.ReadString('\n')
			if err != nil || ready != "locked\n" {
				t.Fatalf("child handshake %q: %v", ready, err)
			}
			if mode == Shared {
				own := mustAcquire(t, dir, Lifecycle, Shared)
				expectBusy(t, dir, Lifecycle, Exclusive)
				own.Close()
			} else {
				expectBusy(t, dir, Lifecycle, Shared)
				expectBusy(t, dir, Lifecycle, Exclusive)
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			cmd.Wait()
			mustAcquire(t, dir, Lifecycle, Exclusive)
		})
	}
}

func TestLockHelper(t *testing.T) {
	dir := os.Getenv("FOCALSPAN_LOCK_HELPER")
	if dir == "" {
		return
	}
	l, err := Acquire(context.Background(), dir, Lifecycle, Mode(os.Getenv("FOCALSPAN_LOCK_MODE")))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	fmt.Println("locked")
	var b [1]byte
	os.Stdin.Read(b[:])
}
