package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStdoutContainsOnlyJSONRPCMessages(t *testing.T) {
	if os.Getenv("FOCALSPAN_MCP_STDOUT_HELPER") == "1" {
		root := os.Getenv("FOCALSPAN_MCP_ROOT")
		service, err := app.New(root)
		if err != nil {
			return
		}
		_ = New(service, false).Run(context.Background(), &mcp.StdioTransport{})
		_ = service.Close()
		return
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"stdout-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	command := exec.Command(os.Args[0], "-test.run=TestMCPStdoutContainsOnlyJSONRPCMessages")
	command.Env = append(os.Environ(), "FOCALSPAN_MCP_STDOUT_HELPER=1", "FOCALSPAN_MCP_ROOT="+root)
	command.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("stdio helper: %v stderr=%s", err, stderr.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "PASS" {
			continue
		}
		if !json.Valid([]byte(line)) {
			t.Fatalf("non-JSON stdout line: %q", line)
		}
		var message map[string]any
		if err := json.Unmarshal([]byte(line), &message); err != nil || message["jsonrpc"] != "2.0" {
			t.Fatalf("invalid JSON-RPC stdout line: %q", line)
		}
	}
	if len(strings.TrimSpace(string(output))) == 0 {
		t.Fatalf("stdio helper returned no protocol output; stderr=%s", stderr.String())
	}
}

func TestServerListsToolsAndHandlesStatusContextAndValidation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := New(service, false)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "focalspan-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	if got := names; len(got) != 4 || got[0] != "code_context" || got[1] != "code_expand" || got[2] != "code_impact" || got[3] != "code_status" {
		t.Fatalf("tools=%v", got)
	}
	status, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_status"})
	if err != nil || status.IsError || status.StructuredContent == nil {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	contextResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "token_budget": 512, "mode": "outline"}})
	if err != nil || contextResult.IsError || contextResult.StructuredContent == nil {
		t.Fatalf("context=%+v err=%v", contextResult, err)
	}
	invalid, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "   "}})
	if err != nil || !invalid.IsError {
		t.Fatalf("invalid=%+v err=%v", invalid, err)
	}
	canceled, cancelCall := context.WithCancel(ctx)
	cancelCall()
	if _, err := session.CallTool(canceled, &mcp.CallToolParams{Name: "code_status"}); err == nil {
		t.Fatal("expected cancellation")
	}
}
