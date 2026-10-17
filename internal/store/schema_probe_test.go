package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/indexlock"
)

func TestSchemaProbeWaitsForSQLiteLockRelease(t *testing.T) {
	root, directory, release := lockedSchemaProbeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	type result struct {
		store *Store
		err   error
	}
	done := make(chan result, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		st, err := openWithLease(ctx, root, directory, false)
		done <- result{st, err}
	}()
	<-started
	select {
	case got := <-done:
		if got.store != nil {
			got.store.Close()
		}
		t.Fatalf("schema probe returned before SQLite lock release: %v", got.err)
	case <-time.After(75 * time.Millisecond):
	}
	release()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("open after lock release: %v", got.err)
		}
		defer got.store.Close()
		if version, err := got.store.Meta(context.Background(), "schema_version"); err != nil || version != "2" {
			t.Fatalf("reopened schema=%q err=%v", version, err)
		}
	case <-ctx.Done():
		t.Fatal("schema probe did not recover after SQLite lock release")
	}
}

func TestSchemaProbeDeadlineReleasesLifecycleLease(t *testing.T) {
	root, directory, _ := lockedSchemaProbeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	st, err := openWithLease(ctx, root, directory, false)
	if st != nil {
		st.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("schema probe must honor caller deadline: %v", err)
	}
	leaseCtx, stop := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer stop()
	lease, err := indexlock.Acquire(leaseCtx, directory, indexlock.Lifecycle, indexlock.Exclusive)
	if err != nil {
		t.Fatalf("deadline leaked lifecycle lease: %v", err)
	}
	lease.Close()
}

func TestSchemaProbeHonorsCancellation(t *testing.T) {
	root, directory, _ := lockedSchemaProbeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	st, err := openWithLease(ctx, root, directory, false)
	if st != nil {
		st.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("schema probe ignored cancellation: %v", err)
	}
}

func TestSchemaProbeDoesNotRetryCorruptDatabase(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".focalspan")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.db"), []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	st, err := openWithLease(ctx, root, directory, false)
	if st != nil {
		st.Close()
	}
	if err == nil || ctx.Err() != nil {
		t.Fatalf("non-busy error should return without waiting for deadline: %v (ctx=%v)", err, ctx.Err())
	}
}

func lockedSchemaProbeFixture(t *testing.T) (string, string, func()) {
	t.Helper()
	root := t.TempDir()
	st, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	path := st.DBPath()
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if _, err := db.Exec("ROLLBACK"); err != nil {
				t.Errorf("release SQLite lock: %v", err)
			}
			db.Close()
		})
	}
	t.Cleanup(release)
	return root, filepath.Dir(path), release
}
