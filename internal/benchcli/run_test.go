package benchcli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunValidatePublicSchemaFixture(t *testing.T) {
	var stdout, stderr bytes.Buffer
	suite := filepath.Join("..", "..", "testdata", "benchmark", "schema-valid.json")
	code := Run(context.Background(), []string{"validate", "--suite", suite}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "cases: 1") || !strings.Contains(stdout.String(), "invalid: 0") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunBenchmarkAndCompare(t *testing.T) {
	repository := t.TempDir()
	runTestGit(t, repository, "init", "-q")
	runTestGit(t, repository, "config", "user.name", "Benchmark Test")
	runTestGit(t, repository, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(repository, "service.go"), []byte("package fixture\nfunc ValidateToken() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "add", "--", "service.go")
	runTestGit(t, repository, "commit", "-q", "-m", "base")
	base := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repository, "service.go"), []byte("package fixture\nfunc ValidateToken() {}\nfunc Caller(){ValidateToken()}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "add", "--", "service.go")
	runTestGit(t, repository, "commit", "-q", "-m", "target")
	target := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))
	suite := filepath.Join(t.TempDir(), "suite.json")
	data := `{"schema":"focalspan.benchmark.v1","name":"small","cases":[{"id":"case","repository":"fixture","base_ref":"` + base + `","target_ref":"` + target + `","query":"ValidateToken definition","budgets":[512],"required_paths":["service.go"]}]}`
	if err := os.WriteFile(suite, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "result.json")
	md := filepath.Join(t.TempDir(), "result.md")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"run", "--suite", suite, "--repo", "fixture=" + repository, "--profile", "fts-evidence-focused", "--repeat", "2", "--json-out", out, "--markdown-out", md}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"compare", "--baseline", out, "--candidate", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("compare code=%d stderr=%q", code, stderr.String())
	}
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestRepositoryMapExplicitOverridesRegistry(t *testing.T) {
	root, registryRepo, explicitRepo := t.TempDir(), t.TempDir(), t.TempDir()
	registry := filepath.Join(t.TempDir(), "registry.json")
	data := `{"schema":"focalspan.benchmark.v1","repositories":{"private":"` + strings.ReplaceAll(registryRepo, "\\", "\\\\") + `"}}`
	if err := os.WriteFile(registry, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := repositoryMap(root, registry, []string{"private=" + explicitRepo})
	if err != nil || got["private"] != filepath.Clean(explicitRepo) {
		t.Fatalf("map=%v err=%v", got, err)
	}
}

func TestRunScaffoldContainsNoSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"scaffold", "--repository", "self", "--base", "HEAD~1", "--target", "HEAD", "--query", "where is benchmark CLI assembled?"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "candidate_paths") || strings.Contains(stdout.String(), "package main") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunRejectsUnknownCommandOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr); code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
