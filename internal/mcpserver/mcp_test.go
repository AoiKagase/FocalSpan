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
	"github.com/focalspan/focalspan/internal/benchmark"
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
	for _, forbidden := range []string{`"score"`, `"weight"`, `"token_savings"`} {
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
	if err != nil || !strings.Contains(string(summary), "FocalSpan evidence:") || strings.Contains(string(summary), "ValidateToken") {
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
	impact, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_impact"})
	if err != nil || impact.IsError || impact.StructuredContent == nil {
		t.Fatalf("impact=%+v err=%v", impact, err)
	}
	impactJSON, _ := json.Marshal(impact.StructuredContent)
	if !strings.Contains(string(impactJSON), "syntax_only_impact") {
		t.Fatalf("impact structured=%s", impactJSON)
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

func TestPrivacyHistoricalSnapshotKeepsScopedTraceOutOfEvidenceAndMCP(t *testing.T) {
	const (
		sourceSentinel      = "TOP_SECRET_SOURCE_SENTINEL_9C42"
		usernameSentinel    = "PRIVATE_USERNAME_SENTINEL_9C42"
		environmentSentinel = "ENVIRONMENT_SENTINEL_9C42"
	)
	repository := filepath.Join(t.TempDir(), "ABSOLUTE_PATH_SENTINEL_9C42")
	sourcePath := filepath.Join(repository, "internal", "indexer", "indexer.go")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package indexer\n\n// " + sourceSentinel + " " + usernameSentinel + " " + environmentSentinel + "\nfunc Run() error { return nil }\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	runMCPGit(t, repository, "init", "-q")
	runMCPGit(t, repository, "add", "--", "internal/indexer/indexer.go")
	runMCPGit(t, repository, "-c", "user.name=FocalSpan Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", "base")

	snapshotRoot := filepath.Join(t.TempDir(), "snapshot")
	snapshot, err := benchmark.NewGitSnapshotter(benchmark.ExecCommandRunner{}).Materialize(context.Background(), "privacy-fixture", repository, "HEAD", snapshotRoot)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(snapshot.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Index(context.Background(), true); err != nil {
		_ = service.Close()
		t.Fatal(err)
	}

	request := app.EvidenceQueryRequest{Query: "internal/indexer/indexer.go Run", TokenBudget: 1200, NoUpdate: true}
	normal, err := service.QueryEvidence(context.Background(), request)
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	attributed, err := service.QueryEvidenceAttributed(context.Background(), request)
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	normalJSON, _ := json.Marshal(normal.Packet)
	attributedJSON, _ := json.Marshal(attributed.Compile.Packet)
	if !bytes.Equal(normalJSON, attributedJSON) {
		_ = service.Close()
		t.Fatalf("trace changed packet bytes\nnormal=%s\nattributed=%s", normalJSON, attributedJSON)
	}
	for _, forbiddenKey := range []string{`"trace"`, `"retriever"`, `"retrieved"`, `"lists"`, `"candidates"`, `"scoped_paths"`} {
		if bytes.Contains(normalJSON, []byte(forbiddenKey)) {
			_ = service.Close()
			t.Fatalf("normal Evidence exposed %s: %s", forbiddenKey, normalJSON)
		}
	}

	input := benchmark.AttributionInput{Indexed: []benchmark.AttributionIdentity{{Path: "internal/indexer/indexer.go", Symbol: "Run", Kind: "function"}}}
	for _, observation := range attributed.Trace.Retrieved {
		input.Retrieved = append(input.Retrieved, benchmark.AttributionObservation{
			AttributionIdentity: benchmark.AttributionIdentity{Path: observation.Path, Symbol: observation.Symbol, Kind: observation.Kind},
			Retriever:           string(observation.Retriever), Position: observation.Position,
			Relation: observation.Relation, RelationResolved: observation.RelationResolved,
		})
	}
	for _, candidate := range attributed.Trace.Candidates {
		input.Ranked = append(input.Ranked, benchmark.AttributionIdentity{Path: candidate.Path, Symbol: candidate.Symbol, Kind: candidate.Kind})
	}
	for _, item := range attributed.Compile.Packet.Evidence {
		input.Packed = append(input.Packed, benchmark.AttributionIdentity{Path: item.Location.Path, Symbol: item.Symbol, Kind: item.Kind})
	}
	labels, err := benchmark.AttributeLabels([]benchmark.AttributionExpectation{{Expectation: "required_symbol", Path: "internal/indexer/indexer.go", Symbol: "Run", Kind: "function"}}, input)
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	attributionJSON, err := benchmark.MarshalAttribution([]benchmark.AttributionResult{{Schema: benchmark.AttributionSchemaV1, CaseID: "privacy", RepositoryID: "privacy-fixture", Profile: "default", Budget: 1200, Labels: labels}})
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	if !bytes.Contains(attributionJSON, []byte(`"retriever": "path-scoped-symbol"`)) {
		_ = service.Close()
		t.Fatalf("scoped retriever absent from attribution: %s", attributionJSON)
	}

	server := New(service, false)
	defer server.Close()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "privacy-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	contextResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": request.Query, "token_budget": request.TokenBudget}})
	if err != nil || contextResult.IsError || contextResult.StructuredContent == nil {
		t.Fatalf("context=%+v err=%v", contextResult, err)
	}
	structured, err := json.Marshal(contextResult.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{sourceSentinel, usernameSentinel, environmentSentinel, repository, snapshot.Root} {
		if bytes.Contains(attributionJSON, []byte(forbidden)) || bytes.Contains(structured, []byte(forbidden)) {
			t.Fatalf("private scoped value %q leaked\nattribution=%s\nstructured=%s", forbidden, attributionJSON, structured)
		}
	}
	for _, forbiddenKey := range []string{`"trace"`, `"retriever"`, `"retrieved"`, `"lists"`, `"candidates"`, `"scoped_paths"`} {
		if bytes.Contains(structured, []byte(forbiddenKey)) {
			t.Fatalf("MCP structured output exposed %s: %s", forbiddenKey, structured)
		}
	}
}

func runMCPGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
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
