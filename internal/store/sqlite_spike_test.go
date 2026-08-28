package store_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteFTS5Driver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE VIRTUAL TABLE chunk_fts USING fts5(path, content)`); err != nil {
		t.Fatalf("create FTS5 table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chunk_fts(path, content) VALUES (?, ?)`, "auth/service.go", "expired token validation"); err != nil {
		t.Fatalf("insert FTS5 row: %v", err)
	}
	var path string
	var score float64
	if err := db.QueryRow(`SELECT path, bm25(chunk_fts) FROM chunk_fts WHERE chunk_fts MATCH ?`, `expired AND token`).Scan(&path, &score); err != nil {
		t.Fatalf("MATCH/bm25 query: %v", err)
	}
	if path != "auth/service.go" {
		t.Fatalf("path = %q, want auth/service.go", path)
	}
}
