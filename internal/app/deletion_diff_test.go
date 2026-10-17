package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestDeletionOnlyDiffReturnsSurvivingFunction(t *testing.T) {
	const source = "func ValidateToken(token string) bool {\n\treturn token != \"\"\n}"
	for _, state := range []string{"unstaged", "staged", "committed"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			runAppGit(t, root, "init")
			path := filepath.Join(root, "auth.go")
			writeAppFile(t, path, "package auth\n\nfunc ValidateToken(token string) bool {\n\tif token == \"expired\" { return false }\n\treturn token != \"\"\n}\n")
			commitGitChangesFixture(t, root)
			writeAppFile(t, path, "package auth\n\n"+source+"\n")
			if state == "staged" {
				runAppGit(t, root, "add", "auth.go")
			} else if state == "committed" {
				commitGitChangesFixture(t, root)
			}
			service, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			if _, err := service.Index(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			base, head := "", ""
			if state == "committed" {
				base, head = "HEAD~1", "HEAD"
			}
			legacy, err := service.Impact(context.Background(), base, head, 1200)
			if err != nil || len(legacy.Items) != 1 || legacy.Items[0].Symbol != "ValidateToken" {
				t.Errorf("deletion-only legacy impact lost function: %+v err=%v", legacy, err)
			}
			impact, err := service.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 1200, Mode: evidence.ModeSource})
			if err != nil {
				t.Fatal(err)
			}
			assertDeletionEvidence(t, impact.Packet, source)
			if state != "committed" {
				query, err := service.Query(context.Background(), QueryRequest{Query: "ValidateToken", ChangedOnly: true, NoUpdate: true, TokenBudget: 1200})
				if err != nil || len(query.Items) != 1 || query.Items[0].Symbol != "ValidateToken" {
					t.Errorf("deletion-only changed query lost function: %+v err=%v", query, err)
				}
				packet, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", ChangedOnly: true, NoUpdate: true, TokenBudget: 1200, Mode: evidence.ModeSource})
				if err != nil {
					t.Fatal(err)
				}
				assertDeletionEvidence(t, packet.Packet, source)
			}
		})
	}
}

func TestDeletionOnlyDiffDeletedFileDoesNotReturnStaleSource(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	path := filepath.Join(root, "auth.go")
	writeAppFile(t, path, "package auth\n\nfunc ValidateToken() bool { return true }\n")
	commitGitChangesFixture(t, root)
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// Leave the old index intact to ensure a deleted file is not turned into
	// a current-source candidate merely by adding a boundary anchor.
	got, err := service.ImpactEvidence(context.Background(), EvidenceImpactRequest{TokenBudget: 1200, Mode: evidence.ModeSource})
	if err != nil || len(got.Packet.Evidence) != 0 {
		t.Fatalf("deleted file returned stale source: %+v err=%v", got.Packet, err)
	}
	if err := evidence.Validate(got.Packet); err != nil {
		t.Fatal(err)
	}
	if used := evidence.MeasureModelVisible(got.Packet, nil); used != got.Packet.Budget.Used || used > 1200 {
		t.Fatalf("deleted-file packet wire usage=%d budget=%+v", used, got.Packet.Budget)
	}
}

func assertDeletionEvidence(t *testing.T, packet evidence.Packet, source string) {
	t.Helper()
	if err := evidence.Validate(packet); err != nil {
		t.Error(err)
	}
	if used := evidence.MeasureModelVisible(packet, nil); used != packet.Budget.Used || used > 1200 {
		t.Errorf("deletion packet wire usage=%d budget=%+v", used, packet.Budget)
	}
	if len(packet.Evidence) != 1 {
		t.Errorf("deletion-only packet lost current function: %+v", packet)
		return
	}
	item := packet.Evidence[0]
	if item.Symbol != "ValidateToken" || item.Location.Path != "auth.go" || item.Location.Lines != [2]int{3, 5} || item.Fidelity != evidence.FidelityVerbatim || item.Source != source {
		t.Errorf("deletion packet source/position mismatch: %+v", item)
	}
}
