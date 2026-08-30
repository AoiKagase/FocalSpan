package model

import "testing"

func TestRelationContextCarriesInternalProvenance(t *testing.T) {
	context := RelationContext{
		AnchorHandle: "target-chunk",
		Kind:         "calls",
		Direction:    RelationIncoming,
		Confidence:   0.95,
		Source:       "go-ast",
		Resolved:     true,
	}
	candidate := RankedCandidate{Handle: "caller-chunk", RelationContext: &context}
	hit := RelationHit{Candidate: candidate, Context: context}
	if hit.Context.Direction != RelationIncoming || hit.Candidate.RelationContext.AnchorHandle != "target-chunk" {
		t.Fatalf("relation provenance was not retained: %+v", hit)
	}
}
