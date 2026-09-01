package store

import (
	"context"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

// SearchPathFiles returns repository-relative paths matching bounded path hints.
// It deliberately does not read chunks or source content.
func (s *Store) SearchPathFiles(ctx context.Context, hints []string, limit int) ([]string, error) {
	hints = lookupValues(hints)
	if len(hints) == 0 {
		return []string{}, nil
	}
	limit = retrievalLimit(limit, 16, 16)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, hint := range hints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hint = strings.ReplaceAll(hint, "\\", "/")
		escaped := escapeLike(strings.ToLower(hint))
		rows, err := s.db.QueryContext(ctx, `SELECT path FROM files WHERE lower(path) LIKE '%' || ? || '%' ESCAPE '\'`, escaped)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0)
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return nil, err
			}
			paths = append(paths, path)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		sort.Slice(paths, func(i, j int) bool {
			leftClass, leftBase := pathMatchClass(paths[i], hint)
			rightClass, rightBase := pathMatchClass(paths[j], hint)
			if leftClass != rightClass {
				return leftClass < rightClass
			}
			if len(leftBase) != len(rightBase) {
				return len(leftBase) < len(rightBase)
			}
			if len(paths[i]) != len(paths[j]) {
				return len(paths[i]) < len(paths[j])
			}
			return paths[i] < paths[j]
		})
		for _, path := range paths {
			key := strings.ToLower(path)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, strings.ReplaceAll(path, "\\", "/"))
			if len(result) >= limit {
				return result, nil
			}
		}
	}
	return result, nil
}

func pathMatchClass(path, hint string) (int, string) {
	path = strings.ReplaceAll(path, "\\", "/")
	lowerPath := strings.ToLower(path)
	lowerHint := strings.ToLower(strings.Trim(hint, "/"))
	base := lowerPath
	if index := strings.LastIndexByte(base, '/'); index >= 0 {
		base = base[index+1:]
	}
	if lowerPath == lowerHint {
		return 0, base
	}
	if base == lowerHint {
		return 1, base
	}
	if strings.HasPrefix(base, lowerHint) {
		return 2, base
	}
	for _, segment := range strings.Split(lowerPath, "/") {
		if strings.HasPrefix(segment, lowerHint) {
			return 3, base
		}
	}
	return 4, base
}

// SearchFTSFiles collapses chunk-level FTS hits into one deterministic path list.
func (s *Store) SearchFTSFiles(ctx context.Context, ftsQuery string, limit int) ([]string, error) {
	ftsQuery = strings.TrimSpace(ftsQuery)
	if ftsQuery == "" {
		return []string{}, nil
	}
	limit = retrievalLimit(limit, 16, 100)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT f.path, COUNT(*) AS matches
FROM chunk_fts JOIN chunks c ON c.handle = chunk_fts.handle JOIN files f ON f.id = c.file_id
WHERE chunk_fts MATCH ?
GROUP BY f.id, f.path
ORDER BY matches DESC, f.path ASC
LIMIT ?`, ftsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0, limit)
	for rows.Next() {
		var path string
		var matches int
		if err := rows.Scan(&path, &matches); err != nil {
			return nil, err
		}
		result = append(result, strings.ReplaceAll(path, "\\", "/"))
	}
	return result, rows.Err()
}

// SearchSymbolFiles returns distinct files containing exact or prefix symbol matches.
func (s *Store) SearchSymbolFiles(ctx context.Context, hints []string, limit int) ([]string, error) {
	hints = lookupValues(hints)
	if len(hints) == 0 {
		return []string{}, nil
	}
	limit = retrievalLimit(limit, 16, 100)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, hint := range hints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped := escapeLike(strings.ToLower(hint))
		rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT f.path
FROM symbols symbol JOIN files f ON f.id = symbol.file_id
WHERE lower(symbol.name) = lower(?) OR lower(symbol.qualified_name) = lower(?)
			OR lower(symbol.name) LIKE ? || '%' ESCAPE '\'
			OR lower(symbol.qualified_name) LIKE ? || '%' ESCAPE '\'
ORDER BY CASE WHEN lower(symbol.name) = lower(?) OR lower(symbol.qualified_name) = lower(?) THEN 0 ELSE 1 END,
         f.path ASC
LIMIT ?`, hint, hint, escaped, escaped, hint, hint, limit)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return nil, err
			}
			path = strings.ReplaceAll(path, "\\", "/")
			key := strings.ToLower(path)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, path)
			if len(result) >= limit {
				break
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

// SearchCandidatesInFiles returns only symbol-owned chunks from selected paths.
func (s *Store) SearchCandidatesInFiles(ctx context.Context, paths, symbolHints []string, ftsQuery string, perFileLimit, totalLimit int) ([]model.RankedCandidate, error) {
	paths = lookupValues(paths)
	symbolHints = lookupValues(symbolHints)
	if len(paths) == 0 {
		return []model.RankedCandidate{}, nil
	}
	if len(paths) > 8 {
		paths = paths[:8]
	}
	perFileLimit = retrievalLimit(perFileLimit, 4, 8)
	totalLimit = retrievalLimit(totalLimit, 24, 40)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RankedCandidate, 0, totalLimit)
	seen := make(map[string]bool, totalLimit)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := s.searchSymbolCandidatesInFile(ctx, path, symbolHints, perFileLimit)
		if err != nil {
			return nil, err
		}
		if len(items) < perFileLimit && strings.TrimSpace(ftsQuery) != "" {
			ftsItems, ftsErr := s.searchFTSCandidatesInFile(ctx, path, ftsQuery, perFileLimit-len(items))
			if ftsErr != nil {
				return nil, ftsErr
			}
			items = append(items, ftsItems...)
		}
		before := len(result)
		appendCandidates(&result, seen, items, totalLimit)
		if len(result)-before > perFileLimit {
			result = result[:before+perFileLimit]
		}
		if len(result) >= totalLimit {
			break
		}
	}
	return result, nil
}

