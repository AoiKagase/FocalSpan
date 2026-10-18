package app

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var _ = os.WriteFile
var _ = filepath.Join
var _ = config.Default
var _ = model.Symbol{}
var _ = reflect.DeepEqual
var _ = strings.Contains
var _ = json.Marshal
var _ = evidence.ModeSource

func TestCPP20_CP19(t *testing.T) {
	t.Run("C21_cross_file_namespace_after_addition", func(t *testing.T) {
		s, root := cppAppAuditService(t, map[string]string{"a.hpp": "namespace n { inline void Target() {} }\n", "b.cpp": "#include \"a.hpp\"\nnamespace n { void Caller() { Target(); } }\n"})
		initialBy, initial := cppAppAuditGraph(t, s)
		correct := false
		for _, r := range initial {
			if r.Kind == "calls" && initialBy[r.FromHandle].QualifiedName == "n::Caller" && initialBy[r.ToHandle].QualifiedName == "n::Target" {
				correct = true
			}
		}
		if !correct {
			t.Fatal("initial control did not link n::Caller to n::Target")
		}
		t.Log("CONTROL initial namespace link PASS")
		writeAppFile(t, filepath.Join(root, "unrelated.hpp"), "namespace m { inline void Target() {} }\n")
		if _, e := s.Index(context.Background(), false); e != nil {
			t.Fatal(e)
		}
		by, rs := cppAppAuditGraph(t, s)
		var wrong string
		for h, v := range by {
			if v.QualifiedName == "m::Target" {
				wrong = h
			}
		}
		if wrong == "" {
			t.Fatal("control target missing")
		}
		hits, e := s.Store.RelatedCandidateHits(context.Background(), []string{wrong}, "callers")
		if e != nil {
			t.Fatal(e)
		}
		for _, h := range hits {
			if h.Candidate.Path == "b.cpp" && (h.Candidate.Symbol == "Caller" || h.Candidate.Symbol == "n::Caller") {
				t.Errorf("unrelated namespace gets caller: %+v", h.Context)
			}
		}
		p, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{wrong}, Relation: "callers", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range p.Packet.Evidence {
			if v.Location.Path == "b.cpp" && (v.Symbol == "Caller" || v.Symbol == "n::Caller") {
				t.Error("Evidence returns caller from different namespace")
			}
		}
		if _, e = s.Index(context.Background(), true); e != nil {
			t.Fatal(e)
		}
		_, full := cppAppAuditGraph(t, s)
		if !reflect.DeepEqual(rs, full) {
			t.Error("incremental graph differs from full rebuild")
		} else {
			t.Log("CONTROL incremental/full graph parity PASS")
		}
	})
}
