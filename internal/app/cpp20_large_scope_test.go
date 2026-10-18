package app

import (
	"context"
	"fmt"
	"github.com/focalspan/focalspan/internal/evidence"
	"path/filepath"
	"testing"
)

func TestCPP20_LargeScopeAndChangedLines(t *testing.T) {
	files := map[string]string{"caller.cpp": "#include \"f129.hpp\"\nnamespace n129{void Caller(){Target();}}\n"}
	for i := 0; i < 130; i++ {
		files[fmt.Sprintf("f%03d.hpp", i)] = fmt.Sprintf("namespace n%d{inline void Target(){}}\n", i)
	}
	ctx := context.Background()
	s, root := cppAppAuditService(t, files)
	by, _ := cppAppAuditGraph(t, s)
	var correct, wrong string
	for h, v := range by {
		if v.QualifiedName == "n129::Target" {
			correct = h
		}
		if v.QualifiedName == "n0::Target" {
			wrong = h
		}
	}
	if correct == "" || wrong == "" {
		t.Fatal("fixture missing")
	}
	for _, q := range []struct {
		h    string
		want bool
	}{{correct, true}, {wrong, false}} {
		hits, e := s.Store.RelatedCandidateHits(ctx, []string{q.h}, "callers")
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, h := range hits {
			if h.Candidate.Path == "caller.cpp" {
				found = true
			}
		}
		if found != q.want {
			t.Fatalf("scoped callers=%v want %v", found, q.want)
		}
	}
	r, e := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "Target", Paths: []string{"f129.hpp"}, NoUpdate: true, TokenBudget: 4000, Mode: evidence.ModeSource})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range r.Packet.Evidence {
		if v.Location.Path == "f129.hpp" && v.Source != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("path-scoped tail target omitted")
	}
	runAppGit(t, root, "init")
	runAppGit(t, root, "config", "user.name", "Test")
	runAppGit(t, root, "config", "user.email", "test@example.invalid")
	runAppGit(t, root, "add", "*.hpp", "caller.cpp")
	runAppGit(t, root, "commit", "-m", "base")
	writeAppFile(t, filepath.Join(root, "f129.hpp"), "namespace n129{inline void Target(){int changed=1;}}\n")
	r, e = s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "Target", ChangedOnly: true, TokenBudget: 4000, Mode: evidence.ModeSource})
	if e != nil {
		t.Fatal(e)
	}
	found = false
	for _, v := range r.Packet.Evidence {
		if v.Location.Path == "f129.hpp" && v.Source != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("changed-line tail target omitted")
	}
}
