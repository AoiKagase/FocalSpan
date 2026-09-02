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

const schemaVersion = "1"

type Store struct {
	db     *sql.DB
	root   string
	dbPath string
	mu     sync.RWMutex
}

// queryCounterKey is used only by package tests to assert that batching
// reduces database round-trips. It is intentionally private and has no
// effect on normal callers.
type queryCounterKey struct{}

func withQueryCounter(ctx context.Context, counter *int) context.Context {
	return context.WithValue(ctx, queryCounterKey{}, counter)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if counter, ok := ctx.Value(queryCounterKey{}).(*int); ok && counter != nil {
		(*counter)++
	}
	return s.db.QueryContext(ctx, query, args...)
}

type FileUpdate struct {
	File       model.SourceFile
	Extraction model.Extraction
}

type MetaUpdate struct {
	Key   string
	Value string
}

func Open(root, indexDir string) (*Store, error) {
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
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	s := &Store{db: db, root: root, dbPath: dbPath}
	if err := s.configureAndMigrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
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
	sqlText, err := migrationFiles.ReadFile("migrations/001_initial.sql")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := tx.Exec(string(sqlText)); err != nil {
		tx.Rollback()
		return fmt.Errorf("apply migration: %w", err)
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
		return fmt.Errorf("unsupported schema version %q; remove %s, then run focalspan update --rebuild", version, s.dbPath)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DBPath() string { return s.dbPath }

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
		if _, err := tx.ExecContext(ctx, `INSERT INTO relations(from_handle, to_handle, unresolved_to, kind, confidence, source) VALUES(?, ?, ?, ?, ?, ?)`, relation.FromHandle, nullable(relation.ToHandle), nullable(relation.UnresolvedTo), relation.Kind, relation.Confidence, relation.Source); err != nil {
			return fmt.Errorf("insert relation: %w", err)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin index update: %w", err)
	}
	rollback := func(e error) error { _ = tx.Rollback(); return e }
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
		return fmt.Errorf("commit index update: %w", err)
	}
	return nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
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

const rankedCandidateColumns = `c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature, c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash, COALESCE(symbol.confidence, 0)`

const rankedCandidateFrom = `FROM chunks c JOIN files f ON f.id = c.file_id LEFT JOIN symbols symbol ON symbol.handle = c.symbol_handle`

const maxBatchParameters = 900
const maxRelationBatchHandles = 64

func batchFits(valueCount, parametersPerValue int) bool {
	return valueCount > 0 && valueCount <= maxBatchParameters/parametersPerValue
}

func ordinalValuesCTE(name string, columns []string, rows [][]any) (string, []any) {
	var query strings.Builder
	query.WriteString("WITH ")
	query.WriteString(name)
	query.WriteByte('(')
	query.WriteString(strings.Join(columns, ", "))
	query.WriteString(") AS (VALUES ")
	args := make([]any, 0, len(rows)*len(columns))
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			query.WriteString(", ")
		}
		query.WriteByte('(')
		for columnIndex, value := range row {
			if columnIndex > 0 {
				query.WriteString(", ")
			}
			query.WriteByte('?')
			args = append(args, value)
		}
		query.WriteByte(')')
	}
	query.WriteString(") ")
	return query.String(), args
}

