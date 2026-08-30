package rank

import (
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func Rank(candidates []model.RankedCandidate, terms []string) []model.RankedCandidate {
	plan := query.Plan{RawQuery: strings.Join(terms, " "), Terms: query.Terms{Words: append([]string(nil), terms...), Identifiers: identifiersFromTerms(terms)}}
	return RankWithPlan(candidates, plan)
}

func RankWithIdentifiers(candidates []model.RankedCandidate, terms, identifiers []string) []model.RankedCandidate {
	plan := query.Plan{RawQuery: strings.Join(terms, " "), Terms: query.Terms{Words: append([]string(nil), terms...), Identifiers: append([]string(nil), identifiers...)}}
	return RankWithPlan(candidates, plan)
}

func RankWithPlan(candidates []model.RankedCandidate, plan query.Plan) []model.RankedCandidate {
	profile := ProfileFor(plan)
	identifierTerms := make(map[string]bool, len(plan.Terms.Identifiers))
	for _, identifier := range plan.Terms.Identifiers {
		identifier = strings.ToLower(strings.TrimSpace(identifier))
		if identifier != "" {
			identifierTerms[identifier] = true
		}
	}
	return rankCandidates(candidates, plan.Terms.Words, identifierTerms, profile, plan.HasIntent(query.IntentTests), plan.HasIntent(query.IntentReferences))
}

func rankCandidates(candidates []model.RankedCandidate, terms []string, identifierTerms map[string]bool, profile Profile, testIntent, referenceIntent bool) []model.RankedCandidate {
	result := append([]model.RankedCandidate(nil), candidates...)
	for i := range result {
		result[i].Reasons = append([]model.ScoreReason(nil), result[i].Reasons...)
		if result[i].RetrievalScore > 0 {
			addReason(&result[i], "retrieval-fusion", result[i].RetrievalScore*fusionScale, "candidate is supported by ranked retrieval signals")
		}
		lowerSymbol := strings.ToLower(result[i].Symbol)
		lowerPath := strings.ToLower(result[i].Path)
		lowerContent := strings.ToLower(result[i].Content + " " + result[i].Signature + " " + result[i].Path)
		matched := 0
		pathScore := 0.0
		for _, term := range terms {
			term = strings.ToLower(term)
			if term == "" {
				continue
			}
			if len([]rune(term)) >= 3 && !isDocumentationCandidate(result[i]) {
				switch {
				case lowerSymbol == term && identifierTerms[term]:
					addReason(&result[i], "symbol-exact", profile.SymbolExact, "symbol matches a query term")
				case lowerSymbol != "" && (identifierTerms[term] || result[i].Signature != "") && symbolContainsTerm(lowerSymbol, term):
					weight := profile.NaturalPrefix
					if identifierTerms[term] {
						weight = profile.Prefix
					}
					addReason(&result[i], "symbol-prefix", weight, "symbol contains a query term")
				}
			}
			if strings.Contains(lowerContent, term) {
				matched++
			}
			if strings.Contains(lowerPath, term) {
				pathScore += 4
			}
		}
		if pathScore > profile.PathMax {
			pathScore = profile.PathMax
		}
		if pathScore > 0 {
			addReason(&result[i], "path", pathScore, "query terms occur in the candidate path")
		}
		if matched > 0 {
			weight := float64(matched * 8)
			if weight > profile.LexicalMax {
				weight = profile.LexicalMax
			}
			addReason(&result[i], "lexical", weight, "query terms occur in source metadata or content")
		}
		if testIntent && (result[i].Kind == "test" || strings.HasPrefix(result[i].Symbol, "Test")) {
			addReason(&result[i], "test-pair", profile.TestMatch, "test candidate retained for the query's test intent")
		} else if !testIntent && isTestCandidate(result[i]) {
			addReason(&result[i], "test-context-penalty", profile.NonTestPenalty, "test candidate is lower priority for a non-test question")
		}
		if result[i].Changed {
			addReason(&result[i], "changed-file", profile.ChangedFile, "file is changed in the selected Git state")
		}
		if isDocumentationCandidate(result[i]) {
			addReason(&result[i], "documentation-penalty", profile.DocumentationPenalty, "documentation is secondary when structural code candidates are available")
		}
		if result[i].Relation != "" {
			weight := profile.RelationWeights[result[i].Relation]
			if weight > 0 {
				addReason(&result[i], "relation-"+result[i].Relation, weight, "candidate was reached through the query's explicit relation intent")
			}
		}
		if referenceIntent && isReferenceKind(result[i].Kind) {
			addReason(&result[i], "reference-kind", 280, "trait or interface declaration is relevant to the reference question")
		}
		if result[i].EstimatedTokens > 4000 {
			addReason(&result[i], "large-span", profile.LargeSpanPenalty, "large span has reduced utility")
		}
	}
	sort.SliceStable(result, func(a, b int) bool {
		if result[a].Score != result[b].Score {
			return result[a].Score > result[b].Score
		}
		if result[a].Confidence != result[b].Confidence {
			return result[a].Confidence > result[b].Confidence
		}
		spanA, spanB := result[a].EndLine-result[a].StartLine, result[b].EndLine-result[b].StartLine
		if spanA != spanB {
			return spanA < spanB
		}
		if result[a].Path != result[b].Path {
			return result[a].Path < result[b].Path
		}
		if result[a].StartLine != result[b].StartLine {
			return result[a].StartLine < result[b].StartLine
		}
		return result[a].Handle < result[b].Handle
	})
	return Deduplicate(result)
}

func isReferenceKind(kind string) bool {
	kind = strings.ToLower(kind)
	return strings.Contains(kind, "trait") || strings.Contains(kind, "interface") || strings.Contains(kind, "protocol")
}

func addReason(candidate *model.RankedCandidate, code string, weight float64, detail string) {
	if hasReason(*candidate, code) {
		return
	}
	candidate.Score += weight
	candidate.Reasons = append(candidate.Reasons, model.ScoreReason{Code: code, Weight: weight, Detail: detail})
}

func identifiersFromTerms(terms []string) []string {
	result := make([]string, 0, len(terms))
	for _, term := range terms {
		if inferredIdentifiers([]string{term})[strings.ToLower(term)] {
			result = append(result, term)
		}
	}
	return result
}

func inferredIdentifiers(terms []string) map[string]bool {
	result := make(map[string]bool)
	for _, term := range terms {
		if strings.ContainsAny(term, "_:.\\/-") || hasUpperBoundary(term) {
			result[strings.ToLower(term)] = true
		}
	}
	return result
}

func symbolContainsTerm(symbol, term string) bool {
	if strings.Contains(symbol, term) {
		return true
	}
	return len([]rune(term)) > 3 && strings.HasSuffix(term, "s") && strings.TrimSuffix(term, "s") == symbol
}

func hasUpperBoundary(value string) bool {
	for index, r := range value {
		if index > 0 && r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func isDocumentationCandidate(candidate model.RankedCandidate) bool {
	return candidate.Kind == "heading" || candidate.Language == "markdown" || candidate.Language == "text"
}

func hasReason(candidate model.RankedCandidate, code string) bool {
	for _, reason := range candidate.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func isTestCandidate(candidate model.RankedCandidate) bool {
	if candidate.Kind != "test" && candidate.Kind != "test-suite" && !strings.HasPrefix(candidate.Symbol, "Test") {
		return false
	}
	path := strings.ToLower(strings.ReplaceAll(candidate.Path, "\\", "/"))
	return strings.Contains(path, "/test") || strings.HasPrefix(path, "test") || strings.Contains(path, "_test.") || strings.Contains(path, ".test.") || strings.Contains(path, ".spec.")
}

func Deduplicate(candidates []model.RankedCandidate) []model.RankedCandidate {
	result := make([]model.RankedCandidate, 0, len(candidates))
	hashes := make(map[string]bool)
	specificByPath := make(map[string]bool)
	for _, candidate := range candidates {
		if !isOutline(candidate.Kind) {
			specificByPath[candidate.Path] = true
		}
	}
	for _, candidate := range candidates {
		if isOutline(candidate.Kind) && candidate.Relation == "" && specificByPath[candidate.Path] && !hasReason(candidate, "symbol-exact") {
			continue
		}
		contentKey := candidate.Path + "\x00" + candidate.ContentHash
		if candidate.ContentHash != "" && hashes[contentKey] {
			continue
		}
		contained := false
		for _, accepted := range result {
			if candidate.Path != accepted.Path || candidate.Score > accepted.Score {
				continue
			}
			if containsSpan(accepted, candidate) || containsSpan(candidate, accepted) {
				contained = true
				break
			}
		}
		if contained {
			continue
		}
		if candidate.ContentHash != "" {
			hashes[contentKey] = true
		}
		result = append(result, candidate)
	}
	return result
}

func isOutline(kind string) bool {
	return strings.HasSuffix(kind, "-outline") || kind == "test-suite"
}

func hasSpecificCandidate(candidates []model.RankedCandidate, path string) bool {
	for _, candidate := range candidates {
		if candidate.Path == path && !strings.HasSuffix(candidate.Kind, "-outline") {
			return true
		}
	}
	return false
}

func containsSpan(outer, inner model.RankedCandidate) bool {
	if outer.StartLine <= 0 || outer.EndLine < outer.StartLine || inner.StartLine <= 0 || inner.EndLine < inner.StartLine {
		return false
	}
	return outer.StartLine <= inner.StartLine && outer.EndLine >= inner.EndLine
}
