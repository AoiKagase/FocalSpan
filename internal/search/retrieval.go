package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

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

	pathScopeHintLimit       = 8
	pathScopeFileLimit       = 8
	pathScopeSymbolHintLimit = 16
	pathScopeSymbolLimit     = 40
	pathScopePerFileLimit    = 8
	pathScopeTokenRuneLimit  = 128
)

var pathScopeNavigationTerms = map[string]bool{
	"where": true, "what": true, "which": true, "who": true, "why": true,
	"how": true, "is": true, "are": true, "was": true, "were": true,
	"does": true, "do": true, "did": true, "before": true, "after": true,
	"adding": true, "support": true, "supports": true, "caller": true,
	"callers": true, "callee": true, "callees": true, "test": true,
	"tests": true, "reference": true, "references": true, "import": true,
	"imports": true, "export": true, "exports": true,
	"どこ": true, "場所": true, "処理": true, "流れ": true,
	"呼び出し元": true, "呼び出し先": true, "テスト": true,
	"参照": true, "定義": true,
}

func pathScopeHints(plan query.Plan) []string {
	result := make([]string, 0, pathScopeHintLimit)
	seen := make(map[string]bool, pathScopeHintLimit)
	add := func(value string, requireWordLength bool) bool {
		value = sanitizeScopeValue(value)
		if value == "" || pathScopeNavigationTerms[strings.ToLower(value)] || requireWordLength && len([]rune(value)) < 3 {
			return true
		}
		key := strings.ToLower(value)
		if seen[key] {
			return true
		}
		if len(result) >= pathScopeHintLimit {
			return false
		}
		seen[key] = true
		result = append(result, value)
		return true
	}
	for _, values := range [][]string{plan.Terms.Paths, plan.Terms.Identifiers, plan.Terms.Symbols, plan.Anchors} {
		for _, value := range values {
			if !add(value, false) {
				return result
			}
		}
	}
	for _, value := range plan.Terms.Words {
		if !add(value, true) {
			return result
		}
	}
	return result
}

func identifierStyleVariants(value string) []string {
	value = sanitizeScopeValue(value)
	if value == "" {
		return []string{}
	}
	result := make([]string, 0, 3)
	seen := make(map[string]bool, 3)
	add := func(candidate string) {
		candidate = sanitizeScopeValue(candidate)
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		result = append(result, candidate)
	}
	add(value)
	if strings.ContainsAny(value, `/\`) {
		normalized := strings.ReplaceAll(value, "\\", "/")
		base := normalized[strings.LastIndex(normalized, "/")+1:]
		add(base)
		if dot := strings.LastIndex(base, "."); dot > 0 {
			add(base[:dot])
		}
		return result
	}

	final := value
	if index := strings.LastIndex(final, "::"); index >= 0 {
		final = final[index+2:]
	} else if index := strings.LastIndexAny(final, ".:"); index >= 0 {
		final = final[index+1:]
	}
	if final != value {
		add(final)
		return result
	}
	parts := strings.FieldsFunc(final, func(r rune) bool { return r == '_' || r == '-' || unicode.IsSpace(r) })
	if len(parts) < 2 {
		return result
	}
	for index := range parts {
		parts[index] = upperFirst(strings.ToLower(parts[index]))
	}
	pascal := strings.Join(parts, "")
	add(lowerFirst(pascal))
	add(pascal)
	return result
}

func pathScopedSymbolHints(plan query.Plan) []string {
	result := make([]string, 0, pathScopeSymbolHintLimit)
	seen := make(map[string]bool, pathScopeSymbolHintLimit)
	add := func(value string, requireWordLength bool) bool {
		value = sanitizeScopeValue(value)
		if value == "" || pathScopeNavigationTerms[strings.ToLower(value)] || requireWordLength && len([]rune(value)) < 3 {
			return true
		}
		for _, variant := range identifierStyleVariants(value) {
			if seen[variant] {
				continue
			}
			if len(result) >= pathScopeSymbolHintLimit {
				return false
			}
			seen[variant] = true
			result = append(result, variant)
		}
		return true
	}
	for _, values := range [][]string{plan.Anchors, plan.Terms.Symbols, plan.Terms.Identifiers} {
		for _, value := range values {
			if !add(value, false) {
				return result
			}
		}
	}
	for _, value := range plan.Terms.Words {
		if !add(value, true) {
			return result
		}
	}
	return result
}

func collectScopedPaths(plan query.Plan, req SearchRequest, lists []RankedList, probed []string) []string {
	if req.Mode == RetrievalFTSOnly {
		return []string{}
	}
	result := make([]string, 0, pathScopeFileLimit)
	seen := make(map[string]bool, pathScopeFileLimit)
	add := func(path string) bool {
		path = sanitizeScopeValue(strings.ReplaceAll(path, "\\", "/"))
		if path == "" {
			return true
		}
		key := strings.ToLower(path)
		if seen[key] {
			return true
		}
		if len(result) >= pathScopeFileLimit {
			return false
		}
		seen[key] = true
		result = append(result, path)
		return true
	}
	for _, path := range req.Paths {
		if !add(path) {
			return result
		}
	}
	for _, retriever := range []RetrieverID{RetrieverPath, RetrieverFTS} {
		for _, list := range lists {
			if list.Retriever != retriever {
				continue
			}
			for _, candidate := range list.Items {
				if !add(candidate.Path) {
					return result
				}
			}
		}
	}
	if len(plan.Relations) == 0 {
		for _, path := range probed {
			if !add(path) {
				return result
			}
		}
	}
	return result
}

func sanitizeScopeValue(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.TrimSpace(value)
	value = strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && !strings.ContainsRune("_:/\\.-", r)
	})
	runes := []rune(value)
	if len(runes) > pathScopeTokenRuneLimit {
		value = string(runes[:pathScopeTokenRuneLimit])
	}
	return value
}

func upperFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func lowerFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

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
	probed := []string{}
	if len(plan.Relations) == 0 {
		probed, err = r.store.SearchFilePaths(ctx, pathScopeHints(plan), pathScopeFileLimit)
		if err != nil {
			return nil, fmt.Errorf("file path probe: %w", err)
		}
	}
	scopedPaths := collectScopedPaths(plan, req, lists, probed)
	scoped, err := r.store.SearchSymbolsInPaths(
		ctx,
		scopedPaths,
		pathScopedSymbolHints(plan),
		query.BuildFTS(plan.Terms),
		pathScopePerFileLimit,
		pathScopeSymbolLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("%s retriever: %w", RetrieverPathScopedSymbol, err)
	}
	if len(scoped) > 0 {
		lists = append(lists, RankedList{Retriever: RetrieverPathScopedSymbol, Items: scoped})
	}
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
	order := []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPathScopedSymbol, RetrieverPath, RetrieverPrefix, RetrieverFTS}
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
