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

func TestCPP20_CP10(t *testing.T) {
	t.Run("C11_wrong_qualified_call_indexed", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"main.cpp": "#include \"external.hpp\"\nnamespace other { void work() {} }\nvoid caller() { external::work(); }\n", "external.hpp": "namespace external { void work(); }\n"})
		by, rs := cppAppAuditGraph(t, s)
		var wrong, caller string
		for h, v := range by {
			if v.QualifiedName == "other::work" {
				wrong = h
			}
			if v.Name == "caller" {
				caller = h
			}
		}
		if wrong == "" || caller == "" {
			t.Fatal("fixture symbols missing")
		}
		for _, r := range rs {
			if r.FromHandle == caller && r.ToHandle == wrong && r.Kind == "calls" {
				t.Error("wrong namespace edge survives indexing")
			}
		}
		hits, e := s.Store.RelatedCandidateHits(context.Background(), []string{wrong}, "callers")
		if e != nil {
			t.Fatal(e)
		}
		for _, h := range hits {
			if h.Candidate.Path == "main.cpp" && h.Candidate.Symbol == "caller" {
				t.Errorf("wrong caller is exposed by expansion: %+v", h.Context)
			}
		}
		packet, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{wrong}, Relation: "callers", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range packet.Packet.Evidence {
			if v.Symbol == "caller" {
				t.Error("Evidence includes caller of a different namespace")
			}
		}
	})
}
