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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeSupervisorReloadsReplacedExecutableOnSameSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "focalspan.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/focalspan")
	build.Env = append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build test binary: %v: %s", err, out)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\nfunc ValidateToken() string {return \"FIXED_STARTUP_ROOT\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "init")
	var out, stderr bytes.Buffer
	if code := Run(ctx, []string{"update", "--root", root, "--quiet"}, &out, &stderr); code != 0 {
		t.Fatalf("prepare index: %s", stderr.String())
	}
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "serve", "--auto-update=false")
	command.Dir = nested
	command.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "launcher-integration", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %+v %v", name, result, err)
		}
		return result
	}
	before := call("code_status", map[string]any{})
	raw, _ := json.Marshal(before.StructuredContent)
	if !strings.Contains(string(raw), quoteJSON(t, root)) {
		t.Fatalf("wrong startup root: %s", raw)
	}
	// Replacing an installed Windows executable requires a rename while the
	// supervisor/worker still map the old file. The captured startup path must
	// continue to select the replacement rather than following the renamed file.
	backup := binary + ".previous"
	if err := os.Rename(binary, backup); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	restarted := call("code_restart_process", map[string]any{})
	raw, _ = json.Marshal(restarted.StructuredContent)
	var result struct {
		Restarted bool `json:"restarted"`
		Previous  int  `json:"previous_pid"`
		Worker    int  `json:"worker_pid"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Restarted || result.Previous == result.Worker || result.Worker <= 0 {
		t.Fatalf("restart: %s", raw)
	}
	call("code_restart", map[string]any{})
	packet := call("code_context", map[string]any{"query": "ValidateToken", "mode": "source", "token_budget": 1200})
	raw, _ = json.Marshal(packet)
	if strings.Count(string(raw), "FIXED_STARTUP_ROOT") != 1 {
		t.Fatalf("source fidelity after executable replacement: %s", raw)
	}
}
