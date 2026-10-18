package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"sort"
)

// Only changed symbols and their file owners seed this traversal. Returned
// dependents are never used as another frontier, even if the graph has cycles.
func (s *Service) impactRelationCandidates(ctx context.Context, changed []model.RankedCandidate) ([]model.RankedCandidate, error) {
	handles := make([]string, 0, len(changed))
	seenHandles := map[string]bool{}
	paths := map[string]bool{}
	add := func(handle string) {
		if handle != "" && !seenHandles[handle] {
			seenHandles[handle] = true
			handles = append(handles, handle)
		}
	}
	for _, candidate := range changed {
		add(candidate.Handle)
		paths[candidate.Path] = true
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)
	fileHandles := append([]string(nil), handles...)
	for _, path := range orderedPaths {
		owners, err := s.Store.FileOwnerHandles(ctx, path)
		if err != nil {
			return nil, err
		}
		fileHandles = append(fileHandles, owners...)
	}
	found := map[string]model.RankedCandidate{}
	for _, relation := range []string{"tests", "callers", "imports", "exports", "references"} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		anchors := handles
		if relation == "imports" || relation == "exports" || relation == "references" {
			anchors = fileHandles
		}
		hits, err := s.Store.RelatedCandidateHits(ctx, anchors, relation)
		if err != nil {
			return nil, err
		}
		for _, hit := range hits {
			if hit.Context.Direction != model.RelationIncoming {
				continue
			}
			candidate := hit.Candidate
			candidate.Changed = false
			candidate.Relation = hit.Context.Kind
			provenance := hit.Context
			candidate.RelationContext = &provenance
			candidate.Score += 12
			candidate.Reasons = append(candidate.Reasons, model.ScoreReason{Code: "relation-" + relation})
			previous, exists := found[candidate.Handle]
			if !exists || preferImpactRelation(candidate.RelationContext, previous.RelationContext) {
				found[candidate.Handle] = candidate
			}
		}
	}
	result := make([]model.RankedCandidate, 0, len(found))
	for _, candidate := range found {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.Handle < b.Handle
	})
	return result, nil
}

func preferImpactRelation(candidate, current *model.RelationContext) bool {
	if current == nil {
		return true
	}
	if candidate.Resolved != current.Resolved {
		return candidate.Resolved
	}
	if candidate.Confidence != current.Confidence {
		return candidate.Confidence > current.Confidence
	}
	if (candidate.Kind == "tests") != (current.Kind == "tests") {
		return candidate.Kind == "tests"
	}
	if candidate.AnchorHandle != current.AnchorHandle {
		return candidate.AnchorHandle < current.AnchorHandle
	}
	return candidate.Kind < current.Kind
}
