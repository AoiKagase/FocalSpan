package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestQueryAutoIndexesAndHonorsBudget(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\n// ValidateToken rejects expired tokens.\nfunc ValidateToken(token string) error { return nil }\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	bundle, err := a.Query(context.Background(), QueryRequest{Query: "expired token ValidateToken", TokenBudget: 512, Mode: "source"})
	if err != nil || len(bundle.Items) == 0 || bundle.EstimatedTokens > 512 || bundle.Items[0].Path != "auth.go" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
	status, err := a.Status(context.Background())
	if err != nil || status.FileCount != 1 || status.ChunkCount == 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	baseline, err := a.BaselineTokens(context.Background(), "expired token ValidateToken")
	if err != nil || baseline <= 0 {
		t.Fatalf("baseline=%d err=%v", baseline, err)
	}
}

func TestExpandReturnsSelfAndEmptyForUnsupportedRelation(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	bundle, err := a.Query(context.Background(), QueryRequest{Query: "ValidateToken", TokenBudget: 512, Mode: "outline"})
	if err != nil || len(bundle.Items) == 0 {
		t.Fatalf("query=%+v err=%v", bundle, err)
	}
	expanded, err := a.Expand(context.Background(), []string{bundle.Items[0].Handle}, "self", 512)
	if err != nil || len(expanded.Items) != 1 {
		t.Fatalf("expanded=%+v err=%v", expanded, err)
	}
	empty, err := a.Expand(context.Background(), []string{bundle.Items[0].Handle}, "unknown", 512)
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("unknown relation=%+v err=%v", empty, err)
	}
}

func TestImpactReturnsChangedSpanWithSyntaxOnlyDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	runAppGit(t, root, "init")
	runAppGit(t, root, "add", "auth.go")
	runAppGit(t, root, "-c", "user.name=focalspan", "-c", "user.email=focalspan@example.invalid", "commit", "-m", "initial")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return ErrExpired }\n")
	bundle, err := a.Impact(context.Background(), "", "", 512)
	if err != nil || len(bundle.Items) == 0 || bundle.Items[0].Path != "auth.go" || len(bundle.Diagnostics) == 0 {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
}

func TestAppIndexesPHPAndIncFilesWithPHPExtractor(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "Service.php"), "<?php\nnamespace App;\nclass Service { public function run(): void {} }\n")
	writeAppFile(t, filepath.Join(root, "bootstrap.inc"), "<?php\nfunction bootstrap(): void {}\n")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	candidates, err := a.Store.AllCandidates(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	seenPaths := map[string]bool{}
	seenSymbols := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.Language != "php" {
			t.Fatalf("candidate language=%q: %+v", candidate.Language, candidate)
		}
		seenPaths[candidate.Path] = true
		seenSymbols[candidate.Symbol] = true
		if candidate.StartLine < 1 || candidate.EndLine < candidate.StartLine {
			t.Fatalf("invalid candidate span=%+v", candidate)
		}
	}
	if !seenPaths["Service.php"] || !seenPaths["bootstrap.inc"] || !seenSymbols["Service"] || !seenSymbols["bootstrap"] {
		t.Fatalf("PHP index paths=%v symbols=%v candidates=%+v", seenPaths, seenSymbols, candidates)
	}
}

func runAppGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}

func writeAppFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
