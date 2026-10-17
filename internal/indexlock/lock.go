// Package indexlock coordinates cooperating FocalSpan processes on local filesystems.
package indexlock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Kind string
type Mode string

const (
	Lifecycle     Kind = "lifecycle"
	Writer        Kind = "writer"
	Configuration Kind = "configuration"
	Shared        Mode = "shared"
	Exclusive     Mode = "exclusive"
	waitLimit          = 5 * time.Second
	retryInterval      = 25 * time.Millisecond
)

var ErrBusy = errors.New("index is in use")

type waitDeadlineKey struct{}

// WithWaitDeadline shares an acquisition budget across multiple lock stages.
// It does not cancel the operation itself after the locks have been acquired.
// Nested budgets can shorten the deadline, but can never extend it.
func WithWaitDeadline(ctx context.Context, deadline time.Time) context.Context {
	if previous, ok := ctx.Value(waitDeadlineKey{}).(time.Time); ok && previous.Before(deadline) {
		return ctx
	}
	return context.WithValue(ctx, waitDeadlineKey{}, deadline)
}

// WaitContext bounds only the waiting phase, including any inherited budget.
func WaitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(waitLimit)
	if inherited, ok := ctx.Value(waitDeadlineKey{}).(time.Time); ok && inherited.Before(deadline) {
		deadline = inherited
	}
	return context.WithDeadline(ctx, deadline)
}

type BusyError struct {
	Path  string
	Kind  Kind
	Mode  Mode
	Cause error
}

func (e *BusyError) Error() string {
	guidance := "retry after the current index update completes"
	if e.Kind == Lifecycle {
		guidance = "close other FocalSpan sessions and restart before creating or migrating the index"
	}
	if e.Kind == Configuration {
		guidance = "retry after the current configuration update completes"
	}
	return fmt.Sprintf("%s: %s %s lock at %s: %v; %s", ErrBusy, e.Mode, e.Kind, e.Path, e.Cause, guidance)
}
func (e *BusyError) Unwrap() error        { return e.Cause }
func (e *BusyError) Is(target error) bool { return target == ErrBusy }

// Lock retains its open file until Close. Lock files must never be unlinked:
// another process may already be waiting on the same file.
type Lock struct {
	file *os.File
	once sync.Once
	err  error
}

// Acquire waits at most five seconds, bounded by the caller's context.
// Use a fresh file handle for each acquisition, including within one process.
func Acquire(ctx context.Context, directory string, kind Kind, mode Mode) (*Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if (kind != Lifecycle && kind != Writer) || (mode != Shared && mode != Exclusive) {
		return nil, fmt.Errorf("invalid index lock kind %q or mode %q", kind, mode)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	path := filepath.Join(directory, ".index-"+string(kind)+".lock")
	return acquireFile(ctx, path, kind, mode)
}

// AcquireConfiguration locks a persistent per-configuration lock file.
// The caller owns creating its parent; the lock file must never be removed.
func AcquireConfiguration(ctx context.Context, path string) (*Lock, error) {
	return acquireFile(ctx, path, Configuration, Exclusive)
}

func acquireFile(ctx context.Context, path string, kind Kind, mode Mode) (*Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open index lock: %w", err)
	}
	waitCtx, cancel := WaitContext(ctx)
	defer cancel()
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		if err := waitCtx.Err(); err != nil {
			f.Close()
			return nil, &BusyError{Path: path, Kind: kind, Mode: mode, Cause: err}
		}
		acquired, err := tryLock(f, mode)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("acquire %s index lock: %w", kind, err)
		}
		if acquired {
			return &Lock{file: f}, nil
		}
		select {
		case <-waitCtx.Done():
		case <-ticker.C:
		}
	}
}

func (l *Lock) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() { l.err = errors.Join(unlock(l.file), l.file.Close()) })
	return l.err
}
