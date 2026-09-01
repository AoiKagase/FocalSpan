package search

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
	"github.com/focalspan/focalspan/internal/rank"
)

type CandidateStore interface {
	SearchFTS(ctx context.Context, query string, limit int) ([]model.RankedCandidate, error)
	SearchQualifiedSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
	SearchExactSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
	SearchSymbolPrefixes(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
	SearchPaths(ctx context.Context, hints []string, limit int) ([]model.RankedCandidate, error)
	RelatedCandidates(ctx context.Context, handles []string, relation string) ([]model.RankedCandidate, error)
	RelatedCandidateHits(ctx context.Context, handles []string, relation string) ([]model.RelationHit, error)
}

// StructuralBridgeStore is implemented by stores that can resolve an
// explicit package/module identity to symbol-bearing chunks. It is optional so
// lightweight CandidateStore implementations retain their existing contract.
type StructuralBridgeStore interface {
	SearchStructuralBridge(ctx context.Context, packageHints, symbolHints []string, limit int) ([]model.RankedCandidate, error)
}

type Searcher struct {
	store      CandidateStore
	retrievers *RetrieverSet
}

func New(store CandidateStore) *Searcher {
	return &Searcher{store: store, retrievers: &RetrieverSet{store: store}}
}

type LineRange struct{ Start, End int }

type SearchRequest struct {
	Query       string
	Paths       []string
	ChangedOnly bool
	Changed     map[string][]LineRange
	Limit       int
	Mode        RetrievalMode
	Trace       bool
}

func (s *Searcher) Search(ctx context.Context, req SearchRequest) ([]model.RankedCandidate, error) {
	result, err := s.SearchDetailed(ctx, req)
	if err != nil {
		return nil, err
	}
	return result.Candidates, nil
}

func (s *Searcher) SearchDetailed(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return SearchResult{}, errors.New("query must not be blank")
	}
	plan := query.PlanQuery(req.Query)
	lists, err := s.retrievers.Retrieve(ctx, plan, req)
	if err != nil {
		return SearchResult{}, err
	}
	candidates, traces := fuseRankedLists(lists, fusedLimit)
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
	result := rank.RankWithPlan(filtered, plan)
	if req.Limit > 0 && len(result) > req.Limit {
		result = result[:req.Limit]
	}
	searchResult := SearchResult{Plan: plan, Candidates: result}
	if !req.Trace {
		return searchResult, nil
	}
	mode := req.Mode
	if mode == "" {
		mode = RetrievalFull
	}
	trace := &SearchTrace{Mode: mode, Lists: retrieverSummaries(lists), Retrieved: stageCandidateTraces(lists), Candidates: make([]CandidateTrace, 0, len(result))}
	traceByIdentity := make(map[string]CandidateTrace, len(traces))
	for _, candidateTrace := range traces {
		traceByIdentity[candidateTrace.Handle] = candidateTrace
	}
	for rankedIndex, candidate := range result {
		candidateTrace, ok := traceByIdentity[candidate.Handle]
		if !ok {
			candidateTrace = CandidateTrace{
				Handle: candidate.Handle, Path: candidate.Path, Symbol: candidate.Symbol, Kind: candidate.Kind,
				StartLine: candidate.StartLine, EndLine: candidate.EndLine,
			}
		}
		candidateTrace.Kind = candidate.Kind
		candidateTrace.RankedPosition = rankedIndex + 1
		candidateTrace.FinalScore = candidate.Score
		candidateTrace.Reasons = append([]model.ScoreReason(nil), candidate.Reasons...)
		trace.Candidates = append(trace.Candidates, candidateTrace)
	}
	searchResult.Trace = trace
	return searchResult, nil
}

func stageCandidateTraces(lists []RankedList) []StageCandidateTrace {
	var traces []StageCandidateTrace
	for _, list := range lists {
		for index, candidate := range list.Items {
			trace := StageCandidateTrace{
				Retriever: list.Retriever,
				Position:  index + 1,
				Path:      candidate.Path,
				Symbol:    candidate.Symbol,
				Kind:      candidate.Kind,
				Relation:  candidate.Relation,
			}
			if candidate.RelationContext != nil {
				trace.RelationResolved = candidate.RelationContext.Resolved
			}
			traces = append(traces, trace)
		}
	}
	return traces
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

func SortedWords(terms query.Terms) []string {
	words := append([]string(nil), terms.Words...)
	sort.Strings(words)
	result := words[:0]
	for _, word := range words {
		if len(result) == 0 || result[len(result)-1] != word {
			result = append(result, word)
		}
	}
	return result
}
