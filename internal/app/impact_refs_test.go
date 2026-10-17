package app

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpactExplicitRefs(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() bool { return false }\n")
	runAppGit(t, root, "add", "auth.go")
	runAppGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() bool { return true }\n")
	runAppGit(t, root, "add", "auth.go")
	runAppGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "change")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"legacy", "evidence"} {
		for _, tc := range []struct {
			name, base, head string
			wantError        bool
		}{
			{"invalid_base", "refs/heads/focalspan-missing-ref", "HEAD", true},
			{"invalid_head", "HEAD", "refs/heads/focalspan-missing-ref", true},
			{"valid_refs", "HEAD~1", "HEAD", false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				var count int
				var err error
				if api == "legacy" {
					result, callErr := service.Impact(context.Background(), tc.base, tc.head, 1200)
					count, err = len(result.Items), callErr
				} else {
					result, callErr := service.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: tc.base, HeadRef: tc.head, TokenBudget: 1200})
					count, err = len(result.Packet.Evidence), callErr
				}
				if tc.wantError {
					if err == nil {
						t.Fatal("invalid Git reference returned success")
					}
					var exitError *exec.ExitError
					if !errors.As(err, &exitError) || !strings.Contains(err.Error(), "read Git changes") {
						t.Fatalf("expected contextual Git error with original cause, got %v", err)
					}
				} else if err != nil || count == 0 {
					t.Fatalf("valid references: evidence count=%d err=%v", count, err)
				}
			})
		}
	}
}
