package store

import (
	"context"
	"github.com/focalspan/focalspan/internal/cppscope"
	"github.com/focalspan/focalspan/internal/model"
)

type cppLexicalEdge struct{ target, kind, source string }
type cppRelationFilter struct {
	symbols     []model.Symbol
	byHandle    map[string]model.Symbol
	chunkSymbol map[string]string
	edges       map[string][]cppLexicalEdge
}

// Called under the request's existing store read lock, after candidate rows close.
func (s *Store) loadCPPRelationFilter(ctx context.Context) (*cppRelationFilter, error) {
	symbols, err := s.symbolsUnlocked(ctx)
	if err != nil {
		return nil, err
	}
	f := &cppRelationFilter{symbols: symbols, byHandle: map[string]model.Symbol{}, chunkSymbol: map[string]string{}, edges: map[string][]cppLexicalEdge{}}
	for _, v := range symbols {
		f.byHandle[v.Handle] = v
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.handle,c.symbol_handle FROM chunks c JOIN files f ON f.id=c.file_id WHERE f.language IN ('c','cpp')`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ch, sy string
		if err = rows.Scan(&ch, &sy); err != nil {
			rows.Close()
			return nil, err
		}
		f.chunkSymbol[ch] = sy
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT r.from_handle,r.unresolved_to,r.kind,r.source FROM relations r JOIN symbols origin ON origin.handle=r.from_handle JOIN files f ON f.id=origin.file_id WHERE r.to_handle IS NULL AND f.language='cpp' AND r.kind IN ('calls','tests')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var from string
		var edge cppLexicalEdge
		if err = rows.Scan(&from, &edge.target, &edge.kind, &edge.source); err != nil {
			return nil, err
		}
		f.edges[from] = append(f.edges[from], edge)
	}
	return f, rows.Err()
}
func (f *cppRelationFilter) symbol(h string) model.Symbol {
	if s := f.chunkSymbol[h]; s != "" {
		h = s
	}
	return f.byHandle[h]
}
func (f *cppRelationFilter) filter(handle string, hits []model.RelationHit) []model.RelationHit {
	result := hits[:0]
	anchor := f.symbol(handle)
	for _, h := range hits {
		if h.Context.Resolved || h.Candidate.Language != "cpp" || !(h.Context.Kind == "callers" || h.Context.Kind == "callees" || h.Context.Kind == "tests" || h.Context.Kind == "neighbors") {
			result = append(result, h)
			continue
		}
		from, to := anchor, f.symbol(h.Candidate.Handle)
		if h.Context.Direction == "incoming" {
			from, to = to, anchor
		}
		if from.Language != "cpp" {
			result = append(result, h)
			continue
		}
		keep := false
		for _, e := range f.edges[from.Handle] {
			if e.source == h.Context.Source && cppscope.Matches(from, e.target, to, f.symbols) {
				keep = true
				break
			}
		}
		if keep {
			result = append(result, h)
		}
	}
	return result
}
