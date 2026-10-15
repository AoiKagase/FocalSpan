package store

import (
	"bytes"
	"context"
 "errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/indexlock"
)

func TestPendingUpgradeExcludesOtherOpenersAndReleasesAfterFailure(t *testing.T) {
	root := t.TempDir()
	legacy, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.SetMeta(context.Background(), "schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	pending, err := OpenForUpdate(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Close()
	temp := pending.DBPath()
	before, err := os.ReadFile(temp)
	if err != nil {
		t.Fatal(err)
	}
	for _, allow := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		other, err := openWithLease(ctx, root, filepath.Dir(temp), allow)
		cancel()
		if other != nil {
			other.Close()
		}
		if !errors.Is(err, indexlock.ErrBusy) {
			t.Fatalf("second opener must be excluded: %v", err)
		}
	}
	after, err := os.ReadFile(temp)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("contender changed pending database: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pending.FinalizeUpgrade(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("finalize: %v", err)
	}
	if err := pending.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary database not cleaned up: %v", err)
	}
	next, err := OpenForUpdate(root, ".focalspan")
	if err != nil {
		t.Fatalf("failed upgrade leaked its lifecycle lease: %v", err)
	}
	defer next.Close()
	if err := next.FinalizeUpgrade(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedSchemaOpenReleasesLifecycleLease(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetMeta(context.Background(), "schema_version", "unsupported"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	bad, err := Open(root, ".focalspan")
	if bad != nil {
		bad.Close()
	}
	if err == nil {
		t.Fatal("unsupported schema was accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	l, err := indexlock.Acquire(ctx, filepath.Join(root, ".focalspan"), indexlock.Lifecycle, indexlock.Exclusive)
	if err != nil {
		t.Fatalf("failed open leaked its lease: %v", err)
	}
	l.Close()
}

func TestConcurrentInitialOpenKeepsReadersAlive(t *testing.T) {
	root := t.TempDir()
	const count = 12
	start := make(chan struct{})
	type result struct {
		store *Store
		err   error
	}
	done := make(chan result, count)
	var ready sync.WaitGroup
	ready.Add(count)
	for range count {
		go func() {
			ready.Done()
			<-start
			st, err := Open(root, ".focalspan")
			done <- result{st, err}
		}()
	}
	ready.Wait()
	close(start)
	// Keep successful readers open until all constructors finish. A constructor
	// which missed initialization must recheck instead of waiting for readers to exit.
	var opened []*Store
	defer func() {
		for _, st := range opened {
			st.Close()
		}
	}()
	for range count {
		r := <-done
		if r.err != nil {
			t.Error(r.err)
		}
		if r.store != nil {
			opened = append(opened, r.store)
		}
	}
}

func TestCurrentSchemaOpenDoesNotRunMigrationWrites(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tx, err := st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO meta(key,value) VALUES('uncommitted','value')"); err != nil {
		t.Fatal(err)
	}
	reader, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatalf("current schema open attempted a write: %v", err)
	}
	defer reader.Close()
}

func TestUpgradePreservesArtifactsWhileStoreIsInUse(t *testing.T) {
	root := t.TempDir()
	reader, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := reader.SetMeta(context.Background(), "schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	temp := reader.DBPath() + ".v2.tmp"
	if err := os.WriteFile(temp, []byte("another session owns this"), 0600); err != nil {
		t.Fatal(err)
	}
	upgraded, err := OpenForUpdate(root, ".focalspan")
	if upgraded != nil {
		upgraded.Close()
	}
	if err == nil {
		t.Fatal("upgrade succeeded while another Store still uses the database")
	}
	if !strings.Contains(err.Error(), "restart") {
		t.Fatalf("missing actionable restart guidance: %v", err)
	}
	data, readErr := os.ReadFile(temp)
	if readErr != nil || string(data) != "another session owns this" {
		t.Fatalf("contended upgrade changed temporary database: %q, %v", data, readErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err = OpenForUpdate(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if err := upgraded.FinalizeUpgrade(context.Background()); err != nil {
		t.Fatal(err)
	}
	if upgraded.DBPath() != filepath.Join(root, ".focalspan", "index.db") {
		t.Fatal(upgraded.DBPath())
	}
}
