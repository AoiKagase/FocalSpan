package store

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
)

// SymbolsForIdentity reads indexed identity, independent of retrieval ranking,
// chunk multiplicity and candidate limits. Empty kind means any exact name.
func (s *Store) SymbolsForIdentity(ctx context.Context, path, name, kind string) ([]model.Symbol, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT symbols.handle, files.path, files.language, symbols.kind, symbols.name, symbols.qualified_name, symbols.signature, symbols.start_line, symbols.end_line, symbols.start_byte, symbols.end_byte, COALESCE(symbols.parent_handle, ''), symbols.confidence FROM symbols JOIN files ON files.id = symbols.file_id WHERE files.path = ? AND symbols.name = ? AND (? = '' OR symbols.kind = ?) ORDER BY symbols.start_line, symbols.handle`, path, name, kind, kind)
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
