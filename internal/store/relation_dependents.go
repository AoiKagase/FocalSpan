package store

import (
	"context"
	"encoding/json"
)

// IncomingRelationPaths is evaluated before replacing symbols. Reextracting
// these sources preserves their original lexical target when a target changes.
func (s *Store) IncomingRelationPaths(ctx context.Context, targetPaths []string) ([]string, error) {
	if len(targetPaths) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(targetPaths)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT source_file.path FROM relations r
 JOIN symbols source ON source.handle=r.from_handle JOIN files source_file ON source_file.id=source.file_id
 JOIN symbols target ON target.handle=r.to_handle JOIN files target_file ON target_file.id=target.file_id
 WHERE target_file.path IN (SELECT value FROM json_each(?)) ORDER BY source_file.path`, string(encoded))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// DependentSourcePaths includes already resolved edges. Their lexical target
// keys remain in relation_lookup after linking and can be invalidated when a
// newly indexed declaration makes the previous unique target ambiguous.
func (s *Store) DependentSourcePaths(ctx context.Context, lookupKeys []string) ([]string, error) {
	if len(lookupKeys) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(lookupKeys)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT f.path FROM relation_lookup dependency
 JOIN relations r ON r.id=dependency.relation_id
 JOIN symbols source ON source.handle=r.from_handle JOIN files f ON f.id=source.file_id
 WHERE dependency.lookup_key IN (SELECT value FROM json_each(?)) ORDER BY f.path`, string(encoded))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
