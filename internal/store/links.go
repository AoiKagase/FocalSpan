package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

func (s *Store) Relations(ctx context.Context) ([]model.Relation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT from_handle, COALESCE(to_handle, ''), COALESCE(unresolved_to, ''), kind, confidence, source FROM relations ORDER BY from_handle, kind, unresolved_to, to_handle`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Relation, 0)
	for rows.Next() {
		var relation model.Relation
		if err := rows.Scan(&relation.FromHandle, &relation.ToHandle, &relation.UnresolvedTo, &relation.Kind, &relation.Confidence, &relation.Source); err != nil {
			return nil, err
		}
		result = append(result, relation)
	}
	return result, rows.Err()
}

func (s *Store) Symbols(ctx context.Context) ([]model.Symbol, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT symbols.handle, files.path, files.language, symbols.kind, symbols.name, symbols.qualified_name, symbols.signature, symbols.start_line, symbols.end_line, symbols.start_byte, symbols.end_byte, COALESCE(symbols.parent_handle, ''), symbols.confidence FROM symbols JOIN files ON files.id = symbols.file_id ORDER BY files.path, symbols.start_line, symbols.handle`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Symbol, 0)
	for rows.Next() {
		var symbol model.Symbol
		if err := rows.Scan(&symbol.Handle, &symbol.FilePath, &symbol.Language, &symbol.Kind, &symbol.Name, &symbol.QualifiedName, &symbol.Signature, &symbol.StartLine, &symbol.EndLine, &symbol.StartByte, &symbol.EndByte, &symbol.ParentHandle, &symbol.Confidence); err != nil {
			return nil, err
		}
		result = append(result, symbol)
	}
	return result, rows.Err()
}

func (s *Store) LinkRelation(ctx context.Context, fromHandle, unresolvedTo, kind, toHandle string) error {
	if fromHandle == "" || unresolvedTo == "" || kind == "" || toHandle == "" {
		return fmt.Errorf("relation link fields must not be blank")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE relations SET to_handle = ?, unresolved_to = NULL WHERE from_handle = ? AND unresolved_to = ? AND kind = ? AND to_handle IS NULL`, toHandle, fromHandle, unresolvedTo, kind)
	return err
}

func (s *Store) LinkRelations(ctx context.Context, links []RelationLink) error {
	if len(links) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin relation link batch: %w", err)
	}
	rollback := func(e error) error {
		_ = tx.Rollback()
		return e
	}
	for _, link := range links {
		if link.FromHandle == "" || link.UnresolvedTo == "" || link.Kind == "" || link.ToHandle == "" {
			return rollback(fmt.Errorf("relation link fields must not be blank"))
		}
		if _, err := tx.ExecContext(ctx, `UPDATE relations SET to_handle = ?, unresolved_to = NULL WHERE from_handle = ? AND unresolved_to = ? AND kind = ? AND to_handle IS NULL`, link.ToHandle, link.FromHandle, link.UnresolvedTo, link.Kind); err != nil {
			return rollback(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit relation link batch: %w", err)
	}
	return nil
}

func (s *Store) RelationsForLink(ctx context.Context, scope LinkScope) ([]model.Relation, error) {
	if !scope.Full && len(scope.ChangedPaths) == 0 && len(scope.LookupKeys) == 0 {
		return []model.Relation{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT r.from_handle, COALESCE(r.to_handle, ''), COALESCE(r.unresolved_to, ''), r.kind, r.confidence, r.source FROM relations r`
	args := make([]any, 0)
	if !scope.Full {
		parts := make([]string, 0, 2)
		if len(scope.ChangedPaths) > 0 {
			placeholders := make([]string, len(scope.ChangedPaths))
			for index, path := range scope.ChangedPaths {
				placeholders[index] = "?"
				args = append(args, path)
			}
			parts = append(parts, `EXISTS (SELECT 1 FROM symbols source_symbol JOIN files source_file ON source_file.id = source_symbol.file_id WHERE source_symbol.handle = r.from_handle AND source_file.path IN (`+strings.Join(placeholders, ",")+`))`)
		}
		if len(scope.LookupKeys) > 0 {
			placeholders := make([]string, len(scope.LookupKeys))
			for index, key := range scope.LookupKeys {
				placeholders[index] = "?"
				args = append(args, key)
			}
			parts = append(parts, `EXISTS (SELECT 1 FROM relation_lookup dependency WHERE dependency.relation_id = r.id AND dependency.lookup_key IN (`+strings.Join(placeholders, ",")+`))`)
		}
		query += ` WHERE r.to_handle IS NULL AND r.unresolved_to IS NOT NULL AND (` + strings.Join(parts, " OR ") + ")"
	} else if scope.UseProjection {
		query += ` WHERE r.to_handle IS NULL AND r.unresolved_to IS NOT NULL AND EXISTS (
SELECT 1 FROM relation_lookup dependency
WHERE dependency.relation_id = r.id AND (
  EXISTS (SELECT 1 FROM symbol_lookup symbol_key WHERE symbol_key.lookup_key = dependency.lookup_key)
  OR EXISTS (SELECT 1 FROM file_lookup file_key WHERE file_key.lookup_key = dependency.lookup_key)
))`
	} else {
		query += ` WHERE r.to_handle IS NULL AND r.unresolved_to IS NOT NULL`
	}
	query += ` ORDER BY r.from_handle, r.kind, r.unresolved_to, r.to_handle`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Relation, 0)
	for rows.Next() {
		var relation model.Relation
		if err := rows.Scan(&relation.FromHandle, &relation.ToHandle, &relation.UnresolvedTo, &relation.Kind, &relation.Confidence, &relation.Source); err != nil {
			return nil, err
		}
		result = append(result, relation)
	}
	return result, rows.Err()
}

func sameStorePath(left, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(strings.ReplaceAll(left, "\\", "/"), "./"), strings.TrimPrefix(strings.ReplaceAll(right, "\\", "/"), "./"))
}