func batchRankedQuery(ctx context.Context, s *Store, cte string, cteArgs []any, condition, order string) ([]model.RankedCandidate, error) {
	query := cte + `SELECT ` + rankedCandidateColumns + ` ` + rankedCandidateFrom + ` JOIN input ON ` + condition + ` ORDER BY ` + order
	rows, err := s.queryContext(ctx, query, cteArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRankedCandidates(rows)
}

type rankedCandidateOrdinal struct {
	Candidate model.RankedCandidate
	Ordinal   int
}

func batchRankedQueryWithOrdinal(ctx context.Context, s *Store, cte string, cteArgs []any, condition, order string) ([]rankedCandidateOrdinal, error) {
	query := cte + `SELECT ` + rankedCandidateColumns + `, input.ordinal ` + rankedCandidateFrom + ` JOIN input ON ` + condition + ` ORDER BY ` + order
	rows, err := s.queryContext(ctx, query, cteArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]rankedCandidateOrdinal, 0)
	for rows.Next() {
		var item rankedCandidateOrdinal
		if err := rows.Scan(&item.Candidate.Handle, &item.Candidate.Path, &item.Candidate.Language, &item.Candidate.Kind, &item.Candidate.Symbol, &item.Candidate.Signature, &item.Candidate.StartLine, &item.Candidate.EndLine, &item.Candidate.StartByte, &item.Candidate.EndByte, &item.Candidate.Content, &item.Candidate.ContentHash, &item.Candidate.Confidence, &item.Ordinal); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func groupRankedCandidates(items []rankedCandidateOrdinal) map[int][]model.RankedCandidate {
	groups := make(map[int][]model.RankedCandidate)
	for _, item := range items {
		groups[item.Ordinal] = append(groups[item.Ordinal], item.Candidate)
	}
	return groups
}

func (s *Store) SearchExactSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	values = lookupValues(values)
	if len(values) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	if !batchFits(len(values), 2) {
		return s.searchExactSymbolsSequential(ctx, values, limit)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	inputRows := make([][]any, 0, len(values))
	for index, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		inputRows = append(inputRows, []any{value, index})
	}
	cte, args := ordinalValuesCTE("input", []string{"value", "ordinal"}, inputRows)
	items, err := batchRankedQuery(ctx, s, cte, args,
		`lower(symbol.name) = lower(input.value)`,
		`input.ordinal, CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`)
	if err != nil {
		return nil, fmt.Errorf("exact symbol search: %w", err)
	}
	result := make([]model.RankedCandidate, 0, len(items))
	seen := make(map[string]bool)
	appendCandidates(&result, seen, items, limit)
	return result, nil
}

func (s *Store) SearchQualifiedSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	values = lookupValues(values)
	if len(values) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	if !batchFits(len(values), 2) {
		return s.searchQualifiedSymbolsSequential(ctx, values, limit)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	inputRows := make([][]any, 0, len(values))
	for index, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		inputRows = append(inputRows, []any{value, index})
	}
	order := `input.ordinal, CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`
	cte, args := ordinalValuesCTE("input", []string{"value", "ordinal"}, inputRows)
	exact, err := batchRankedQueryWithOrdinal(ctx, s, cte, args, `symbol.qualified_name = input.value`, order)
	if err != nil {
		return nil, fmt.Errorf("qualified symbol search: %w", err)
	}
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	exactGroups := groupRankedCandidates(exact)
	pending := make([][]any, 0)
	for index, value := range values {
		before := len(result)
		appendCandidates(&result, seen, exactGroups[index], limit)
		if len(result) >= limit {
			break
		}
		if len(result) == before {
			pending = append(pending, []any{value, len(pending)})
		}
	}
	if len(result) >= limit || len(pending) == 0 {
		return result, nil
	}
	lowerCTE, lowerArgs := ordinalValuesCTE("input", []string{"value", "ordinal"}, pending)
	lower, err := batchRankedQueryWithOrdinal(ctx, s, lowerCTE, lowerArgs, `lower(symbol.qualified_name) = lower(input.value)`, order)
	if err != nil {
		return nil, fmt.Errorf("qualified symbol search: %w", err)
	}
	lowerGroups := groupRankedCandidates(lower)
	for index := range pending {
		appendCandidates(&result, seen, lowerGroups[index], limit)
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
	if !batchFits(len(values), 3) {
		return s.searchSymbolPrefixesSequential(ctx, values, limit)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	inputRows := make([][]any, 0, len(values))
	for index, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped := escapeLike(strings.ToLower(value))
		inputRows = append(inputRows, []any{value, escaped, index})
	}
	cte, args := ordinalValuesCTE("input", []string{"value", "escaped", "ordinal"}, inputRows)
	items, err := batchRankedQuery(ctx, s, cte, args,
		`(lower(symbol.name) LIKE input.escaped || '%' ESCAPE '\' OR lower(symbol.qualified_name) LIKE input.escaped || '%' ESCAPE '\')`,
		`input.ordinal, CASE WHEN lower(symbol.name) = lower(input.value) THEN 0 ELSE 1 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`)
	if err != nil {
		return nil, fmt.Errorf("symbol prefix search: %w", err)
	}
	result := make([]model.RankedCandidate, 0, len(items))
	seen := make(map[string]bool)
	appendCandidates(&result, seen, items, limit)
	return result, nil
}

func (s *Store) SearchPaths(ctx context.Context, hints []string, limit int) ([]model.RankedCandidate, error) {
	hints = lookupValues(hints)
	if len(hints) == 0 {
		return []model.RankedCandidate{}, nil
	}
	limit = retrievalLimit(limit, 100, 500)
	if !batchFits(len(hints), 2) {
		return s.searchPathsSequential(ctx, hints, limit)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	inputRows := make([][]any, 0, len(hints))
	for index, hint := range hints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hint = strings.ReplaceAll(hint, "\\", "/")
		escaped := escapeLike(strings.ToLower(hint))
		inputRows = append(inputRows, []any{escaped, index})
	}
	cte, args := ordinalValuesCTE("input", []string{"escaped", "ordinal"}, inputRows)
	items, err := batchRankedQuery(ctx, s, cte, args,
		`(lower(f.path) = input.escaped OR lower(f.path) LIKE '%' || '/' || input.escaped ESCAPE '\' OR lower(f.path) LIKE input.escaped || '/%' ESCAPE '\' OR lower(f.path) LIKE '%' || input.escaped || '%' ESCAPE '\')`,
		`input.ordinal, CASE WHEN lower(f.path) = input.escaped THEN 0 WHEN lower(f.path) LIKE '%' || '/' || input.escaped ESCAPE '\' THEN 1 WHEN lower(f.path) LIKE input.escaped || '/%' ESCAPE '\' THEN 2 ELSE 3 END, COALESCE(symbol.confidence, 0) DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`)
	if err != nil {
		return nil, fmt.Errorf("path search: %w", err)
	}
	result := make([]model.RankedCandidate, 0, len(items))
	seen := make(map[string]bool)
	appendCandidates(&result, seen, items, limit)
	return result, nil
}

func (s *Store) searchExactSymbolsSequential(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := s.queryContext(ctx, rankedCandidateProjection+`lower(symbol.name) = lower(?) ORDER BY CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, value)
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

func (s *Store) searchQualifiedSymbolsSequential(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
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
			rows, err := s.queryContext(ctx, rankedCandidateProjection+candidateQuery.condition+` ORDER BY CASE WHEN c.kind LIKE '%-outline' OR c.kind = 'test-suite' THEN 1 ELSE 0 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, candidateQuery.args...)
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

func (s *Store) searchSymbolPrefixesSequential(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped := escapeLike(strings.ToLower(value))
		rows, err := s.queryContext(ctx, rankedCandidateProjection+`(lower(symbol.name) LIKE ? || '%' ESCAPE '\' OR lower(symbol.qualified_name) LIKE ? || '%' ESCAPE '\') ORDER BY CASE WHEN lower(symbol.name) = lower(?) THEN 0 ELSE 1 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, escaped, escaped, value)
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

func (s *Store) searchPathsSequential(ctx context.Context, hints []string, limit int) ([]model.RankedCandidate, error) {
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
		rows, err := s.queryContext(ctx, rankedCandidateProjection+`(lower(f.path) = ? OR lower(f.path) LIKE '%' || '/' || ? ESCAPE '\' OR lower(f.path) LIKE ? || '/%' ESCAPE '\' OR lower(f.path) LIKE '%' || ? || '%' ESCAPE '\') ORDER BY CASE WHEN lower(f.path) = ? THEN 0 WHEN lower(f.path) LIKE '%' || '/' || ? ESCAPE '\' THEN 1 WHEN lower(f.path) LIKE ? || '/%' ESCAPE '\' THEN 2 ELSE 3 END, COALESCE(symbol.confidence, 0) DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC`, escaped, escaped, escaped, escaped, escaped, escaped, escaped)
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
	if len(handles) > maxRelationBatchHandles {
		all := make([]model.RelationHit, 0)
		for start := 0; start < len(handles); start += maxRelationBatchHandles {
			end := start + maxRelationBatchHandles
			if end > len(handles) {
				end = len(handles)
			}
			part, err := s.RelatedCandidateHits(ctx, handles[start:end], relation)
			if err != nil {
				return nil, err
			}
			all = append(all, part...)
		}
		return normalizeRelationHits(all, relation), nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RelationHit, 0)
	queries := make([]string, 0, len(handles))
	queryArgs := make([][]any, 0, len(handles))
	camelPatterns := make([]string, len(handles))
	if relation == "callers" || relation == "tests" {
		var err error
		camelPatterns, err = s.camelCaseRelationPatterns(ctx, handles)
		if err != nil {
			return nil, err
		}
	}
	for index, handle := range handles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		camelPattern := camelPatterns[index]
		query, args := relatedHitQuery(handle, relation, camelPattern)
		queries = append(queries, query)
		queryArgs = append(queryArgs, args)
	}
	query, args := unionBatchQueriesWithAnchor(queries, queryArgs, handles)
	rows, err := s.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	hits, scanErr := scanRelationHits(rows, relation)
	rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	result = append(result, hits...)

	return normalizeRelationHits(result, relation), nil
}

func normalizeRelationHits(result []model.RelationHit, relation string) []model.RelationHit {
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
	return deduped
}

func unionBatchCandidateQueries(queries []string, args [][]any) (string, []any) {
	if len(queries) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(queries))
	flatArgs := make([]any, 0)
	for index, query := range queries {
		parts = append(parts, fmt.Sprintf("SELECT %d AS batch_ordinal, batch_query_%d.* FROM (%s) AS batch_query_%d", index, index, query, index))
		flatArgs = append(flatArgs, args[index]...)
	}
	return `SELECT batch.handle, batch.path, batch.language, batch.kind, batch.symbol_name, batch.signature, batch.start_line, batch.end_line, batch.start_byte, batch.end_byte, batch.content, batch.content_hash
FROM (` + strings.Join(parts, " UNION ALL ") + `) AS batch
ORDER BY batch.batch_ordinal, batch.path, batch.start_line, batch.handle`, flatArgs
}

func unionBatchQueriesWithAnchor(queries []string, args [][]any, handles []string) (string, []any) {
	if len(queries) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(queries))
	flatArgs := make([]any, 0)
	for index, query := range queries {
		parts = append(parts, fmt.Sprintf("SELECT ? AS batch_anchor, batch_query_%d.* FROM (%s) AS batch_query_%d", index, query, index))
		flatArgs = append(flatArgs, handles[index])
		flatArgs = append(flatArgs, args[index]...)
	}
	return strings.Join(parts, " UNION ALL "), flatArgs
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

func scanRelationHits(rows *sql.Rows, relation string) ([]model.RelationHit, error) {
	result := make([]model.RelationHit, 0)
	for rows.Next() {
		var candidate model.RankedCandidate
		var anchorHandle, source, direction string
		var confidence float64
		var resolved int
		if err := rows.Scan(&anchorHandle, &candidate.Handle, &candidate.Path, &candidate.Language, &candidate.Kind, &candidate.Symbol, &candidate.Signature, &candidate.StartLine, &candidate.EndLine, &candidate.StartByte, &candidate.EndByte, &candidate.Content, &candidate.ContentHash, &confidence, &source, &resolved, &direction); err != nil {
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
	if len(handles) > maxRelationBatchHandles {
		all := make([]model.RankedCandidate, 0)
		for start := 0; start < len(handles); start += maxRelationBatchHandles {
			end := start + maxRelationBatchHandles
			if end > len(handles) {
				end = len(handles)
			}
			part, err := s.relatedCandidatesLegacy(ctx, handles[start:end], relation)
			if err != nil {
				return nil, err
			}
			all = append(all, part...)
		}
		return all, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0)
	queries := make([]string, 0, len(handles))
	queryArgs := make([][]any, 0, len(handles))
	camelPatterns := make([]string, len(handles))
	if relation == "callers" || relation == "tests" {
		var err error
		camelPatterns, err = s.camelCaseRelationPatterns(ctx, handles)
		if err != nil {
			return nil, err
		}
	}
	for index, handle := range handles {
		var query string
		var args []any
		camelCasePattern := camelPatterns[index]
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
		queries = append(queries, query)
		queryArgs = append(queryArgs, args)
	}
	query, args := unionBatchCandidateQueries(queries, queryArgs)
	rows, err := s.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	items, scanErr := scanCandidates(rows)
	rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	result = append(result, items...)
	return result, nil
}

func (s *Store) camelCaseRelationPatterns(ctx context.Context, handles []string) ([]string, error) {
	rowsInput := make([][]any, 0, len(handles))
	for index, handle := range handles {
		rowsInput = append(rowsInput, []any{handle, index})
	}
	cte, args := ordinalValuesCTE("input", []string{"handle", "ordinal"}, rowsInput)
	query := cte + `SELECT input.ordinal, symbol.name
FROM input
LEFT JOIN chunks chunk ON chunk.handle = input.handle
LEFT JOIN symbols symbol ON symbol.handle = COALESCE(chunk.symbol_handle, input.handle)
ORDER BY input.ordinal`
	rows, err := s.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find relation targets: %w", err)
	}
	defer rows.Close()
	patterns := make([]string, len(handles))
	for rows.Next() {
		var ordinal int
		var name sql.NullString
		if err := rows.Scan(&ordinal, &name); err != nil {
			return nil, err
		}
		if ordinal >= 0 && ordinal < len(patterns) && name.Valid {
			patterns[ordinal] = camelCaseLikePattern(name.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
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
