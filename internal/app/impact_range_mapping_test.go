package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/evidence"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoricalImpactMapsCurrentWorktreeLines(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	path := filepath.Join(root, "auth.go")
	writeAppFile(t, path, "package p\n\nfunc ValidateToken() bool {\n return false\n}\n")
	commitGitChangesFixture(t, root)
	current := "package p\n\nfunc ValidateToken() bool {\n return true\n}\n"
	writeAppFile(t, path, current)
	commitGitChangesFixture(t, root)
	writeAppFile(t, path, strings.Repeat("// padding\n", 200)+current)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: "HEAD~1", HeadRef: "HEAD", Mode: evidence.ModeSource, TokenBudget: 1200})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range got.Packet.Evidence {
		if item.Location.Path == "auth.go" && item.Location.Lines[0] >= 203 && strings.Contains(item.Source, "return true") {
			found = true
		}
	}
	if !found {
		t.Fatalf("historical range not mapped: %+v", got.Packet)
	}
	if evidence.MeasureModelVisible(got.Packet, nil) > 1200 {
		t.Fatal("wire budget")
	}
	if err := evidence.Validate(got.Packet); err != nil {
		t.Fatal(err)
	}
}
