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

func sameStorePath(left, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(strings.ReplaceAll(left, "\\", "/"), "./"), strings.TrimPrefix(strings.ReplaceAll(right, "\\", "/"), "./"))
}
