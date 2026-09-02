package search

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

const (
	qualifiedLimit = 50
	exactLimit     = 50
	prefixLimit    = 50
	ftsLimit       = 100
	pathLimit      = 50
	relationLimit  = 100
	fusedLimit     = 400
)

type RetrieverSet struct {
	store CandidateStore
}

func NewRetrieverSet(store CandidateStore) *RetrieverSet {
	return &RetrieverSet{store: store}
}

func (r *RetrieverSet) Retrieve(ctx context.Context, plan query.Plan, req SearchRequest) ([]RankedList, error) {
	mode := req.Mode
	if mode == "" {
		mode = RetrievalFull
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if mode == RetrievalFTSOnly {
		items, err := r.store.SearchFTS(ctx, query.BuildFTS(plan.Terms), ftsLimit)
		if err != nil {
			return nil, fmt.Errorf("%s retriever: %w", RetrieverFTS, err)
		}
		return []RankedList{{Retriever: RetrieverFTS, Items: items}}, nil
	}
	lists := make([]RankedList, 0, 6)
	qualified, err := r.store.SearchQualifiedSymbols(ctx, plan.Terms.Identifiers, qualifiedLimit)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverQualified, err)
	}
	lists = append(lists, RankedList{Retriever: RetrieverQualified, Items: qualified})
	exact, err := r.store.SearchExactSymbols(ctx, plan.Terms.Identifiers, exactLimit)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverSymbol, err)
	}
	lists = append(lists, RankedList{Retriever: RetrieverSymbol, Items: exact})
	prefix, err := r.store.SearchSymbolPrefixes(ctx, plan.Terms.Identifiers, prefixLimit)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverPrefix, err)
	}
	lists = append(lists, RankedList{Retriever: RetrieverPrefix, Items: prefix})
	fts, err := r.store.SearchFTS(ctx, query.BuildFTS(plan.Terms), ftsLimit)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverFTS, err)
	}
	lists = append(lists, RankedList{Retriever: RetrieverFTS, Items: fts})
	paths, err := r.store.SearchPaths(ctx, plan.Terms.Paths, pathLimit)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverPath, err)
	}
	lists = append(lists, RankedList{Retriever: RetrieverPath, Items: paths})
	if mode == RetrievalNoRelations || len(plan.Relations) == 0 {
		return lists, nil
	}
	anchors := retrievalAnchors(plan, lists)
	if len(anchors) == 0 {
		return lists, nil
	}
	handles := make([]string, 0, len(anchors))
	for _, candidate := range anchors {
		if candidate.Handle != "" {
			handles = append(handles, candidate.Handle)
		}
	}
	for _, relation := range plan.Relations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		legacyItems, relationErr := r.store.RelatedCandidates(ctx, handles, relation)
		if relationErr != nil {
			return nil, fmt.Errorf("%s retriever: %w", RetrieverRelation, relationErr)
		}
		hits, relationErr := r.store.RelatedCandidateHits(ctx, handles, relation)
		if relationErr != nil {
			return nil, fmt.Errorf("%s provenance: %w", RetrieverRelation, relationErr)
		}
		items := mergeRelationCandidates(legacyItems, hits, relation)
		lists = append(lists, RankedList{Retriever: RetrieverRelation, Items: items})
	}
	return lists, nil
}

func mergeRelationCandidates(legacy []model.RankedCandidate, hits []model.RelationHit, relation string) []model.RankedCandidate {
	provenance := relationCandidates(hits)
	if len(legacy) == 0 {
		return provenance
	}
	byIdentity := make(map[string]model.RankedCandidate, len(provenance))
	byHandle := make(map[string]model.RankedCandidate, len(provenance))
	for _, candidate := range provenance {
		byIdentity[candidateIdentity(candidate)] = candidate
		if current, exists := byHandle[candidate.Handle]; !exists || strongerRelationContext(candidate.RelationContext, current.RelationContext) {
			byHandle[candidate.Handle] = candidate
		}
	}
	result := append([]model.RankedCandidate(nil), legacy...)
	for index := range result {
		result[index].Relation = relation
		matched, ok := byIdentity[candidateIdentity(result[index])]
		if !ok {
			matched, ok = byHandle[result[index].Handle]
		}
		if ok && matched.RelationContext != nil {
			contextCopy := *matched.RelationContext
			result[index].RelationContext = &contextCopy
		}
	}
	return result
}

func relationCandidates(hits []model.RelationHit) []model.RankedCandidate {
	result := make([]model.RankedCandidate, 0, len(hits))
	byIdentity := make(map[string]int, len(hits))
	for _, hit := range hits {
		candidate := hit.Candidate
		candidate.Relation = hit.Context.Kind
		contextCopy := hit.Context
		candidate.RelationContext = &contextCopy
		key := candidateIdentity(candidate)
		if index, exists := byIdentity[key]; exists {
			if strongerRelationContext(candidate.RelationContext, result[index].RelationContext) {
				result[index].Relation = candidate.Relation
				result[index].RelationContext = candidate.RelationContext
			}
			continue
		}
		byIdentity[key] = len(result)
		result = append(result, candidate)
	}
	return result
}

func strongerRelationContext(left, right *model.RelationContext) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	if left.Resolved != right.Resolved {
		return left.Resolved
	}
	if left.Confidence != right.Confidence {
		return left.Confidence > right.Confidence
	}
	leftExact := left.Direction != model.RelationRelated
	rightExact := right.Direction != model.RelationRelated
	if leftExact != rightExact {
		return leftExact
	}
	leftKey := string(left.Direction) + "\x00" + left.Kind + "\x00" + left.AnchorHandle + "\x00" + left.Source
	rightKey := string(right.Direction) + "\x00" + right.Kind + "\x00" + right.AnchorHandle + "\x00" + right.Source
	return leftKey < rightKey
}

func retrievalAnchors(plan query.Plan, lists []RankedList) []model.RankedCandidate {
	anchorValues := make(map[string]bool, len(plan.Anchors))
	for _, anchor := range plan.Anchors {
		anchorValues[strings.ToLower(strings.TrimSpace(anchor))] = true
	}
	order := []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPath, RetrieverPrefix, RetrieverFTS}
	byRetriever := make(map[RetrieverID][]model.RankedCandidate, len(lists))
	for _, list := range lists {
		byRetriever[list.Retriever] = list.Items
	}
	result := make([]model.RankedCandidate, 0, 8)
	seen := make(map[string]bool, 8)
	for _, retriever := range order {
		for _, candidate := range byRetriever[retriever] {
			if len(anchorValues) > 0 && !candidateMatchesAnchor(candidate, anchorValues) {
				continue
			}
			key := candidate.Handle
			if key == "" {
				key = candidate.Path + "\x00" + candidate.Symbol
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, candidate)
			if len(result) == 8 {
				return result
			}
		}
	}
	return result
}

func candidateMatchesAnchor(candidate model.RankedCandidate, anchors map[string]bool) bool {
	symbol := strings.ToLower(strings.TrimSpace(candidate.Symbol))
	path := strings.ToLower(strings.ReplaceAll(candidate.Path, "\\", "/"))
	for anchor := range anchors {
		if symbol == anchor || path == anchor || strings.HasSuffix(path, "/"+anchor) {
			return true
		}
	}
	return false
}

func retrieverSummaries(lists []RankedList) []RetrieverSummary {
	result := make([]RetrieverSummary, 0, len(lists))
	for _, list := range lists {
		result = append(result, RetrieverSummary{Retriever: list.Retriever, Count: len(list.Items)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Retriever < result[j].Retriever })
	return result
}
