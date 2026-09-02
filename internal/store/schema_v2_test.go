package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestOpenCreatesSchemaV2(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	version, err := s.Meta(context.Background(), "schema_version")
	if err != nil {
		t.Fatal(err)
	}
	if version != "2" {
		t.Fatalf("schema version=%q, want 2", version)
	}
}

func TestOpenForUpdateRebuildsLegacyAndFinalizesAtomically(t *testing.T) {
	root := t.TempDir()
	legacy, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.SetMeta(context.Background(), "schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	livePath := legacy.DBPath()
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	updated, err := OpenForUpdate(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if updated.DBPath() == livePath {
		updated.Close()
		t.Fatal("update open did not use a temporary database")
	}
	if err := updated.ReplaceFile(context.Background(), model.SourceFile{Path: "main.go", Language: "go", SHA256: "new"}, model.Extraction{}); err != nil {
		updated.Close()
		t.Fatal(err)
	}
	if err := updated.FinalizeUpgrade(context.Background()); err != nil {
		updated.Close()
		t.Fatal(err)
	}
	defer updated.Close()
	if updated.DBPath() != filepath.Join(root, ".focalspan", "index.db") {
		t.Fatalf("final db path=%q", updated.DBPath())
	}
	version, err := updated.Meta(context.Background(), "schema_version")
	if err != nil || version != "2" {
		t.Fatalf("version=%q err=%v", version, err)
	}
	if _, err := os.Stat(livePath + ".v1.bak"); !os.IsNotExist(err) {
		t.Fatalf("legacy backup still exists: %v", err)
	}
}

func TestReplaceAndDeleteMaintainSchemaV2Lookups(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	extraction := model.Extraction{
		Symbols:   []model.Symbol{{Handle: "main", Name: "Run", QualifiedName: "app.Run"}},
		Relations: []model.Relation{{FromHandle: "main", UnresolvedTo: "app.Run", Kind: "calls"}},
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/main.go", Language: "go", SHA256: "main"}, extraction); err != nil {
		t.Fatal(err)
	}
	var symbolCount, fileCount, relationCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM symbol_lookup`).Scan(&symbolCount); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM file_lookup`).Scan(&fileCount); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM relation_lookup`).Scan(&relationCount); err != nil {
		t.Fatal(err)
	}
	if symbolCount != 2 || fileCount != 7 || relationCount != 2 {
		t.Fatalf("lookup counts symbol=%d file=%d relation=%d", symbolCount, fileCount, relationCount)
	}
	if err := s.DeleteFile(context.Background(), "src/main.go"); err != nil {
		t.Fatal(err)
	}
	for name, query := range map[string]string{"symbol": `SELECT COUNT(*) FROM symbol_lookup`, "file": `SELECT COUNT(*) FROM file_lookup`, "relation": `SELECT COUNT(*) FROM relation_lookup`} {
		var count int
		if err := s.db.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s lookup count=%d after delete", name, count)
		}
	}
}

func TestApplyIndexDeltaIncludesReplacedLookupKeys(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.ReplaceFile(ctx, model.SourceFile{Path: "target.go", Language: "go", SHA256: "old"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "target-old", FilePath: "target.go", Language: "go", Kind: "function", Name: "OldName", QualifiedName: "OldName", Signature: "func OldName()", StartLine: 1, EndLine: 1, Confidence: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	delta, err := s.ApplyIndexWithDelta(ctx, nil, []FileUpdate{{File: model.SourceFile{Path: "target.go", Language: "go", SHA256: "new"}, Extraction: model.Extraction{Symbols: []model.Symbol{{Handle: "target-new", FilePath: "target.go", Language: "go", Kind: "function", Name: "NewName", QualifiedName: "NewName", Signature: "func NewName()", StartLine: 1, EndLine: 1, Confidence: 1}}}}}, nil, model.IndexRun{})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, key := range delta.LookupKeys {
		keys[key] = true
	}
	if !keys["oldname"] || !keys["newname"] {
		t.Fatalf("lookup delta=%v, want old and new keys", delta.LookupKeys)
	}
}

func TestNormalOpenRejectsV1WithoutMutatingDatabase(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(context.Background(), "schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	dbPath := s.DBPath()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(root, ".focalspan")
	if opened != nil {
		_ = opened.Close()
		t.Fatal("normal open returned a store for schema v1")
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "schema") || !strings.Contains(strings.ToLower(err.Error()), "upgrade") {
		t.Fatalf("error=%v, want schema upgrade diagnostic", err)
	}
	after, readErr := os.ReadFile(dbPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("normal open mutated the v1 database")
	}
}

func TestOpenRejectsFutureSchemaWithoutMutatingDatabase(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(context.Background(), "schema_version", "999"); err != nil {
		_ = s.Close()
		t.Fatal(err)
	}
	path := s.DBPath()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenForUpdate(root, ".focalspan"); err == nil || strings.Contains(err.Error(), "schema upgrade required") {
		t.Fatalf("future schema error=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("future schema open mutated the database")
	}
}
