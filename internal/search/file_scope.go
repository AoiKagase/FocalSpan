package search

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

const (
	pathScopeHintLimit   = 8
	symbolHintLimit      = 16
	pathScopeFileLimit   = 8
	pathScopePerFile     = 4
	pathScopeTotal       = 24
	pathScopeSignalLimit = 100
)

var fileScopeWeights = map[string]float64{
	"symbol": 1.80,
	"fts":    1.00,
	"path":   0.90,
}

var navigationWords = map[string]bool{
	"where": true, "what": true, "which": true, "who": true, "why": true, "how": true,
	"does": true, "do": true, "did": true, "is": true, "are": true, "was": true, "were": true,
	"before": true, "after": true, "adding": true, "support": true, "supports": true,
	"caller": true, "callers": true, "callee": true, "callees": true, "test": true,
	"tests": true, "reference": true, "references": true, "import": true, "imports": true,
	"export": true, "exports": true,
	"どこ": true, "場所": true, "処理": true, "流れ": true, "呼び出し元": true,
	"呼び出し先": true, "テスト": true, "参照": true, "定義": true,
}

func (r *RetrieverSet) retrievePathScope(ctx context.Context, plan query.Plan, req SearchRequest) ([]model.RankedCandidate, error) {
	pathHints := scopePathHints(plan, req)
	symbolHints := scopeSymbolHints(plan)
	ftsQuery := query.BuildFTS(plan.Terms)

	symbolPaths, err := r.store.SearchSymbolFiles(ctx, symbolHints, pathScopeSignalLimit)
	if err != nil {
		return nil, err
	}
	ftsPaths, err := r.store.SearchFTSFiles(ctx, ftsQuery, pathScopeSignalLimit)
	if err != nil {
		return nil, err
	}
	pathPaths, err := r.store.SearchPathFiles(ctx, pathHints, pathScopeSignalLimit)
	if err != nil {
		return nil, err
	}
	paths := fuseFileScopes([]fileScopeList{
		{signal: "symbol", paths: symbolPaths},
		{signal: "fts", paths: ftsPaths},
		{signal: "path", paths: pathPaths},
	}, pathScopeFileLimit)
	paths = filterFileScopes(paths, req.Paths)
	if len(paths) == 0 {
		return []model.RankedCandidate{}, nil
	}
	items, err := r.store.SearchCandidatesInFiles(ctx, paths, symbolHints, ftsQuery, pathScopePerFile, pathScopeTotal)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// filterFileScopes applies an explicit request path restriction before the
// bounded candidate query. This prevents broad symbol/FTS signals from
// consuming the per-file budget with candidates that will be discarded later
// by SearchDetailed.
func filterFileScopes(paths, filters []string) []string {
	if len(filters) == 0 {
		return paths
	}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if pathMatches(path, filters) {
			result = append(result, path)
		}
	}
	return result
}

type fileScopeList struct {
	signal string
	paths  []string
}

type fileScopeScore struct {
	path  string
	score float64
	count int
}

func fuseFileScopes(lists []fileScopeList, limit int) []string {
	merged := make(map[string]*fileScopeScore)
	for _, list := range lists {
		weight := fileScopeWeights[list.signal]
		if weight == 0 {
			continue
		}
		for index, rawPath := range list.paths {
			path := strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/")
			if path == "" {
				continue
			}
			key := strings.ToLower(path)
			entry := merged[key]
			if entry == nil {
				entry = &fileScopeScore{path: path}
				merged[key] = entry
			}
			entry.score += weight / (rrfK + float64(index+1))
			entry.count++
		}
	}
	values := make([]*fileScopeScore, 0, len(merged))
	for _, entry := range merged {
		values = append(values, entry)
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].score != values[j].score {
			return values[i].score > values[j].score
		}
		if values[i].count != values[j].count {
			return values[i].count > values[j].count
		}
		return values[i].path < values[j].path
	})
	if limit <= 0 || limit > pathScopeFileLimit {
		limit = pathScopeFileLimit
	}
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.path)
	}
	return result
}

func scopePathHints(plan query.Plan, req SearchRequest) []string {
	result := make([]string, 0, pathScopeHintLimit)
	seen := make(map[string]bool, pathScopeHintLimit)
	add := func(value string) {
		value = strings.Trim(strings.ReplaceAll(value, "\\", "/"), " ,;()[]{}\t\r\n")
		if value == "" || seen[strings.ToLower(value)] || len(result) >= pathScopeHintLimit {
			return
		}
		seen[strings.ToLower(value)] = true
		result = append(result, value)
	}
	for _, value := range req.Paths {
		add(value)
	}
	for _, value := range plan.Terms.Paths {
		add(value)
	}
	for _, value := range plan.Anchors {
		add(value)
	}
	for _, value := range plan.Terms.Identifiers {
		add(value)
	}
	for _, value := range plan.Terms.Symbols {
		add(value)
	}
	for _, value := range plan.Terms.Words {
		if utf8.RuneCountInString(value) >= 3 && !navigationWords[strings.ToLower(value)] {
			add(value)
		}
	}
	return result
}

func scopeSymbolHints(plan query.Plan) []string {
	result := make([]string, 0, symbolHintLimit)
	seen := make(map[string]bool, symbolHintLimit)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] || len(result) >= symbolHintLimit {
			return
		}
		seen[strings.ToLower(value)] = true
		result = append(result, value)
	}
	for _, source := range [][]string{plan.Anchors, plan.Terms.Symbols, plan.Terms.Identifiers, plan.Terms.Words} {
		for _, value := range source {
			if utf8.RuneCountInString(value) < 3 || navigationWords[strings.ToLower(value)] {
				continue
			}
			for _, variant := range identifierStyleVariants(value) {
				add(variant)
			}
		}
	}
	return result
}

func identifierStyleVariants(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == '\\' || r == '.' || r == ':' || r == '_'
	})
	if len(parts) == 0 {
		return []string{value}
	}
	result := []string{value}
	last := parts[len(parts)-1]
	if last != value {
		result = append(result, last)
	}
	var camel strings.Builder
	var pascal strings.Builder
	for index, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if index == 0 {
			camel.WriteString(lower)
			pascal.WriteString(upperFirstRune(lower))
		} else {
			camel.WriteString(upperFirstRune(lower))
			pascal.WriteString(upperFirstRune(lower))
		}
	}
	if camel.Len() > 0 {
		result = append(result, camel.String(), pascal.String())
	}
	return result
}

func upperFirstRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
