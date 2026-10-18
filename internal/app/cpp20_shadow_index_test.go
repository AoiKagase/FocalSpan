package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestCPP20_ShadowNoFalseEdgeAfterIndex(t *testing.T) {
	for _, source := range []string{"void target(){} void caller(void(*target)()){target();}", "void target(){} namespace n{struct Object{void target();};} void caller(n::Object& object){object.target();}"} {
		s, _ := cppAppAuditService(t, map[string]string{"a.cpp": source})
		by, rs := cppAppAuditGraph(t, s)
		var target string
		for h, v := range by {
			if v.QualifiedName == "target" {
				target = h
			}
		}
		for _, r := range rs {
			if r.ToHandle == target && by[r.FromHandle].Name == "caller" && r.Kind == "calls" {
				t.Fatal("false edge survived index")
			}
		}
		hits, e := s.Store.RelatedCandidateHits(context.Background(), []string{target}, "callers")
		if e != nil {
			t.Fatal(e)
		}
		for _, h := range hits {
			if h.Candidate.Symbol == "caller" {
				t.Fatal("false unresolved caller exposed")
			}
		}
		p, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{target}, Relation: "callers", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range p.Packet.Evidence {
			if v.Symbol == "caller" {
				t.Fatal("false caller in Evidence")
			}
		}
	}
}
