package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	if hasProcessArgument("mcp-stdout-helper") {
		root := os.Getenv("FOCALSPAN_MCP_ROOT")
		service, err := app.New(root)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return
		}
		if _, err := service.Index(context.Background(), true); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			_ = service.Close()
			return
		}
		_ = New(service, false).Run(context.Background(), &mcp.StdioTransport{})
		_ = service.Close()
		return
	}
	root := t.TempDir()
	const marker = "FOCALSPAN_UNIQUE_EVIDENCE_MARKER_9F2A"
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() string { return \""+marker+"\" }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestMCPStdoutContainsOnlyJSONRPCMessages$", "mcp-stdout-helper")
	command.Env = make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		upper := strings.ToUpper(value)
		if strings.HasPrefix(upper, "FOCALSPAN_MCP_STDOUT_HELPER=") || strings.HasPrefix(upper, "FOCALSPAN_MCP_ROOT=") {
			continue
		}
		command.Env = append(command.Env, value)
	}
	command.Env = append(command.Env, "FOCALSPAN_MCP_STDOUT_HELPER=1", "FOCALSPAN_MCP_ROOT="+root)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	scanner := bufio.NewScanner(stdout)
	lines := make([]string, 0, 3)
	write := func(value string) {
		t.Helper()
		if _, err := io.WriteString(stdin, value+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	readResponse := func(id float64) (string, map[string]any) {
		t.Helper()
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !json.Valid([]byte(line)) {
				t.Fatalf("non-JSON stdout line: %q", line)
			}
			lines = append(lines, line)
			var message map[string]any
			if err := json.Unmarshal([]byte(line), &message); err != nil || message["jsonrpc"] != "2.0" {
				t.Fatalf("invalid JSON-RPC stdout line: %q", line)
			}
			if message["id"] == id {
				return line, message
			}
		}
		t.Fatalf("response %v absent: scanner=%v stderr=%s", id, scanner.Err(), stderr.String())
		return "", nil
	}
	write(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"stdout-test","version":"1"}}}`)
	_, _ = readResponse(1)
	write(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	write(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	_, _ = readResponse(2)
	write(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"code_context","arguments":{"query":"ValidateToken","mode":"source","token_budget":1200}}}`)
	line, message := readResponse(3)
	if strings.Count(line, marker) != 1 {
		t.Fatalf("marker count=%d response=%s", strings.Count(line, marker), line)
	}
	result, _ := message["result"].(map[string]any)
	structured, _ := json.Marshal(result["structuredContent"])
	content, _ := json.Marshal(result["content"])
	if !strings.Contains(string(structured), marker) || strings.Contains(string(content), marker) {
		t.Fatalf("marker placement structured=%s content=%s", structured, content)
	}
	for _, forbidden := range []string{`"score"`, `"weight"`, `"token_savings"`, "benchmark-diagnosis", `"diagnostic_stage"`, `"path_hits"`, `"wire_bytes"`, `"envelope_metadata_bytes"`, `"signature_items"`} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("forbidden key %s in %s", forbidden, line)
		}
	}
	_ = stdin.Close()
	if err := command.Wait(); err != nil {
		t.Fatalf("stdio helper: %v stderr=%s", err, stderr.String())
	}
	for _, line := range lines {
		var message map[string]any
		if err := json.Unmarshal([]byte(line), &message); err != nil || message["jsonrpc"] != "2.0" {
			t.Fatalf("invalid JSON-RPC stdout line: %q", line)
		}
	}
	if strings.Contains(stderr.String(), marker) {
		t.Fatalf("source marker leaked to stderr: %s", stderr.String())
	}
}

