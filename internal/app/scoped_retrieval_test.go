package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestScopedRetrievalFindsLateExactMatch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runAppGit(t, root, "init")
	for n := 0; n < 130; n++ {
		writeAppFile(t, filepath.Join(root, fmt.Sprintf("file%03d.go", n)), "package auth\nfunc ValidateToken() bool { return true }\n")
	}
	commitGitChangesFixture(t, root)
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err = service.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []bool{false, true} {
		req := EvidenceQueryRequest{Query: "ValidateToken", Paths: []string{"file129.go"}, NoUpdate: true, TokenBudget: 1200, Mode: evidence.ModeSource}
		if changed {
			writeAppFile(t, filepath.Join(root, "file129.go"), "package auth\nfunc ValidateToken() bool { return false }\n")
			if _, err = service.Index(ctx, false); err != nil {
				t.Fatal(err)
			}
			req.Paths = nil
			req.ChangedOnly = true
		}
		got, err := service.QueryEvidence(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Packet.Evidence) != 1 || got.Packet.Evidence[0].Location.Path != "file129.go" {
			t.Errorf("changed=%v: scoped match missing: %+v", changed, got.Packet.Evidence)
		}
		if err := evidence.Validate(got.Packet); err != nil {
			t.Fatal(err)
		}
		if used := evidence.MeasureModelVisible(got.Packet, nil); used != got.Packet.Budget.Used || used > 1200 {
			t.Fatalf("wire=%d budget=%+v", used, got.Packet.Budget)
		}
	}
}

func TestScopedRelationsKeepOutOfScopeAnchor(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() bool { return true }\n")
	writeAppFile(t, filepath.Join(root, "handler.go"), "package auth\nfunc Authenticate() bool { return ValidateToken() }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err = service.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	got, err := service.QueryEvidence(ctx, EvidenceQueryRequest{Query: "callers of ValidateToken", Paths: []string{"handler.go"}, NoUpdate: true, TokenBudget: 2000, Mode: evidence.ModeSource})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Packet.Evidence) == 0 {
		t.Fatal("caller lost because its anchor is outside path scope")
	}
	for _, item := range got.Packet.Evidence {
		if item.Location.Path != "handler.go" {
			t.Fatalf("out-of-scope candidate: %+v", item)
		}
	}
}
