package app

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/evidence"
	"path/filepath"
	"testing"
)

func TestCPP20_Controls(t *testing.T) {
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
}