func hasProcessArgument(want string) bool {
	for _, value := range os.Args {
		if value == want {
			return true
		}
	}
	return false
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
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	server := New(service, false)
	defer server.Close()
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
	if got := names; len(got) != 5 || got[0] != "code_context" || got[1] != "code_expand" || got[2] != "code_impact" || got[3] != "code_restart" || got[4] != "code_status" {
		t.Fatalf("tools=%v", got)
	}
	status, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_status"})
	if err != nil || status.IsError || status.StructuredContent == nil {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	restarted, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_restart"})
	if err != nil || restarted.IsError || restarted.StructuredContent == nil {
		t.Fatalf("restart=%+v err=%v", restarted, err)
	}
	restartedJSON, err := json.Marshal(restarted.StructuredContent)
	if err != nil || !strings.Contains(string(restartedJSON), `"restarted":true`) {
		t.Fatalf("restart structured=%s err=%v", restartedJSON, err)
	}
	contextResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "token_budget": 512, "mode": "outline"}})
	if err != nil || contextResult.IsError || contextResult.StructuredContent == nil {
		t.Fatalf("context=%+v err=%v", contextResult, err)
	}
	structured, err := json.Marshal(contextResult.StructuredContent)
	if err != nil || !strings.Contains(string(structured), `"schema":"focalspan.context.v1"`) || strings.Contains(string(structured), "token_savings") {
		t.Fatalf("context structured=%s err=%v", structured, err)
	}
	summary, err := json.Marshal(contextResult.Content)
	if err != nil || !strings.Contains(string(summary), "items=") || !strings.Contains(string(summary), " tokens=") || !strings.Contains(string(summary), " omitted=") || strings.Contains(string(summary), "FocalSpan evidence:") || strings.Contains(string(summary), "ValidateToken") {
		t.Fatalf("context summary=%s err=%v", summary, err)
	}
	var packet struct {
		Evidence []struct {
			Handle string `json:"handle"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(structured, &packet); err != nil || len(packet.Evidence) == 0 {
		t.Fatalf("packet=%+v err=%v", packet, err)
	}
	expanded, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_expand", Arguments: map[string]any{"handles": []string{packet.Evidence[0].Handle}, "relation": "self", "known_handles": []string{packet.Evidence[0].Handle}}})
	if err != nil || expanded.IsError || expanded.StructuredContent == nil {
		t.Fatalf("expand=%+v err=%v", expanded, err)
	}
	expandedJSON, _ := json.Marshal(expanded.StructuredContent)
	if !strings.Contains(string(expandedJSON), `"skipped_known":1`) {
		t.Fatalf("expand structured=%s", expandedJSON)
	}
	expandedSummary, err := json.Marshal(expanded.Content)
	if err != nil || !strings.Contains(string(expandedSummary), "items=") || strings.Contains(string(expandedSummary), "FocalSpan evidence:") {
		t.Fatalf("expand summary=%s err=%v", expandedSummary, err)
	}
	impact, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_impact"})
	if err != nil || impact.IsError || impact.StructuredContent == nil {
		t.Fatalf("impact=%+v err=%v", impact, err)
	}
	impactJSON, _ := json.Marshal(impact.StructuredContent)
	if !strings.Contains(string(impactJSON), "syntax_only_impact") {
		t.Fatalf("impact structured=%s", impactJSON)
	}
	impactSummary, err := json.Marshal(impact.Content)
	if err != nil || !strings.Contains(string(impactSummary), "items=") || strings.Contains(string(impactSummary), "FocalSpan evidence:") {
		t.Fatalf("impact summary=%s err=%v", impactSummary, err)
	}
	for _, args := range []map[string]any{
		{"query": "ValidateToken", "mode": "bad"},
		{"query": "ValidateToken", "known_handles": []string{"bad\u0000handle"}},
	} {
		invalidRequest, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_context", Arguments: args})
		if err != nil || !invalidRequest.IsError {
			t.Fatalf("invalid request=%+v err=%v args=%v", invalidRequest, err, args)
		}
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

func TestRestartToolReloadsTheService(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\n\nfunc ValidateToken() error { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".focalspan.json"), []byte(`{"index_directory":".focalspan-restarted"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := New(service, false)
	defer server.Close()
	if err := server.Restart(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	status, err := server.service.Status(context.Background())
	if err != nil {
		t.Fatalf("status after restart: %v", err)
	}
	if status.Root != root {
		t.Fatalf("root after restart=%q, want %q", status.Root, root)
	}
	if server.service.Config.IndexDirectory != ".focalspan-restarted" {
		t.Fatalf("config after restart=%q", server.service.Config.IndexDirectory)
	}
}