func (s *Store) searchSymbolCandidatesInFile(ctx context.Context, path string, hints []string, limit int) ([]model.RankedCandidate, error) {
	if len(hints) == 0 || limit <= 0 {
		return []model.RankedCandidate{}, nil
	}
	conditions := make([]string, 0, len(hints)*2)
	args := make([]any, 0, len(hints)*4+1)
	for _, hint := range hints {
		escaped := escapeLike(strings.ToLower(hint))
		conditions = append(conditions, `(lower(symbol.name) = lower(?) OR lower(symbol.qualified_name) = lower(?) OR lower(symbol.name) LIKE ? || '%' ESCAPE '\' OR lower(symbol.qualified_name) LIKE ? || '%' ESCAPE '\')`)
		args = append(args, hint, hint, escaped, escaped)
	}
	args = append(args, path, limit)
	query := rankedCandidateProjection + `c.symbol_handle IS NOT NULL AND f.path = ? AND (` + strings.Join(conditions, " OR ") + `)
ORDER BY CASE WHEN lower(symbol.name) = lower(?) THEN 0 ELSE 1 END, symbol.confidence DESC, c.end_line - c.start_line ASC, f.path ASC, c.start_line ASC, c.handle ASC LIMIT ?`
	// The ordering expression uses the first hint as a deterministic exact-match preference.
	conditionArgs := append([]any(nil), args[:len(args)-2]...)
	args = append(conditionArgs, args[len(args)-2], hints[0], args[len(args)-1])
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRankedCandidates(rows)
}

func (s *Store) searchFTSCandidatesInFile(ctx context.Context, path, ftsQuery string, limit int) ([]model.RankedCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.handle, f.path, f.language, c.kind, c.symbol_name, c.signature, c.start_line, c.end_line, c.start_byte, c.end_byte, c.content, c.content_hash, COALESCE(symbol.confidence, 0)
FROM chunk_fts JOIN chunks c ON c.handle = chunk_fts.handle JOIN files f ON f.id = c.file_id LEFT JOIN symbols symbol ON symbol.handle = c.symbol_handle
WHERE chunk_fts MATCH ? AND f.path = ? AND c.symbol_handle IS NOT NULL
ORDER BY bm25(chunk_fts), COALESCE(symbol.confidence, 0) DESC, c.end_line - c.start_line ASC, c.start_line ASC, c.handle ASC LIMIT ?`, ftsQuery, path, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRankedCandidates(rows)
}
