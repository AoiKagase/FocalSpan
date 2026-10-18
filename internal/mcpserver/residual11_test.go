package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestR09MCPImpactRelations(t *testing.T) {
	for _, compact := range []bool{false, true} {
		t.Run(map[bool]string{false: "v1", true: "v2"}[compact], func(t *testing.T) {
			root := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				out, e := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if e != nil {
					t.Fatalf("git %v: %v %s", args, e, out)
				}
			}
			write := func(p, text string) {
				t.Helper()
				if e := os.WriteFile(filepath.Join(root, p), []byte(text), 0600); e != nil {
					t.Fatal(e)
				}
			}
			git("init")
			git("config", "user.name", "Test")
			git("config", "user.email", "test@example.invalid")
			write("target.go", "package p\nfunc Target() int { return 1 }\n")
			write("caller.go", "package p\nfunc Caller() int { return Target() /* R09_CALLER_MARKER */ }\n")
			write("target_test.go", "package p\nimport \"testing\"\nfunc TestTarget(t *testing.T) { Target() }\n")
			git("add", ".")
			git("commit", "-m", "base")
			write("target.go", "package p\nfunc Target() int { return 2 }\n")
			service, e := app.New(root)
			if e != nil {
				t.Fatal(e)
			}
			server := New(service, false)
			st, ct := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _ = server.Run(ctx, st) }()
			var caps *mcp.ClientCapabilities
			if compact {
				caps = contextV2Capabilities(map[string]any{"accept": []string{evidence.SchemaContextV2}})
			}
			client := mcp.NewClient(&mcp.Implementation{Name: "impact-regression", Version: "1"}, &mcp.ClientOptions{Capabilities: caps})
			session, e := client.Connect(ctx, ct, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer session.Close()
			defer server.Close()
			result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_impact", Arguments: map[string]any{"token_budget": 4000}})
			if e != nil || result.IsError {
				t.Fatalf("result=%+v err=%v", result, e)
			}
			raw, e := json.Marshal(result.StructuredContent)
			if e != nil {
				t.Fatal(e)
			}
			var packet evidence.Packet
			var header struct {
				Schema string `json:"schema"`
			}
			if e = json.Unmarshal(raw, &header); e != nil {
				t.Fatal(e)
			}
			if header.Schema == evidence.SchemaContextV2 {
				packet, e = evidence.DecodeContextV2(raw)
			} else {
				e = json.Unmarshal(raw, &packet)
			}
			if e != nil {
				t.Fatal(e)
			}
			if e = evidence.Validate(packet); e != nil {
				t.Fatal(e)
			}
			paths := map[string]bool{}
			for _, item := range packet.Evidence {
				paths[item.Location.Path] = true
			}
			for _, p := range []string{"target.go", "caller.go", "target_test.go"} {
				if !paths[p] {
					t.Errorf("MCP impact omitted %s", p)
				}
			}
			summary := ""
			for _, c := range result.Content {
				if text, ok := c.(*mcp.TextContent); ok {
					summary += text.Text
				}
			}
			if strings.Contains(summary, "func Caller() int") {
				t.Error("source duplicated into text summary")
			}
			if strings.Count(string(raw), "func Caller() int") != 1 {
				t.Errorf("source-bearing signature count=%d", strings.Count(string(raw), "func Caller() int"))
			}
			if measured := budget.NewEstimator().Estimate(string(raw) + "\n" + summary); measured > 4000 {
				t.Errorf("actual wire budget exceeded: %d", measured)
			}
		})
	}
}
