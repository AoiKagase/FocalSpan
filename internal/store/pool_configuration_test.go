package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestPoolConnectionsEnforceSettingsAndForeignKeys(t *testing.T) {
	s := openTestStore(t)
	for i, conn := range reserveStoreConnections(t, s, 4) {
		for _, setting := range []struct {
			name string
			want int
		}{{"foreign_keys", 1}, {"busy_timeout", 5000}, {"synchronous", 1}} {
			var got int
			if err := conn.QueryRowContext(context.Background(), "PRAGMA "+setting.name).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != setting.want {
				t.Errorf("connection %d: %s=%d, want %d", i+1, setting.name, got, setting.want)
			}
		}
		_, err := conn.ExecContext(context.Background(), `INSERT INTO diagnostics(file_id,level,code,message,created_at) VALUES(999,'warning','test','invalid parent','now')`)
		if err == nil {
			t.Errorf("connection %d accepted an invalid foreign key", i+1)
		}
	}
}

func TestPoolDeletionDoesNotLeakOldSourceIntoNewPath(t *testing.T) {
	s := openTestStore(t)
	storeSeedFile(t, s, "old.go", "go", "old", "Old", "Old", "old source", 1, 1, 1)
	// Reserve the first three physical connections so DeleteFile must open a
	// fourth one; it cannot accidentally reuse the initially configured handle.
	reserveStoreConnections(t, s, 3)
	if err := s.DeleteFile(context.Background(), "old.go"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"chunks", "symbols", "file_lookup", "symbol_lookup"} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("pooled deletion left %d orphan rows in %s", count, table)
		}
	}
	storeSeedFile(t, s, "fresh.go", "go", "fresh", "Fresh", "Fresh", "fresh source", 1, 1, 1)
	got, err := s.CandidatesForPath(context.Background(), "fresh.go")
	if err != nil || len(got) != 1 || got[0].Content != "fresh source" {
		t.Fatalf("new path inherited stale source: %+v err=%v", got, err)
	}
}

func TestPoolAdditionalConnectionWaitsForWriter(t *testing.T) {
	s := openTestStore(t)
	conns := reserveStoreConnections(t, s, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := conns[0].ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = conns[0].ExecContext(context.Background(), "ROLLBACK") })
	done := make(chan error, 1)
	go func() {
		_, err := conns[1].ExecContext(ctx, "INSERT INTO meta(key,value) VALUES('pooled_write','ok')")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("additional connection returned before writer release: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	if _, err := conns[0].ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("additional connection did not recover after writer release: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("additional connection remained blocked after writer release")
	}
}

func reserveStoreConnections(t *testing.T, s *Store, count int) []*sql.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conns := make([]*sql.Conn, 0, count)
	for range count {
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conns = append(conns, conn)
	}
	return conns
}
