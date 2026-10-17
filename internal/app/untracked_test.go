package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/evidence"
	"path/filepath"
	"testing"
)

func TestUntrackedSourceInChangedQueryAndImpact(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	writeAppFile(t, filepath.Join(root, "base.go"), "package p\nfunc Base(){}\n")
	commitGitChangesFixture(t, root)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err = s.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	writeAppFile(t, filepath.Join(root, "new.go"), "package p\nfunc NewFeature(){}\n")
	query, err := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "NewFeature", ChangedOnly: true, Mode: evidence.ModeSource, TokenBudget: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Packet.Evidence) == 0 {
		t.Fatal("untracked changed-only query empty")
	}
	impact, err := s.ImpactEvidence(ctx, EvidenceImpactRequest{Mode: evidence.ModeSource, TokenBudget: 1200})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range impact.Packet.Evidence {
		if item.Location.Path == "new.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("untracked impact omitted: %+v", impact.Packet)
	}
	legacy, err := s.Impact(ctx, "", "", 1200)
	if err != nil || len(legacy.Items) == 0 {
		t.Fatalf("legacy impact empty: %+v %v", legacy, err)
	}
}
