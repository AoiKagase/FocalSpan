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

func cppAppAuditService(t *testing.T, files map[string]string) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	for path, src := range files {
		if e := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); e != nil {
			t.Fatal(e)
		}
		writeAppFile(t, filepath.Join(root, path), src)
	}
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	return s, root
}
func cppAppAuditGraph(t *testing.T, s *Service) (map[string]model.Symbol, []model.Relation) {
	t.Helper()
	syms, e := s.Store.Symbols(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	rs, e := s.Store.Relations(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	by := map[string]model.Symbol{}
	for _, v := range syms {
		by[v.Handle] = v
	}
	for _, r := range rs {
		t.Logf("EDGE %s:%s -> %s:%s unresolved=%s kind=%s confidence=%v source=%s", by[r.FromHandle].FilePath, by[r.FromHandle].QualifiedName, by[r.ToHandle].FilePath, by[r.ToHandle].QualifiedName, r.UnresolvedTo, r.Kind, r.Confidence, r.Source)
	}
	return by, rs
}
func TestCppFocusedAppAudit(t *testing.T) {
	t.Run("C18_include_owner", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"main.cpp": "#include \"dep.hpp\"\nvoid run() {}\n", "dep.hpp": "struct Item {};\nvoid helper();\n"})
		by, rs := cppAppAuditGraph(t, s)
		found := false
		var from string
		for _, r := range rs {
			if r.Kind == "imports" {
				from = r.FromHandle
				if by[r.ToHandle].Kind == "translation_unit" && by[r.ToHandle].FilePath == "dep.hpp" {
					found = true
				}
			}
		}
		if !found {
			t.Log("include remains unresolved; public fallback is evaluated below")
		}
		r, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{from}, Relation: "imports", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(r.Packet)
		t.Log("PACKET " + string(b))
		hit := false
		for _, v := range r.Packet.Evidence {
			if v.Location.Path == "dep.hpp" {
				hit = true
			}
		}
		if !hit {
			t.Error("include expansion omits header")
		}
	})
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
	t.Run("C18_include_impact", func(t *testing.T) {
		s, root := cppAppAuditService(t, map[string]string{"main.cpp": "#include \"dep.hpp\"\nvoid run() {}\n", "dep.hpp": "struct Item { int first; };\n"})
		runAppGit(t, root, "init")
		runAppGit(t, root, "config", "user.name", "Test")
		runAppGit(t, root, "config", "user.email", "test@example.invalid")
		runAppGit(t, root, "add", "main.cpp", "dep.hpp")
		runAppGit(t, root, "commit", "-m", "base")
		base := r09GitID(t, root)
		writeAppFile(t, filepath.Join(root, "dep.hpp"), "struct Item { int first; int second; };\n")
		runAppGit(t, root, "add", "dep.hpp")
		runAppGit(t, root, "commit", "-m", "change header")
		head := r09GitID(t, root)
		r, e := s.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		cppAppAuditGraph(t, s)
		b, _ := json.Marshal(r.Packet)
		t.Log("PACKET " + string(b))
		found := false
		for _, v := range r.Packet.Evidence {
			if v.Location.Path == "main.cpp" {
				found = true
			}
		}
		if !found {
			t.Error("changed header omitted direct includer from Impact")
		}
	})
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
	t.Run("C17_module_expansion", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"utilities.cppm": "export module utilities;\nexport void helper() {}\n", "main.cpp": "import utilities;\nvoid run() {}\n"})
		by, _ := cppAppAuditGraph(t, s)
		var from string
		for h, v := range by {
			if v.FilePath == "main.cpp" && v.Kind == "translation_unit" {
				from = h
			}
		}
		if from == "" {
			t.Fatal("importing unit missing")
		}
		p, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{from}, Relation: "imports", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(p.Packet)
		t.Log("PACKET " + string(b))
		found := false
		for _, v := range p.Packet.Evidence {
			if v.Location.Path == "utilities.cppm" {
				found = true
			}
		}
		if !found {
			t.Error("C++ module import expansion omits provider")
		}
	})
	t.Run("C03_custom_return_query", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"main.cpp": "struct Item {};\nItem make() { return {}; }\nint known_function() { return 1; }\n"})
		for _, name := range []string{"known_function", "make"} {
			p, e := s.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: name, TokenBudget: 4000, Mode: evidence.ModeSource})
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, v := range p.Packet.Evidence {
				if strings.Contains(v.Source, name+"(") {
					found = true
				}
			}
			if !found {
				t.Errorf("query %s did not return its source", name)
			}
		}
	})
}
