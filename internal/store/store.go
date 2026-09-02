package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/focalspan/focalspan/internal/model"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const schemaVersion = "2"

// SchemaUpgradeRequiredError reports that the on-disk derived index is an
// older schema and must be rebuilt through an explicitly allowed update path.
// Normal opens never mutate the old database.
type SchemaUpgradeRequiredError struct {
	Path    string
	Version string
}

func (e *SchemaUpgradeRequiredError) Error() string {
	if e == nil {
		return "schema upgrade required"
	}
	return fmt.Sprintf("schema upgrade required for %s (found version %q); run focalspan update --rebuild", e.Path, e.Version)
}

func IsSchemaUpgradeRequired(err error) bool {
	var target *SchemaUpgradeRequiredError
	return errors.As(err, &target)
}

type Store struct {
	db             *sql.DB
	root           string
	dbPath         string
	liveDBPath     string
	upgradePending bool
	mu             sync.RWMutex
}

type FileUpdate struct {
	File       model.SourceFile
	Extraction model.Extraction
}

type MetaUpdate struct {
	Key   string
	Value string
}

type IndexDelta struct {
	ChangedPaths []string
	LookupKeys   []string
}

type LinkScope struct {
	Full          bool
	UseProjection bool
	ChangedPaths  []string
	LookupKeys    []string
}

type RelationLink struct {
	FromHandle   string
	UnresolvedTo string
	Kind         string
	ToHandle     string
}

func Open(root, indexDir string) (*Store, error) {
	return open(root, indexDir, false)
}

// OpenForUpdate allows setup/update entry points to rebuild a legacy derived
// database in an isolated sibling path. The old database remains untouched
// until FinalizeUpgrade succeeds.
func OpenForUpdate(root, indexDir string) (*Store, error) {
	return open(root, indexDir, true)
}

func open(root, indexDir string, allowUpgrade bool) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("absolute root: %w", err)
	}
	if indexDir == "" {
		indexDir = ".focalspan"
	}
	if filepath.IsAbs(indexDir) {
		return nil, errors.New("index directory must be relative to repository root")
	}
	dir := filepath.Join(root, filepath.Clean(indexDir))
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return nil, errors.New("index directory escapes repository root")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}
	dbPath := filepath.Join(dir, "index.db")
	if allowUpgrade {
		if err := recoverUpgradeArtifacts(dbPath); err != nil {
			return nil, err
		}
	}
	exists := false
	if _, statErr := os.Stat(dbPath); statErr == nil {
		exists = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("stat index database: %w", statErr)
	}
	if allowUpgrade && exists {
		version, versionErr := readSchemaVersionPath(dbPath)
		if versionErr != nil && !errors.Is(versionErr, sql.ErrNoRows) {
			return nil, versionErr
		}
		if version == "1" {
			tempPath := dbPath + ".v2.tmp"
			if err := removeSQLiteFiles(tempPath); err != nil {
				return nil, fmt.Errorf("remove stale schema v2 temporary database: %w", err)
			}
			s, err := openDatabase(root, tempPath, false)
			if err != nil {
				return nil, err
			}
			s.liveDBPath = dbPath
			s.upgradePending = true
			return s, nil
		}
		if version != schemaVersion {
			return nil, unsupportedSchemaVersionError(dbPath, version)
		}
	}
	if !allowUpgrade && exists {
		version, versionErr := readSchemaVersionPath(dbPath)
		if versionErr != nil && !errors.Is(versionErr, sql.ErrNoRows) {
			return nil, versionErr
		}
		if version == "1" {
			return nil, &SchemaUpgradeRequiredError{Path: dbPath, Version: version}
		}
		if version != schemaVersion {
			return nil, unsupportedSchemaVersionError(dbPath, version)
		}
	}
	return openDatabase(root, dbPath, exists)
}

func unsupportedSchemaVersionError(path, version string) error {
	if version == "" {
		version = "<missing>"
	}
	return fmt.Errorf("unsupported schema version %q; remove %s, then run focalspan update --rebuild", version, path)
}

