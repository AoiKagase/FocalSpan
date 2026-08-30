package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServeUsesProcessWorkingDirectory(t *testing.T) {
	if os.Getenv("FOCALSPAN_CWD_SERVE_HELPER") == "1" {
		os.Exit(Run(context.Background(), []string{"serve", "--auto-update=false"}, os.Stdout, os.Stderr))
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "init")
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"setup", "--root", root, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("setup code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"update", "--root", root, "--quiet"}, &stdout, &stderr); code != 0 {
		t.Fatalf("update code=%d stderr=%s", code, stderr.String())
	}
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestMCPServeUsesProcessWorkingDirectory$")
	command.Dir = nested
	command.Env = append(os.Environ(), "FOCALSPAN_CWD_SERVE_HELPER=1")
	client := mcp.NewClient(&mcp.Implementation{Name: "cwd-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	status, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_status"})
	if err != nil || status.IsError {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	encodedStatus, _ := json.Marshal(status.StructuredContent)
	if !strings.Contains(string(encodedStatus), `"root":`+quoteJSON(t, root)) {
		t.Fatalf("status root=%s want=%q", encodedStatus, root)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "mode": "source", "token_budget": 800},
	})
	if err != nil || result.IsError {
		t.Fatalf("context=%+v err=%v", result, err)
	}
	encodedContext, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(encodedContext), "ValidateToken") {
		t.Fatalf("context=%s", encodedContext)
	}
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
