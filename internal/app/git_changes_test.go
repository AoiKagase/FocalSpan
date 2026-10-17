package app

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
)

func TestGitChangesImpactFiltersBeforeCandidateLimit(t *testing.T) {
	for _, tc := range []struct {
		name, path, prefix string
		unrelated          bool
	}{
		{"late_file", "z.go", "package auth\n\n", true},
		{"late_chunk", "a.go", "package auth\n\nfunc Earlier() bool { return false }\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			runAppGit(t, root, "init")
			if tc.unrelated {
				writeAppFile(t, filepath.Join(root, "a.go"), "package auth\n\nfunc Earlier() bool { return false }\n")
			}
			writeAppFile(t, filepath.Join(root, tc.path), tc.prefix+"func ChangedTarget() bool { return false }\n")
			commitGitChangesFixture(t, root)
			writeAppFile(t, filepath.Join(root, tc.path), tc.prefix+"func ChangedTarget() bool { return true }\n")
			cfg := config.Default()
			cfg.MaxCandidates = 1
			service, err := NewWithConfig(root, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			if _, err := service.Index(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			got, err := service.impactCandidates(context.Background(), "", "")
			if err != nil || len(got.Candidates) != 1 || got.Candidates[0].Symbol != "ChangedTarget" {
				t.Fatalf("changed target lost behind candidate limit: candidates=%+v err=%v", got.Candidates, err)
			}
			if tc.unrelated {
				writeAppFile(t, filepath.Join(root, "a.go"), "package auth\n\nfunc Earlier() bool { return true }\n")
				if _, err := service.Index(context.Background(), false); err != nil {
					t.Fatal(err)
				}
				first, err := service.impactCandidates(context.Background(), "", "")
				if err != nil || len(first.Candidates) != 1 {
					t.Fatalf("post-filter candidate cap not respected: %+v err=%v", first.Candidates, err)
				}
				second, err := service.impactCandidates(context.Background(), "", "")
				if err != nil || len(second.Candidates) != 1 || second.Candidates[0].Handle != first.Candidates[0].Handle {
					t.Fatalf("candidate limit is not deterministic: %+v err=%v", second.Candidates, err)
				}
			}
		})
	}
}

func TestGitChangesChangedOnlyIncludesStagedAndQuotedPaths(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		staged     bool
	}{
		{"staged", "auth.go", true},
		{"unicode_unstaged", "認証.go", false},
		{"unicode_staged", "認証.go", true},
		{"spaces_staged", "auth config.go", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			runAppGit(t, root, "init")
			writeAppFile(t, filepath.Join(root, tc.path), "package auth\n\nfunc ValidateToken() bool { return false }\n")
			commitGitChangesFixture(t, root)
			writeAppFile(t, filepath.Join(root, tc.path), "package auth\n\nfunc ValidateToken() bool { return true }\n")
			if tc.staged {
				runAppGit(t, root, "add", "--", tc.path)
			}
			service, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			if _, err := service.Index(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			req := QueryRequest{Query: "ValidateToken", ChangedOnly: true, NoUpdate: true, TokenBudget: 1200}
			legacy, err := service.Query(context.Background(), req)
			if err != nil || len(legacy.Items) == 0 {
				t.Errorf("legacy changed-only returned no source: %+v err=%v", legacy, err)
			}
			packet, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: req.Query, ChangedOnly: true, NoUpdate: true, TokenBudget: 1200})
			if err != nil || len(packet.Packet.Evidence) == 0 {
				t.Errorf("evidence changed-only returned no source: %+v err=%v", packet.Packet, err)
			}
			if tokens, err := service.BaselineTokensForRequest(context.Background(), req); err != nil || tokens == 0 {
				t.Errorf("changed-only baseline tokens=%d err=%v", tokens, err)
			}
		})
	}
}

func TestGitChangesChangedOnlyPropagatesGitFailure(t *testing.T) {
	service, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	// Keep the index readable so this test reaches the Git failure boundary.
	if _, err := service.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"legacy", "evidence", "baseline"} {
		t.Run(api, func(t *testing.T) {
			req := QueryRequest{Query: "ValidateToken", ChangedOnly: true, NoUpdate: true}
			var err error
			switch api {
			case "legacy":
				_, err = service.Query(ctx, req)
			case "evidence":
				_, err = service.QueryEvidence(ctx, EvidenceQueryRequest{Query: req.Query, ChangedOnly: true, NoUpdate: true})
			case "baseline":
				_, err = service.BaselineTokensForRequest(ctx, req)
			}
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) {
				t.Fatalf("expected original Git failure, got %v", err)
			}
		})
	}
}

func commitGitChangesFixture(t *testing.T, root string) {
	t.Helper()
	runAppGit(t, root, "add", ".")
	runAppGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "baseline")
}

func TestGitChangesChangedOnlyUsesWorktreeLineCoordinates(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	before := "package auth\n\nfunc ValidateToken() bool { return false }\n"
	writeAppFile(t, filepath.Join(root, "auth.go"), before)
	commitGitChangesFixture(t, root)
	after := "package auth\n\nfunc ValidateToken() bool { return true }\n"
	writeAppFile(t, filepath.Join(root, "auth.go"), after)
	runAppGit(t, root, "add", "auth.go")
	// Unstaged lines above the staged change move its current source location.
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\n// a\n// b\n// c\n// d\n\nfunc ValidateToken() bool { return true }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	got, err := service.changedRanges(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range got["auth.go"] {
		found = found || (r.Start <= 8 && r.End >= 8)
	}
	if !found {
		t.Fatalf("staged change missing at current worktree line 8: %+v", got)
	}
}

func TestGitChangesUnbornRepositoryIncludesCurrentStagedFile(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() bool { return false }\n")
	runAppGit(t, root, "add", "auth.go")
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\n// a\n// b\n// c\n// d\n\nfunc ValidateToken() bool { return true }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	legacy, err := service.Query(context.Background(), QueryRequest{Query: "ValidateToken", ChangedOnly: true, TokenBudget: 1200})
	if err != nil || len(legacy.Items) == 0 {
		t.Fatalf("unborn changed-only source=%+v err=%v", legacy, err)
	}
	packet, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", ChangedOnly: true, TokenBudget: 1200, NoUpdate: true})
	if err != nil || len(packet.Packet.Evidence) == 0 {
		t.Fatalf("unborn changed-only packet=%+v err=%v", packet.Packet, err)
	}
	impact, err := service.impactCandidates(context.Background(), "", "")
	if err != nil || len(impact.Candidates) == 0 {
		t.Fatalf("unborn impact candidates=%+v err=%v", impact.Candidates, err)
	}
}