func openDatabase(root, dbPath string, existing bool) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	s := &Store{db: db, root: root, dbPath: dbPath, liveDBPath: dbPath}
	if err := s.configureAndMigrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func readSchemaVersionPath(path string) (string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", fmt.Errorf("open index database: %w", err)
	}
	defer db.Close()
	var version string
	err = db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func (s *Store) configureAndMigrate() error {
	for _, pragma := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=NORMAL"} {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite: %w", err)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	for _, migration := range []string{"migrations/001_initial.sql", "migrations/002_schema_v2.sql"} {
		sqlText, readErr := migrationFiles.ReadFile(migration)
		if readErr != nil {
			tx.Rollback()
			return fmt.Errorf("read migration %s: %w", migration, readErr)
		}
		if _, execErr := tx.Exec(string(sqlText)); execErr != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", migration, execErr)
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('schema_version', ?) ON CONFLICT(key) DO NOTHING`, schemaVersion); err != nil {
		tx.Rollback()
		return fmt.Errorf("set schema version: %w", err)
	}
	var version string
	if err := tx.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		tx.Rollback()
		return fmt.Errorf("read schema version: %w", err)
	}
	if version != schemaVersion {
		tx.Rollback()
		return unsupportedSchemaVersionError(s.dbPath, version)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	if s.upgradePending {
		_ = removeSQLiteFiles(s.dbPath)
		s.upgradePending = false
	}
	return err
}

func (s *Store) DBPath() string { return s.dbPath }

// FinalizeUpgrade atomically replaces the legacy database after the temporary
// v2 database has been fully indexed and integrity-checked.
func (s *Store) FinalizeUpgrade(ctx context.Context) error {
	if s == nil || !s.upgradePending {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.verifyIntegrity(); err != nil {
		return err
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close temporary schema v2 database: %w", err)
	}
	tempPath := s.dbPath
	livePath := s.liveDBPath
	backupPath := livePath + ".v1.bak"
	if err := removeSQLiteFiles(backupPath); err != nil {
		return fmt.Errorf("remove stale schema v1 backup: %w", err)
	}
	if err := renameSQLiteFiles(livePath, backupPath); err != nil {
		return fmt.Errorf("stage legacy database for replacement: %w", err)
	}
	if err := renameSQLiteFiles(tempPath, livePath); err != nil {
		_ = renameSQLiteFiles(backupPath, livePath)
		return fmt.Errorf("replace index database: %w", err)
	}
	if err := removeSQLiteFiles(backupPath); err != nil {
		return fmt.Errorf("remove legacy database backup: %w", err)
	}
	db, err := sql.Open("sqlite", livePath)
	if err != nil {
		return fmt.Errorf("reopen schema v2 database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	s.db = db
	s.dbPath = livePath
	s.liveDBPath = livePath
	s.upgradePending = false
	if err := s.configureAndMigrate(); err != nil {
		_ = db.Close()
		return err
	}
	return nil
}

func (s *Store) verifyIntegrity() error {
	var result string
	if err := s.db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("schema v2 integrity check: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		return fmt.Errorf("schema v2 integrity check failed: %s", result)
	}
	version, err := s.Meta(context.Background(), "schema_version")
	if err != nil {
		return fmt.Errorf("verify schema version: %w", err)
	}
	if version != schemaVersion {
		return fmt.Errorf("temporary database has schema version %q, want %q", version, schemaVersion)
	}
	return nil
}

func recoverUpgradeArtifacts(livePath string) error {
	backupPath := livePath + ".v1.bak"
	tempPath := livePath + ".v2.tmp"
	liveExists := fileExists(livePath)
	backupExists := fileExists(backupPath)
	tempExists := fileExists(tempPath)
	if !liveExists && backupExists {
		if err := os.Rename(backupPath, livePath); err != nil {
			return fmt.Errorf("recover legacy index database: %w", err)
		}
		liveExists = true
	}
	if backupExists && liveExists {
		if err := removeSQLiteFiles(backupPath); err != nil {
			return fmt.Errorf("remove recovered legacy backup: %w", err)
		}
	}
	if tempExists && liveExists {
		if err := removeSQLiteFiles(tempPath); err != nil {
			return fmt.Errorf("remove interrupted schema v2 database: %w", err)
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func removeSQLiteFiles(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func renameSQLiteFiles(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(from+suffix, to+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) ReplaceFile(ctx context.Context, file model.SourceFile, extraction model.Extraction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace file: %w", err)
	}
	if err := s.replaceFileTx(ctx, tx, file, extraction); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace file: %w", err)
	}
	return nil
}

func (s *Store) replaceFileTx(ctx context.Context, tx *sql.Tx, file model.SourceFile, extraction model.Extraction) error {
	var oldID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, file.Path).Scan(&oldID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find file: %w", err)
	}
	if err == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_fts WHERE handle IN (SELECT handle FROM chunks WHERE file_id = ?)`, oldID); err != nil {
			return fmt.Errorf("delete FTS rows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM relations WHERE from_handle IN (SELECT handle FROM symbols WHERE file_id = ?) OR to_handle IN (SELECT handle FROM symbols WHERE file_id = ?)`, oldID, oldID); err != nil {
			return fmt.Errorf("delete relations: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM diagnostics WHERE file_id = ?`, oldID); err != nil {
			return fmt.Errorf("delete diagnostics: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE file_id = ?`, oldID); err != nil {
			return fmt.Errorf("delete chunks: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE file_id = ?`, oldID); err != nil {
			return fmt.Errorf("delete symbols: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, oldID); err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `INSERT INTO files(path, language, sha256, size_bytes, mtime_ns, extractor, indexed_at, diagnostics_count) VALUES(?, ?, ?, ?, 0, ?, ?, ?)`, file.Path, file.Language, file.SHA256, file.SizeBytes, "", now, len(extraction.Diagnostics))
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	fileID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("file id: %w", err)
	}
	for _, symbol := range extraction.Symbols {
		if _, err := tx.ExecContext(ctx, `INSERT INTO symbols(handle, file_id, kind, name, qualified_name, signature, start_line, end_line, start_byte, end_byte, parent_handle, confidence) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, symbol.Handle, fileID, symbol.Kind, symbol.Name, symbol.QualifiedName, symbol.Signature, symbol.StartLine, symbol.EndLine, symbol.StartByte, symbol.EndByte, nullable(symbol.ParentHandle), symbol.Confidence); err != nil {
			return fmt.Errorf("insert symbol: %w", err)
		}
		for keyKind, key := range symbolLookupKeys(symbol) {
			if key == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO symbol_lookup(lookup_key, key_kind, handle) VALUES(?, ?, ?)`, key, keyKind, symbol.Handle); err != nil {
				return fmt.Errorf("insert symbol lookup: %w", err)
			}
		}
	}
	for keyKind, key := range fileLookupKeys(file.Path) {
		if key == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO file_lookup(lookup_key, key_kind, file_id) VALUES(?, ?, ?)`, key, keyKind, fileID); err != nil {
			return fmt.Errorf("insert file lookup: %w", err)
		}
	}
	for _, chunk := range extraction.Chunks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chunks(handle, file_id, symbol_handle, kind, symbol_name, signature, start_line, end_line, start_byte, end_byte, content, content_hash, estimated_tokens) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, chunk.Handle, fileID, nullable(chunk.SymbolHandle), chunk.Kind, chunk.SymbolName, chunk.Signature, chunk.StartLine, chunk.EndLine, chunk.StartByte, chunk.EndByte, chunk.Content, chunk.ContentHash, chunk.EstimatedTokens); err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO chunk_fts(path, symbol_name, signature, content, handle) VALUES(?, ?, ?, ?, ?)`, file.Path, chunk.SymbolName, chunk.Signature, chunk.Content, chunk.Handle); err != nil {
			return fmt.Errorf("insert FTS row: %w", err)
		}
	}
	for _, relation := range extraction.Relations {
		result, err := tx.ExecContext(ctx, `INSERT INTO relations(from_handle, to_handle, unresolved_to, kind, confidence, source) VALUES(?, ?, ?, ?, ?, ?)`, relation.FromHandle, nullable(relation.ToHandle), nullable(relation.UnresolvedTo), relation.Kind, relation.Confidence, relation.Source)
		if err != nil {
			return fmt.Errorf("insert relation: %w", err)
		}
		if relation.UnresolvedTo != "" {
			relationID, idErr := result.LastInsertId()
			if idErr != nil {
				return fmt.Errorf("relation id: %w", idErr)
			}
			for keyKind, key := range relationLookupKeys(relation.UnresolvedTo) {
				if key == "" {
					continue
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO relation_lookup(relation_id, lookup_key, key_kind, origin_path) VALUES(?, ?, ?, ?)`, relationID, key, keyKind, file.Path); err != nil {
					return fmt.Errorf("insert relation lookup: %w", err)
				}
			}
		}
	}
	for _, diagnostic := range extraction.Diagnostics {
		if _, err := tx.ExecContext(ctx, `INSERT INTO diagnostics(file_id, level, code, message, created_at) VALUES(?, ?, ?, ?, ?)`, fileID, diagnostic.Level, diagnostic.Code, diagnostic.Message, now); err != nil {
			return fmt.Errorf("insert diagnostic: %w", err)
		}
	}
	return nil
}

func (s *Store) DeleteFile(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := s.deleteFileTx(ctx, tx, path); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) deleteFileTx(ctx context.Context, tx *sql.Tx, path string) error {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, path).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	statements := []string{
		`DELETE FROM chunk_fts WHERE handle IN (SELECT handle FROM chunks WHERE file_id = ?)`,
		`DELETE FROM relations WHERE from_handle IN (SELECT handle FROM symbols WHERE file_id = ?) OR to_handle IN (SELECT handle FROM symbols WHERE file_id = ?)`,
		`DELETE FROM files WHERE id = ?`,
	}
	for i, statement := range statements {
		args := []any{id}
		if i == 1 {
			args = []any{id, id}
		}
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ApplyIndex(ctx context.Context, deletions []string, updates []FileUpdate, metadata []MetaUpdate, run model.IndexRun) error {
	_, err := s.ApplyIndexWithDelta(ctx, deletions, updates, metadata, run)
	return err
}

func (s *Store) ApplyIndexWithDelta(ctx context.Context, deletions []string, updates []FileUpdate, metadata []MetaUpdate, run model.IndexRun) (IndexDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IndexDelta{}, fmt.Errorf("begin index update: %w", err)
	}
	delta := IndexDelta{ChangedPaths: append([]string(nil), deletions...)}
	for _, update := range updates {
		delta.ChangedPaths = append(delta.ChangedPaths, update.File.Path)
	}
	keySet := make(map[string]bool)
	for _, path := range deletions {
		keys, keyErr := lookupKeysForPathTx(ctx, tx, path)
		if keyErr != nil {
			_ = tx.Rollback()
			return IndexDelta{}, fmt.Errorf("read lookup keys for deleted file %s: %w", path, keyErr)
		}
		for _, key := range keys {
			keySet[key] = true
		}
	}
	for _, update := range updates {
		keys, keyErr := lookupKeysForPathTx(ctx, tx, update.File.Path)
		if keyErr != nil {
			_ = tx.Rollback()
			return IndexDelta{}, fmt.Errorf("read lookup keys for updated file %s: %w", update.File.Path, keyErr)
		}
		for _, key := range keys {
			keySet[key] = true
		}
		for _, key := range extractionLookupKeys(update.File, update.Extraction) {
			keySet[key] = true
		}
	}
	sort.Strings(delta.ChangedPaths)
	delta.ChangedPaths = uniqueStrings(delta.ChangedPaths)
	for key := range keySet {
		delta.LookupKeys = append(delta.LookupKeys, key)
	}
	sort.Strings(delta.LookupKeys)
	rollback := func(e error) (IndexDelta, error) { _ = tx.Rollback(); return IndexDelta{}, e }
	for _, path := range deletions {
		if err := s.deleteFileTx(ctx, tx, path); err != nil {
			return rollback(fmt.Errorf("delete stale file %s: %w", path, err))
		}
	}
	for _, update := range updates {
		if err := s.replaceFileTx(ctx, tx, update.File, update.Extraction); err != nil {
			return rollback(fmt.Errorf("store %s: %w", update.File.Path, err))
		}
	}
	for _, update := range metadata {
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, update.Key, update.Value); err != nil {
			return rollback(fmt.Errorf("set metadata %s: %w", update.Key, err))
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO index_runs(started_at, completed_at, files_seen, files_added, files_changed, files_unchanged, files_deleted, parse_failures, duration_ms, revision) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.StartedAt, run.CompletedAt, run.FilesSeen, run.FilesAdded, run.FilesChanged, run.FilesUnchanged, run.FilesDeleted, run.ParseFailures, run.DurationMS, run.Revision); err != nil {
		return rollback(fmt.Errorf("record index run: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return IndexDelta{}, fmt.Errorf("commit index update: %w", err)
	}
	return delta, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func normalizeLookupKey(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
}

func relationLookupKeys(value string) map[string]string {
	raw := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	normalized := strings.ToLower(raw)
	if normalized == "" {
		return nil
	}
	result := map[string]string{"target": normalized}
	if strings.Contains(normalized, ".") {
		moduleRaw := strings.TrimLeft(raw, ".")
		parts := strings.Split(moduleRaw, ".")
		if len(parts) > 1 {
			last := parts[len(parts)-1]
			if last != "" && last[0] >= 'A' && last[0] <= 'Z' {
				parts = parts[:len(parts)-1]
			}
		}
		result["module"] = strings.ToLower(strings.ReplaceAll(strings.Join(parts, "."), ".", "/"))
	}
	if strings.Contains(normalized, "::") {
		parts := strings.Split(normalized, "::")
		if len(parts) > 1 {
			parts = parts[:len(parts)-1]
			if len(parts) > 0 && (parts[0] == "crate" || parts[0] == "self" || parts[0] == "super") {
				parts = parts[1:]
			}
			if len(parts) > 0 {
				result["rust_module"] = strings.Join(parts, "/")
			}
		}
	}
	return result
}

func symbolLookupKeys(symbol model.Symbol) map[string]string {
	result := make(map[string]string, 2)
	if key := normalizeLookupKey(symbol.Name); key != "" {
		result["name"] = key
	}
	if key := normalizeLookupKey(symbol.QualifiedName); key != "" {
		result["qualified"] = key
	}
	return result
}

func fileLookupKeys(path string) map[string]string {
	normalized := normalizeLookupKey(path)
	if normalized == "" {
		return nil
	}
	result := map[string]string{"path": normalized}
	if extension := filepath.Ext(normalized); extension != "" {
		result["extensionless"] = strings.TrimSuffix(normalized, extension)
	}
	if extensionless := strings.TrimSuffix(normalized, filepath.Ext(normalized)); extensionless != "" {
		result["module"] = strings.ReplaceAll(extensionless, "/", ".")
	}
	if base := filepath.Base(normalized); base != "." && base != "" {
		result["basename"] = base
	}
	parts := strings.Split(strings.Trim(normalized, "/"), "/")
	for start := 1; start < len(parts); start++ {
		if suffix := strings.Join(parts[start:], "/"); suffix != "" {
			result["suffix:"+suffix] = suffix
		}
	}
	if extensionless := strings.TrimSuffix(normalized, filepath.Ext(normalized)); extensionless != normalized {
		extensionlessParts := strings.Split(strings.Trim(extensionless, "/"), "/")
		for start := 1; start < len(extensionlessParts); start++ {
			if suffix := strings.Join(extensionlessParts[start:], "/"); suffix != "" {
				result["suffix:"+suffix] = suffix
			}
		}
	}
	for start := 0; start < len(parts)-1; start++ {
		if directory := strings.Join(parts[start:len(parts)-1], "/"); directory != "" {
			result["directory:"+directory] = directory
		}
	}
	return result
}

func extractionLookupKeys(file model.SourceFile, extraction model.Extraction) []string {
	set := make(map[string]bool)
	for _, key := range fileLookupKeys(file.Path) {
		if key != "" {
			set[key] = true
		}
	}
	for _, symbol := range extraction.Symbols {
		for _, key := range symbolLookupKeys(symbol) {
			if key != "" {
				set[key] = true
			}
		}
	}
	for _, relation := range extraction.Relations {
		for _, key := range relationLookupKeys(relation.UnresolvedTo) {
			if key != "" {
				set[key] = true
			}
		}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func lookupKeysForPathTx(ctx context.Context, tx *sql.Tx, path string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT lookup_key FROM file_lookup WHERE file_id = (SELECT id FROM files WHERE path = ?) UNION SELECT lookup_key FROM symbol_lookup WHERE handle IN (SELECT handle FROM symbols WHERE file_id = (SELECT id FROM files WHERE path = ?))`, path, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	return result, rows.Err()
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (s *Store) SearchFTS(ctx context.Context, query string, limit int) ([]model.RankedCandidate, error) {
	limit = retrievalLimit(limit, 100, 500)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature, c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash, bm25(chunk_fts) FROM chunk_fts JOIN chunks c ON c.handle = chunk_fts.handle JOIN files f ON f.id = c.file_id WHERE chunk_fts MATCH ? ORDER BY bm25(chunk_fts), f.path, c.start_line, c.handle LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("FTS search: %w", err)
	}
	defer rows.Close()
	results := make([]model.RankedCandidate, 0)
	for rows.Next() {
		var candidate model.RankedCandidate
		var bm25 float64
		if err := rows.Scan(&candidate.Handle, &candidate.Path, &candidate.Language, &candidate.Kind, &candidate.Symbol, &candidate.Signature, &candidate.StartLine, &candidate.EndLine, &candidate.StartByte, &candidate.EndByte, &candidate.Content, &candidate.ContentHash, &bm25); err != nil {
			return nil, err
		}
		candidate.Score = -bm25
		results = append(results, candidate)
	}
	return results, rows.Err()
}

func retrievalLimit(value, fallback, maximum int) int {
	if fallback <= 0 {
		fallback = 1
	}
	if maximum < fallback {
		maximum = fallback
	}
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

const rankedCandidateProjection = `SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature, c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash, COALESCE(symbol.confidence, 0) FROM chunks c JOIN files f ON f.id = c.file_id LEFT JOIN symbols symbol ON symbol.handle = c.symbol_handle WHERE `

func (s *Store) SearchExactSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	values = lookupValues(values)
	if len(values) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := s.db.QueryContext(ctx, rankedCandidateProjection+`lower(symbol.name) = lower(?) ORDER BY CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, value)
		if err != nil {
			return nil, fmt.Errorf("exact symbol search: %w", err)
		}
		items, scanErr := scanRankedCandidates(rows)
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		appendCandidates(&result, seen, items, limit)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *Store) SearchQualifiedSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	values = lookupValues(values)
	if len(values) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		queries := []struct {
			condition string
			args      []any
		}{
			{condition: `symbol.qualified_name = ?`, args: []any{value}},
			{condition: `lower(symbol.qualified_name) = lower(?)`, args: []any{value}},
		}
		for queryIndex, candidateQuery := range queries {
			rows, err := s.db.QueryContext(ctx, rankedCandidateProjection+candidateQuery.condition+` ORDER BY CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, candidateQuery.args...)
			if err != nil {
				return nil, fmt.Errorf("qualified symbol search: %w", err)
			}
			items, scanErr := scanRankedCandidates(rows)
			rows.Close()
			if scanErr != nil {
				return nil, scanErr
			}
			before := len(result)
			appendCandidates(&result, seen, items, limit)
			if len(result) >= limit || len(result) > before || queryIndex == len(queries)-1 {
				break
			}
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *Store) SearchSymbolPrefixes(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	values = lookupValues(values)
	if len(values) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped := escapeLike(strings.ToLower(value))
		rows, err := s.db.QueryContext(ctx, rankedCandidateProjection+`(lower(symbol.name) LIKE ? || '%' ESCAPE '\' OR lower(symbol.qualified_name) LIKE ? || '%' ESCAPE '\') ORDER BY CASE WHEN lower(symbol.name) = lower(?) THEN 0 ELSE 1 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, escaped, escaped, value)
		if err != nil {
			return nil, fmt.Errorf("symbol prefix search: %w", err)
		}
		items, scanErr := scanRankedCandidates(rows)
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		appendCandidates(&result, seen, items, limit)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *Store) SearchPaths(ctx context.Context, hints []string, limit int) ([]model.RankedCandidate, error) {
	hints = lookupValues(hints)
	if len(hints) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, hint := range hints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hint = strings.ReplaceAll(hint, "\\", "/")
		escaped := escapeLike(strings.ToLower(hint))
		rows, err := s.db.QueryContext(ctx, rankedCandidateProjection+`(lower(f.path) = ? OR lower(f.path) LIKE '%' || '/' || ? ESCAPE '\' OR lower(f.path) LIKE ? || '/%' ESCAPE '\' OR lower(f.path) LIKE '%' || ? || '%' ESCAPE '\') ORDER BY CASE WHEN lower(f.path) = ? THEN 0 WHEN lower(f.path) LIKE '%' || '/' || ? ESCAPE '\' THEN 1 WHEN lower(f.path) LIKE ? || '/%' ESCAPE '\' THEN 2 ELSE 3 END, COALESCE(symbol.confidence, 0) DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, escaped, escaped, escaped, escaped, escaped, escaped, escaped)
		if err != nil {
			return nil, fmt.Errorf("path search: %w", err)
		}
		items, scanErr := scanRankedCandidates(rows)
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		appendCandidates(&result, seen, items, limit)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func lookupValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func appendCandidates(result *[]model.RankedCandidate, seen map[string]bool, items []model.RankedCandidate, limit int) {
	for _, item := range items {
		key := item.Handle
		if key == "" {
			key = fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%s", item.Path, item.StartByte, item.EndByte, item.Kind, item.Symbol)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		*result = append(*result, item)
		if len(*result) >= limit {
			return
		}
	}
}

func (s *Store) AllCandidates(ctx context.Context, limit int) ([]model.RankedCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, candidateQuery(`1=1`)+` LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCandidates(rows)
}

func (s *Store) RelatedCandidateHits(ctx context.Context, handles []string, relation string) ([]model.RelationHit, error) {
	if len(handles) == 0 {
		return []model.RelationHit{}, nil
	}
	if relation == "" {
		relation = "neighbors"
	}
	if !supportedRelation(relation) {
		return []model.RelationHit{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RelationHit, 0)
	for _, handle := range handles {
		camelPattern := ""
		if relation == "callers" || relation == "tests" {
			var err error
			camelPattern, err = s.camelCaseRelationPattern(ctx, handle)
			if err != nil {
				return nil, err
			}
		}
		query, args := relatedHitQuery(handle, relation, camelPattern)
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		hits, scanErr := scanRelationHits(rows, handle, relation)
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, hits...)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Context.Kind != right.Context.Kind {
			return left.Context.Kind < right.Context.Kind
		}
		if left.Context.Confidence != right.Context.Confidence {
			return left.Context.Confidence > right.Context.Confidence
		}
		if left.Candidate.Path != right.Candidate.Path {
			return left.Candidate.Path < right.Candidate.Path
		}
		if left.Candidate.StartLine != right.Candidate.StartLine {
			return left.Candidate.StartLine < right.Candidate.StartLine
		}
		if left.Candidate.Handle != right.Candidate.Handle {
			return left.Candidate.Handle < right.Candidate.Handle
		}
		if left.Context.AnchorHandle != right.Context.AnchorHandle {
			return left.Context.AnchorHandle < right.Context.AnchorHandle
		}
		if left.Context.Direction != right.Context.Direction {
			return left.Context.Direction < right.Context.Direction
		}
		if left.Context.Resolved != right.Context.Resolved {
			return left.Context.Resolved
		}
		return left.Context.Source < right.Context.Source
	})

	seen := make(map[string]bool, len(result))
	deduped := result[:0]
	for _, hit := range result {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%t", hit.Candidate.Handle, hit.Context.AnchorHandle, hit.Context.Kind, hit.Context.Direction, hit.Context.Source, hit.Context.Resolved)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, hit)
	}
	return deduped, nil
}

func (s *Store) RelatedCandidates(ctx context.Context, handles []string, relation string) ([]model.RankedCandidate, error) {
	return s.relatedCandidatesLegacy(ctx, handles, relation)
}

func supportedRelation(relation string) bool {
	switch relation {
	case "self", "parent", "children", "callers", "callees", "imports", "exports", "references", "tests", "neighbors":
		return true
	default:
		return false
	}
}

func relatedHitQuery(handle, relation, camelPattern string) (string, []any) {
	anchor := `WITH anchor AS (
SELECT ? AS requested_handle, s.handle, s.name, s.qualified_name, f.path
FROM symbols s JOIN files f ON f.id = s.file_id
WHERE s.handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
)`
	if relation == "self" {
		return anchor + `
SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature,
       c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash,
       1.0, 'self', 1, 'related'
FROM anchor a JOIN chunks c ON c.symbol_handle = a.handle JOIN files f ON f.id = c.file_id
ORDER BY f.path, c.start_line, c.handle`, []any{handle, handle, handle}
	}

	var matches string
	args := []any{handle, handle, handle}
	switch relation {
	case "children":
		matches = `SELECT r.to_handle AS candidate_handle, r.confidence, r.source, 1 AS resolved, 'outgoing' AS direction FROM relations r, anchor a WHERE r.from_handle = a.handle AND r.kind = 'contains' AND r.to_handle IS NOT NULL`
	case "parent":
		matches = `SELECT r.from_handle AS candidate_handle, r.confidence, r.source, 1 AS resolved, 'incoming' AS direction FROM relations r, anchor a WHERE r.to_handle = a.handle AND r.kind = 'contains'`
	case "callers":
		matches = incomingMatches("calls", callAnchorMatch("a", "r", true))
		args = append(args, camelPattern)
	case "callees":
		matches = outgoingMatches("calls", callTargetMatch("target", "r"))
	case "tests":
		matches = incomingMatches("tests", callAnchorMatch("a", "r", true)) + " UNION ALL " + outgoingMatches("tests", callTargetMatch("target", "r"))
		args = append(args, camelPattern)
	case "imports", "exports", "references":
		matches = incomingMatches(relation, importAnchorMatch("a", "r")) + " UNION ALL " + outgoingMatches(relation, importTargetMatch("target", "target_file", "r"))
	case "neighbors":
		matches = `SELECT r.from_handle AS candidate_handle, r.confidence, r.source, 1 AS resolved, 'incoming' AS direction FROM relations r, anchor a WHERE r.to_handle = a.handle
UNION ALL
SELECT r.to_handle AS candidate_handle, r.confidence, r.source, 1 AS resolved, 'outgoing' AS direction FROM relations r, anchor a WHERE r.from_handle = a.handle AND r.to_handle IS NOT NULL`
	}
	filter := ""
	if relation == "imports" || relation == "exports" || relation == "references" {
		filter = ` AND (c.kind = 'template-outline' OR NOT EXISTS (SELECT 1 FROM chunks outline WHERE outline.symbol_handle = c.symbol_handle AND outline.kind = 'template-outline'))`
	}
	query := anchor + `,
matches AS (` + matches + `)
SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature,
       c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash,
       m.confidence, m.source, m.resolved, m.direction
FROM matches m JOIN chunks c ON c.symbol_handle = m.candidate_handle JOIN files f ON f.id = c.file_id
WHERE m.candidate_handle IS NOT NULL` + filter + `
ORDER BY m.confidence DESC, f.path, c.start_line, c.handle, m.direction, m.source, m.resolved DESC`
	return query, args
}

func incomingMatches(kind, unresolvedMatch string) string {
	return fmt.Sprintf(`SELECT r.from_handle AS candidate_handle, r.confidence, r.source,
CASE WHEN r.to_handle = a.handle THEN 1 ELSE 0 END AS resolved, 'incoming' AS direction
FROM relations r, anchor a
WHERE r.kind = '%s' AND (r.to_handle = a.handle OR (r.to_handle IS NULL AND (%s)))`, kind, unresolvedMatch)
}

func outgoingMatches(kind, unresolvedMatch string) string {
	return fmt.Sprintf(`SELECT r.to_handle AS candidate_handle, r.confidence, r.source, 1 AS resolved, 'outgoing' AS direction
FROM relations r, anchor a
WHERE r.kind = '%s' AND r.from_handle = a.handle AND r.to_handle IS NOT NULL
UNION ALL
SELECT target.handle AS candidate_handle, r.confidence, r.source, 0 AS resolved, 'outgoing' AS direction
FROM relations r, anchor a JOIN symbols target JOIN files target_file ON target_file.id = target.file_id
WHERE r.kind = '%s' AND r.from_handle = a.handle AND r.to_handle IS NULL AND (%s)`, kind, kind, unresolvedMatch)
}

func callAnchorMatch(anchor, relation string, camel bool) string {
	match := fmt.Sprintf(`lower(%[2]s.unresolved_to) = lower(%[1]s.name)
OR lower(%[2]s.unresolved_to) = lower(%[1]s.qualified_name)
OR lower(%[2]s.unresolved_to) LIKE '%%.' || lower(%[1]s.name)
OR lower(%[2]s.unresolved_to) LIKE '%%::' || lower(%[1]s.name)
OR lower(%[2]s.unresolved_to) LIKE '%%->' || lower(%[1]s.name)`, anchor, relation)
	if camel {
		match += fmt.Sprintf(` OR lower(%s.unresolved_to) LIKE ?`, relation)
	}
	return match
}

func callTargetMatch(target, relation string) string {
	return callAnchorMatch(target, relation, false)
}

func importAnchorMatch(anchor, relation string) string {
	return fmt.Sprintf(`lower(%[2]s.unresolved_to) = lower(%[1]s.name)
OR lower(%[2]s.unresolved_to) = lower(%[1]s.qualified_name)
OR lower(%[2]s.unresolved_to) = lower(%[1]s.path)
OR lower(%[1]s.path) LIKE '%%/' || lower(%[2]s.unresolved_to)
OR lower(%[2]s.unresolved_to) LIKE '%%::' || lower(%[1]s.name) || '::%%'
OR lower(%[2]s.unresolved_to) LIKE '%%.' || lower(%[1]s.name) || '.%%'
OR lower(%[2]s.unresolved_to) LIKE '%%.' || lower(%[1]s.name)`, anchor, relation)
}

func importTargetMatch(target, targetFile, relation string) string {
	return fmt.Sprintf(`lower(%[3]s.unresolved_to) = lower(%[1]s.name)
OR lower(%[3]s.unresolved_to) = lower(%[1]s.qualified_name)
OR lower(%[3]s.unresolved_to) = lower(%[2]s.path)
OR lower(%[2]s.path) LIKE '%%/' || lower(%[3]s.unresolved_to)
OR lower(%[3]s.unresolved_to) LIKE '%%::' || lower(%[1]s.name) || '::%%'
OR lower(%[3]s.unresolved_to) LIKE '%%.' || lower(%[1]s.name) || '.%%'
OR lower(%[3]s.unresolved_to) LIKE '%%.' || lower(%[1]s.name)`, target, targetFile, relation)
}

func scanRelationHits(rows *sql.Rows, anchorHandle, relation string) ([]model.RelationHit, error) {
	result := make([]model.RelationHit, 0)
	for rows.Next() {
		var candidate model.RankedCandidate
		var source, direction string
		var confidence float64
		var resolved int
		if err := rows.Scan(&candidate.Handle, &candidate.Path, &candidate.Language, &candidate.Kind, &candidate.Symbol, &candidate.Signature, &candidate.StartLine, &candidate.EndLine, &candidate.StartByte, &candidate.EndByte, &candidate.Content, &candidate.ContentHash, &confidence, &source, &resolved, &direction); err != nil {
			return nil, err
		}
		context := model.RelationContext{AnchorHandle: anchorHandle, Kind: relation, Direction: model.RelationDirection(direction), Confidence: confidence, Source: source, Resolved: resolved != 0}
		result = append(result, model.RelationHit{Candidate: candidate, Context: context})
	}
	return result, rows.Err()
}

func (s *Store) relatedCandidatesLegacy(ctx context.Context, handles []string, relation string) ([]model.RankedCandidate, error) {
	if len(handles) == 0 {
		return []model.RankedCandidate{}, nil
	}
	if relation == "" {
		relation = "neighbors"
	}
	allowed := map[string]bool{"self": true, "parent": true, "children": true, "callers": true, "callees": true, "imports": true, "exports": true, "references": true, "tests": true, "neighbors": true}
	if !allowed[relation] {
		return []model.RankedCandidate{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	for _, handle := range handles {
		var query string
		var args []any
		camelCasePattern := ""
		if relation == "callers" || relation == "tests" {
			var patternErr error
			camelCasePattern, patternErr = s.camelCaseRelationPattern(ctx, handle)
			if patternErr != nil {
				return nil, patternErr
			}
		}
		switch relation {
		case "self":
			query = candidateQuery(`c.symbol_handle = ? OR c.handle = ?`)
			args = []any{handle, handle}
		case "children":
			query = candidateQuery(`c.symbol_handle IN (SELECT to_handle FROM relations WHERE from_handle = (SELECT COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) ) AND kind = 'contains')`)
			args = []any{handle, handle}
		case "parent":
			query = candidateQuery(`c.symbol_handle IN (SELECT from_handle FROM relations WHERE to_handle = (SELECT COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) ) AND kind = 'contains')`)
			args = []any{handle, handle}
		case "callers":
			query = candidateQuery(`c.symbol_handle IN (
SELECT r.from_handle FROM relations r
WHERE r.kind = 'calls' AND (
  r.to_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
  OR EXISTS (
    SELECT 1 FROM symbols target
    WHERE target.handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
      AND (
        lower(r.unresolved_to) = lower(target.name)
        OR lower(r.unresolved_to) = lower(target.qualified_name)
        OR lower(r.unresolved_to) LIKE '%.' || lower(target.name)
        OR lower(r.unresolved_to) LIKE '%::' || lower(target.name)
        OR lower(r.unresolved_to) LIKE '%->' || lower(target.name)
        OR lower(r.unresolved_to) LIKE ?
      )
  )
))`)
			args = []any{handle, handle, handle, handle, camelCasePattern}
		case "callees":
			query = candidateQuery(`c.symbol_handle IN (
SELECT r.to_handle FROM relations r
WHERE r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.kind = 'calls' AND r.to_handle IS NOT NULL
UNION SELECT target.handle FROM relations r
JOIN symbols target ON lower(r.unresolved_to) = lower(target.name) OR lower(r.unresolved_to) = lower(target.qualified_name)
WHERE r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.kind = 'calls' AND r.to_handle IS NULL
)`)
			args = []any{handle, handle, handle, handle}
		case "tests":
			query = candidateQuery(`c.symbol_handle IN (
SELECT r.from_handle FROM relations r
WHERE r.kind = 'tests' AND (
  r.to_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
  OR EXISTS (
    SELECT 1 FROM symbols target
    WHERE target.handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
      AND (
        lower(r.unresolved_to) = lower(target.name)
        OR lower(r.unresolved_to) = lower(target.qualified_name)
        OR lower(r.unresolved_to) LIKE '%.' || lower(target.name)
        OR lower(r.unresolved_to) LIKE '%::' || lower(target.name)
        OR lower(r.unresolved_to) LIKE '%->' || lower(target.name)
        OR lower(r.unresolved_to) LIKE ?
      )
  )
)
UNION SELECT r.to_handle FROM relations r
WHERE r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.kind = 'tests' AND r.to_handle IS NOT NULL
UNION SELECT target.handle FROM relations r
JOIN symbols target ON lower(r.unresolved_to) = lower(target.name) OR lower(r.unresolved_to) = lower(target.qualified_name)
WHERE r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.kind = 'tests' AND r.to_handle IS NULL
)`)
			args = []any{handle, handle, handle, handle, camelCasePattern, handle, handle, handle, handle}
		case "neighbors":
			query = candidateQuery(`c.symbol_handle IN (SELECT from_handle FROM relations WHERE to_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) OR from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) UNION SELECT to_handle FROM relations WHERE from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) OR to_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?))`)
			args = []any{handle, handle, handle, handle, handle, handle, handle, handle}
		case "imports", "exports", "references":
			query = candidateQuery(`c.symbol_handle IN (
SELECT r.from_handle FROM relations r
WHERE r.kind = ? AND (
  r.to_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
  OR EXISTS (
    SELECT 1 FROM symbols target JOIN files target_file ON target.file_id = target_file.id
    WHERE target.handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)
      AND (
        lower(r.unresolved_to) = lower(target.name)
        OR lower(r.unresolved_to) = lower(target.qualified_name)
        OR lower(r.unresolved_to) = lower(target_file.path)
        OR lower(target_file.path) LIKE '%/' || lower(r.unresolved_to)
        OR lower(r.unresolved_to) LIKE '%::' || lower(target.name) || '::%'
        OR lower(r.unresolved_to) LIKE '%.' || lower(target.name) || '.%'
        OR lower(r.unresolved_to) LIKE '%.' || lower(target.name)
      )
  )
)
UNION SELECT r.to_handle FROM relations r
WHERE r.kind = ? AND r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.to_handle IS NOT NULL
UNION SELECT target.handle FROM relations r
JOIN symbols target ON lower(r.unresolved_to) = lower(target.name) OR lower(r.unresolved_to) = lower(target.qualified_name)
JOIN files target_file ON target.file_id = target_file.id
WHERE r.kind = ? AND r.from_handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?) AND r.to_handle IS NULL
  AND (
    lower(r.unresolved_to) = lower(target.name)
    OR lower(r.unresolved_to) = lower(target.qualified_name)
    OR lower(r.unresolved_to) = lower(target_file.path)
    OR lower(target_file.path) LIKE '%/' || lower(r.unresolved_to)
    OR lower(r.unresolved_to) LIKE '%::' || lower(target.name) || '::%'
    OR lower(r.unresolved_to) LIKE '%.' || lower(target.name) || '.%'
    OR lower(r.unresolved_to) LIKE '%.' || lower(target.name)
  )
) AND (c.kind = 'template-outline' OR NOT EXISTS (
SELECT 1 FROM chunks outline
WHERE outline.symbol_handle = c.symbol_handle AND outline.kind = 'template-outline'
))`)
			args = []any{relation, handle, handle, handle, handle, relation, handle, handle, relation, handle, handle}
		}
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		items, err := scanCandidates(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}
	return result, nil
}

func (s *Store) camelCaseRelationPattern(ctx context.Context, handle string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM symbols WHERE handle = COALESCE((SELECT symbol_handle FROM chunks WHERE handle = ?), ?)`, handle, handle).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find relation target: %w", err)
	}
	return camelCaseLikePattern(name), nil
}

