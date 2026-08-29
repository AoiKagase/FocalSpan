package rank

import (
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

const (
	qualifiedExactWeight = 120.0
	symbolExactWeight    = 100.0
	prefixWeight         = 70.0
	lexicalMaxWeight     = 40.0
	changedLineWeight    = 25.0
	changedFileWeight    = 15.0
	testPairWeight       = 15.0
)

func Rank(candidates []model.RankedCandidate, terms []string) []model.RankedCandidate {
	return rankCandidates(candidates, terms, nil)
}

func RankWithIdentifiers(candidates []model.RankedCandidate, terms, identifiers []string) []model.RankedCandidate {
	identifierTerms := make(map[string]bool, len(identifiers))
	for _, identifier := range identifiers {
		identifier = strings.ToLower(strings.TrimSpace(identifier))
		if identifier != "" {
			identifierTerms[identifier] = true
		}
	}
	return rankCandidates(candidates, terms, identifierTerms)
}

func rankCandidates(candidates []model.RankedCandidate, terms []string, identifierTerms map[string]bool) []model.RankedCandidate {
	result := append([]model.RankedCandidate(nil), candidates...)
	for i := range result {
		result[i].Reasons = append([]model.ScoreReason(nil), result[i].Reasons...)
		lowerSymbol := strings.ToLower(result[i].Symbol)
		lowerPath := strings.ToLower(result[i].Path)
		lowerContent := strings.ToLower(result[i].Content + " " + result[i].Signature + " " + result[i].Path)
		matched := 0
		for _, term := range terms {
			term = strings.ToLower(term)
			if term == "" {
				continue
			}
			switch {
			case lowerSymbol == term:
				result[i].Score += symbolExactWeight
				result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "symbol-exact", Weight: symbolExactWeight, Detail: "symbol matches a query term"})
			case lowerSymbol != "" && (result[i].Language != "php" || identifierTerms[term]) && strings.Contains(lowerSymbol, term):
				result[i].Score += prefixWeight
				result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "symbol-prefix", Weight: prefixWeight, Detail: "symbol contains a query term"})
			}
			if strings.Contains(lowerContent, term) {
				matched++
			}
			if strings.Contains(lowerPath, term) {
				result[i].Score += 4
			}
		}
		if matched > 0 {
			weight := float64(matched * 8)
			if weight > lexicalMaxWeight {
				weight = lexicalMaxWeight
			}
			result[i].Score += weight
			result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "lexical", Weight: weight, Detail: "query terms occur in source metadata or content"})
		}
		if result[i].Kind == "test" || strings.HasPrefix(result[i].Symbol, "Test") {
			result[i].Score += testPairWeight
			result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "test-pair", Weight: testPairWeight, Detail: "test candidate retained for production behavior"})
		}
		if result[i].Changed {
			result[i].Score += changedFileWeight
			result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "changed-file", Weight: changedFileWeight, Detail: "file is changed in the selected Git state"})
		}
		if result[i].Relation != "" {
			weight := map[string]float64{"callers": 150, "callees": 100, "tests": 90, "imports": 80}[result[i].Relation]
			if weight > 0 {
				result[i].Score += weight
				result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "relation-" + result[i].Relation, Weight: weight, Detail: "candidate was reached through the query's explicit relation intent"})
			}
		}
		if result[i].EstimatedTokens > 4000 {
			penalty := -10.0
			result[i].Score += penalty
			result[i].Reasons = append(result[i].Reasons, model.ScoreReason{Code: "large-span", Weight: penalty, Detail: "large span has reduced utility"})
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

func Deduplicate(candidates []model.RankedCandidate) []model.RankedCandidate {
	result := make([]model.RankedCandidate, 0, len(candidates))
	hashes := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.ContentHash != "" && hashes[candidate.ContentHash] {
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
			hashes[candidate.ContentHash] = true
		}
		result = append(result, candidate)
	}
	return result
}

func containsSpan(outer, inner model.RankedCandidate) bool {
	if outer.StartLine <= 0 || outer.EndLine < outer.StartLine || inner.StartLine <= 0 || inner.EndLine < inner.StartLine {
		return false
	}
	return outer.StartLine <= inner.StartLine && outer.EndLine >= inner.EndLine
}
