package search

import (
	"context"
	"errors"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/rank"
)

type CandidateStore interface {
	SearchFTS(ctx context.Context, query string) ([]model.RankedCandidate, error)
}

type relationStore interface {
	RelatedCandidates(ctx context.Context, handles []string, relation string) ([]model.RankedCandidate, error)
}

type Searcher struct{ store CandidateStore }

func New(store CandidateStore) *Searcher { return &Searcher{store: store} }

type LineRange struct{ Start, End int }

type SearchRequest struct {
	Query       string
	Paths       []string
	ChangedOnly bool
	Changed     map[string][]LineRange
	Limit       int
}

func (s *Searcher) Search(ctx context.Context, req SearchRequest) ([]model.RankedCandidate, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query must not be blank")
	}
	terms := NormalizeQuery(req.Query)
	candidates, err := s.store.SearchFTS(ctx, BuildFTSQuery(req.Query))
	if err != nil {
		return nil, err
	}
	if related, ok := s.store.(relationStore); ok {
		for _, relation := range queryRelations(terms) {
			for _, candidate := range candidates {
				if relation != "imports" && !containsSymbol(terms.Symbols, candidate.Symbol) {
					continue
				}
				neighbors, relationErr := related.RelatedCandidates(ctx, []string{candidate.Handle}, relation)
				if relationErr != nil {
					return nil, relationErr
				}
				for index := range neighbors {
					neighbors[index].Relation = relation
				}
				candidates = append(candidates, neighbors...)
			}
		}
	}
	filtered := make([]model.RankedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if len(req.Paths) > 0 && !pathMatches(candidate.Path, req.Paths) {
			continue
		}
		changed := overlaps(candidate.Path, candidate.StartLine, candidate.EndLine, req.Changed)
		if req.ChangedOnly && !changed {
			continue
		}
		candidate.Changed = changed
		filtered = append(filtered, candidate)
	}
	result := rank.RankWithIdentifiers(filtered, terms.Words, terms.Identifiers)
	if req.Limit > 0 && len(result) > req.Limit {
		result = result[:req.Limit]
	}
	return result, nil
}

func pathMatches(path string, filters []string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	for _, filter := range filters {
		filter = strings.ReplaceAll(filter, "\\", "/")
		if path == strings.TrimSuffix(filter, "/") || strings.HasPrefix(path, strings.TrimSuffix(filter, "/")+"/") {
			return true
		}
	}
	return false
}

func overlaps(path string, start, end int, changed map[string][]LineRange) bool {
	for _, r := range changed[path] {
		if start <= r.End && end >= r.Start {
			return true
		}
	}
	return false
}

func SortedWords(terms QueryTerms) []string { return sortedUnique(terms.Words) }

func queryRelations(terms QueryTerms) []string {
	imports := false
	for _, word := range terms.Words {
		switch word {
		case "call", "calls", "caller", "callers":
			return []string{"callers"}
		case "callee", "callees":
			return []string{"callees"}
		case "test", "tests", "testing", "coverage", "cover":
			return []string{"tests"}
		case "include", "includes", "included", "extends", "extend", "inherit", "inherits", "inherited", "layout", "layouts", "partial", "partials", "template":
			imports = true
		}
	}
	if imports {
		return []string{"imports"}
	}
	return nil
}

func containsSymbol(symbols []string, candidate string) bool {
	for _, symbol := range symbols {
		if strings.EqualFold(symbol, candidate) {
			return true
		}
	}
	return false
}
