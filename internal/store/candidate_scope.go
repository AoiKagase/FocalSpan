package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type candidateScopeKey struct{}
type candidateScopeSQL struct {
	paths, ranges         string
	hasPaths, changedOnly bool
}
type scopedCandidates struct {
	store *Store
	scope candidateScopeSQL
}
type scopedRange struct {
	Path  string `json:"p"`
	Start int    `json:"s"`
	End   int    `json:"e"`
}

// Fixed-size JSON batches keep SQL parameters/expression depth independent of
// the number of requested paths or changed ranges.
func scopeBatches[T any](items []T) string {
	batches := make([][]T, 0)
	for len(items) > 0 {
		n := 100
		if len(items) < n {
			n = len(items)
		}
		batches = append(batches, items[:n])
		items = items[n:]
	}
	data, _ := json.Marshal(batches)
	return string(data)
}

func (s *Store) ScopedCandidates(scope model.CandidateScope) model.CandidateReader {
	paths := make([]string, 0, len(scope.Paths))
	for _, p := range scope.Paths {
		paths = append(paths, strings.TrimSuffix(strings.ReplaceAll(p, "\\", "/"), "/"))
	}
	keys := make([]string, 0, len(scope.Changed))
	for path := range scope.Changed {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	ranges := make([]scopedRange, 0)
	for _, path := range keys {
		for _, r := range scope.Changed[path] {
			ranges = append(ranges, scopedRange{path, r.Start, r.End})
		}
	}
	return &scopedCandidates{s, candidateScopeSQL{scopeBatches(paths), scopeBatches(ranges), len(paths) > 0, scope.ChangedOnly}}
}
func (v *scopedCandidates) ctx(ctx context.Context) context.Context {
	return context.WithValue(ctx, candidateScopeKey{}, v.scope)
}
func (v *scopedCandidates) SearchFTS(ctx context.Context, q string, n int) ([]model.RankedCandidate, error) {
	return v.store.SearchFTS(v.ctx(ctx), q, n)
}
func (v *scopedCandidates) SearchQualifiedSymbols(ctx context.Context, q []string, n int) ([]model.RankedCandidate, error) {
	return v.store.SearchQualifiedSymbols(v.ctx(ctx), q, n)
}
func (v *scopedCandidates) SearchExactSymbols(ctx context.Context, q []string, n int) ([]model.RankedCandidate, error) {
	return v.store.SearchExactSymbols(v.ctx(ctx), q, n)
}
func (v *scopedCandidates) SearchSymbolPrefixes(ctx context.Context, q []string, n int) ([]model.RankedCandidate, error) {
	return v.store.SearchSymbolPrefixes(v.ctx(ctx), q, n)
}
func (v *scopedCandidates) SearchPaths(ctx context.Context, q []string, n int) ([]model.RankedCandidate, error) {
	return v.store.SearchPaths(v.ctx(ctx), q, n)
}
func (v *scopedCandidates) RelatedCandidates(ctx context.Context, h []string, r string) ([]model.RankedCandidate, error) {
	return v.store.RelatedCandidates(v.ctx(ctx), h, r)
}
func (v *scopedCandidates) RelatedCandidateHits(ctx context.Context, h []string, r string) ([]model.RelationHit, error) {
	return v.store.RelatedCandidateHits(v.ctx(ctx), h, r)
}

func (s *Store) queryCandidateRows(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	scope, ok := ctx.Value(candidateScopeKey{}).(candidateScopeSQL)
	if !ok || (!scope.hasPaths && !scope.changedOnly) {
		return s.db.QueryContext(ctx, query, args...)
	}
	conditions := make([]string, 0, 2)
	if scope.hasPaths {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM json_each(cs.paths) batch, json_each(batch.value) p WHERE sf.path = p.value OR substr(sf.path, 1, length(p.value)+1) = p.value || '/')`)
	}
	if scope.changedOnly {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM json_each(cs.ranges) batch, json_each(batch.value) r WHERE sf.path = json_extract(r.value, '$.p') AND sc.start_line <= json_extract(r.value, '$.e') AND sc.end_line >= json_extract(r.value, '$.s'))`)
	}
	derived := `(SELECT sc.* FROM chunks sc JOIN files sf ON sf.id = sc.file_id CROSS JOIN candidate_scope cs WHERE ` + strings.Join(conditions, " AND ") + `) c`
	query = strings.ReplaceAll(query, "chunks c", derived)
	prefix := `WITH candidate_scope(paths, ranges) AS (VALUES (?, ?)) `
	if strings.HasPrefix(strings.TrimSpace(query), "WITH ") {
		query = prefix + ", " + strings.TrimPrefix(strings.TrimSpace(query), "WITH ")
	} else {
		query = prefix + query
	}
	params := make([]any, 0, len(args)+2)
	params = append(params, scope.paths, scope.ranges)
	params = append(params, args...)
	return s.db.QueryContext(ctx, query, params...)
}
