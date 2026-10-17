package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestImpactFreshnessHonorsAutoUpdateForWorkingTreeAndRefs(t *testing.T) {
	for _, refs := range []bool{false, true} {
		for _, auto := range []bool{false, true} {
			t.Run(map[bool]string{false: "worktree", true: "refs"}[refs]+map[bool]string{false: " manual", true: " automatic"}[auto], func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				runAppGit(t, root, "init")
				path := filepath.Join(root, "auth.go")
				writeAppFile(t, path, "package auth\nfunc ValidateToken() bool { return false }\n")
				commitGitChangesFixture(t, root)
				s, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				s.Config.AutoUpdateBeforeQuery = auto
				initial, err := s.Index(ctx, true)
				if err != nil {
					t.Fatal(err)
				}
				writeAppFile(t, path, "package auth\nfunc ValidateToken() bool { return true }\n")
				base, head := "", ""
				if refs {
					commitGitChangesFixture(t, root)
					base, head = "HEAD~1", "HEAD"
				}
				packet, err := s.ImpactEvidence(ctx, EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 1200, Mode: evidence.ModeSource})
				if !auto {
					if err == nil {
						t.Fatalf("stale impact succeeded: %+v", packet.Packet.Evidence)
					}
					revision, _ := s.Store.Meta(ctx, "last_revision")
					if revision != initial.Revision {
						t.Fatal("manual mode mutated index")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(packet.Packet.Evidence) != 1 || !strings.Contains(packet.Packet.Evidence[0].Source, "return true") {
					t.Fatalf("impact returned old source: %+v", packet.Packet.Evidence)
				}
				if err := evidence.Validate(packet.Packet); err != nil {
					t.Fatal(err)
				}
				if used := evidence.MeasureModelVisible(packet.Packet, nil); used != packet.Packet.Budget.Used || used > 1200 {
					t.Fatalf("wire=%d budget=%+v", used, packet.Packet.Budget)
				}
				legacy, err := s.Impact(ctx, base, head, 1200)
				if err != nil || len(legacy.Items) != 1 || !strings.Contains(legacy.Items[0].Content, "return true") {
					t.Fatalf("legacy=%+v err=%v", legacy, err)
				}
			})
		}
	}
}