func camelCaseLikePattern(value string) string {
	parts := make([]string, 0, len(value))
	start := 0
	for index, r := range value {
		if index > start && r >= 'A' && r <= 'Z' {
			parts = append(parts, value[start:index])
			start = index
		}
	}
	if start < len(value) {
		parts = append(parts, value[start:])
	}
	if len(parts) < 2 {
		return ""
	}
	var pattern strings.Builder
	pattern.WriteByte('%')
	for _, part := range parts {
		pattern.WriteString(strings.ToLower(part))
		pattern.WriteByte('%')
	}
	return pattern.String()
}

func candidateQuery(condition string) string {
	return `SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature, c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash FROM chunks c JOIN files f ON f.id = c.file_id WHERE ` + condition + ` ORDER BY f.path, c.start_line, c.handle`
}

func scanCandidates(rows *sql.Rows) ([]model.RankedCandidate, error) {
	result := make([]model.RankedCandidate, 0)
	for rows.Next() {
		var candidate model.RankedCandidate
		if err := rows.Scan(&candidate.Handle, &candidate.Path, &candidate.Language, &candidate.Kind, &candidate.Symbol, &candidate.Signature, &candidate.StartLine, &candidate.EndLine, &candidate.StartByte, &candidate.EndByte, &candidate.Content, &candidate.ContentHash); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func scanRankedCandidates(rows *sql.Rows) ([]model.RankedCandidate, error) {
	result := make([]model.RankedCandidate, 0)
	for rows.Next() {
		var candidate model.RankedCandidate
		if err := rows.Scan(&candidate.Handle, &candidate.Path, &candidate.Language, &candidate.Kind, &candidate.Symbol, &candidate.Signature, &candidate.StartLine, &candidate.EndLine, &candidate.StartByte, &candidate.EndByte, &candidate.Content, &candidate.ContentHash, &candidate.Confidence); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (s *Store) FileHash(ctx context.Context, path string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT sha256 FROM files WHERE path = ?`, path).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return hash, err == nil, err
}

func (s *Store) Paths(ctx context.Context) (map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make(map[string]bool)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths[path] = true
	}
	return paths, rows.Err()
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) Meta(ctx context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (s *Store) RecordRun(ctx context.Context, run model.IndexRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO index_runs(started_at, completed_at, files_seen, files_added, files_changed, files_unchanged, files_deleted, parse_failures, duration_ms, revision) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.StartedAt, run.CompletedAt, run.FilesSeen, run.FilesAdded, run.FilesChanged, run.FilesUnchanged, run.FilesDeleted, run.ParseFailures, run.DurationMS, run.Revision)
	return err
}

func (s *Store) FinalizeIndexRun(ctx context.Context, run model.IndexRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE index_runs SET completed_at = ?, duration_ms = ? WHERE id = (SELECT id FROM index_runs ORDER BY id DESC LIMIT 1)`, run.CompletedAt, run.DurationMS)
	return err
}

func (s *Store) Status(ctx context.Context, root string) (model.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var status model.Status
	status.Root, status.DBPath = root, s.dbPath
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key='schema_version'`).Scan(&status.SchemaVersion); err != nil {
		return status, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT value FROM meta WHERE key='last_revision'), '')`).Scan(&status.LastRevision); err != nil {
		return status, err
	}
	queries := []struct {
		dest  *int
		query string
	}{{&status.FileCount, `SELECT COUNT(*) FROM files`}, {&status.SymbolCount, `SELECT COUNT(*) FROM symbols`}, {&status.ChunkCount, `SELECT COUNT(*) FROM chunks`}, {&status.RelationCount, `SELECT COUNT(*) FROM relations`}, {&status.DiagnosticCount, `SELECT COUNT(*) FROM diagnostics`}}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.query).Scan(item.dest); err != nil {
			return status, err
		}
	}
	var duration sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT duration_ms FROM index_runs ORDER BY id DESC LIMIT 1`).Scan(&duration); err == nil && duration.Valid {
		status.LastDurationMS = duration.Int64
	}
	return status, nil
}
