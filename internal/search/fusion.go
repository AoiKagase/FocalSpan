package search

import (
	"fmt"
	"sort"

	"github.com/focalspan/focalspan/internal/model"
)

const rrfK = 60.0

var retrieverWeights = map[RetrieverID]float64{
	RetrieverQualified:             2.00,
	RetrieverSymbol:                1.80,
	RetrieverRelation:              1.60,
	RetrieverStructuralConstructor: 1.50,
	RetrieverPrefix:                1.20,
	RetrieverFTS:                   1.00,
	RetrieverPath:                  0.90,
}

type fusedCandidate struct {
	candidate model.RankedCandidate
	trace     CandidateTrace
	count     int
}

func fuseRankedLists(lists []RankedList, limit int) ([]model.RankedCandidate, []CandidateTrace) {
	ordered := append([]RankedList(nil), lists...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Retriever < ordered[j].Retriever })
	merged := make(map[string]*fusedCandidate)
	for _, list := range ordered {
		weight, ok := retrieverWeights[list.Retriever]
		if !ok {
			continue
		}
		for index, item := range list.Items {
			key := candidateIdentity(item)
			entry, exists := merged[key]
			if !exists {
				item.Score = 0
				item.RetrievalScore = 0
				entry = &fusedCandidate{candidate: item, trace: CandidateTrace{
					Handle: item.Handle, Path: item.Path, Symbol: item.Symbol,
					StartLine: item.StartLine, EndLine: item.EndLine,
				}}
				merged[key] = entry
			} else if item.Relation != "" && (entry.candidate.Relation == "" || strongerRelationContext(item.RelationContext, entry.candidate.RelationContext)) {
				entry.candidate.Relation = item.Relation
				entry.candidate.RelationContext = item.RelationContext
			}
			rank := index + 1
			contribution := weight / (rrfK + float64(rank))
			entry.candidate.RetrievalScore += contribution
			entry.trace.Contributions = append(entry.trace.Contributions, RetrievalContribution{Retriever: list.Retriever, Rank: rank, Weight: weight, Contribution: contribution})
			entry.trace.FusionScore += contribution
			entry.count++
		}
	}
	values := make([]*fusedCandidate, 0, len(merged))
	for _, value := range merged {
		sort.Slice(value.trace.Contributions, func(i, j int) bool {
			return value.trace.Contributions[i].Retriever < value.trace.Contributions[j].Retriever
		})
		values = append(values, value)
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if left.candidate.RetrievalScore != right.candidate.RetrievalScore {
			return left.candidate.RetrievalScore > right.candidate.RetrievalScore
		}
		if left.count != right.count {
			return left.count > right.count
		}
		if left.candidate.Confidence != right.candidate.Confidence {
			return left.candidate.Confidence > right.candidate.Confidence
		}
		if span := left.candidate.EndLine - left.candidate.StartLine; span != right.candidate.EndLine-right.candidate.StartLine {
			return span < right.candidate.EndLine-right.candidate.StartLine
		}
		if left.candidate.Path != right.candidate.Path {
			return left.candidate.Path < right.candidate.Path
		}
		if left.candidate.StartLine != right.candidate.StartLine {
			return left.candidate.StartLine < right.candidate.StartLine
		}
		return left.candidate.Handle < right.candidate.Handle
	})
	if limit <= 0 || limit > fusedLimit {
		limit = fusedLimit
	}
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]model.RankedCandidate, 0, len(values))
	traces := make([]CandidateTrace, 0, len(values))
	for _, value := range values {
		result = append(result, value.candidate)
		traces = append(traces, value.trace)
	}
	return result, traces
}

func candidateIdentity(candidate model.RankedCandidate) string {
	if candidate.Handle != "" {
		return "handle\x00" + candidate.Handle
	}
	return fmt.Sprintf("span\x00%s\x00%d\x00%d\x00%s\x00%s", candidate.Path, candidate.StartByte, candidate.EndByte, candidate.Kind, candidate.Symbol)
}
