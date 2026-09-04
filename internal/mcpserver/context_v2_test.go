package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newContextV2Session(t *testing.T, capabilities *mcp.ClientCapabilities) (*mcp.ClientSession, func()) {
	t.Helper()
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
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = server.Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "context-v2-test", Version: "1"}, &mcp.ClientOptions{Capabilities: capabilities})
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		_ = server.Close()
		t.Fatal(err)
	}
	return session, func() {
		_ = session.Close()
		cancel()
		_ = server.Close()
	}
}

func contextV2Capabilities(value any) *mcp.ClientCapabilities {
	return &mcp.ClientCapabilities{Extensions: map[string]any{contextEncodingExtension: value}}
}

func callPacket(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) ([]byte, evidence.Packet) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil || result.IsError || result.StructuredContent == nil {
		t.Fatalf("%s result=%+v err=%v", name, result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := evidence.DecodeContextV2(raw)
	if err != nil {
		t.Fatalf("%s v2=%s err=%v", name, raw, err)
	}
	content, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range packet.Evidence {
		if item.Source != "" && bytesContain(content, []byte(item.Source)) {
			t.Fatalf("%s source leaked into summary: %s", name, content)
		}
	}
	return raw, packet
}

func bytesContain(haystack, needle []byte) bool {
	return len(needle) > 0 && stringContains(string(haystack), string(needle))
}

func stringContains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}

func TestServerAdvertisesContextV2AndDefaultsToV1(t *testing.T) {
	session, closeSession := newContextV2Session(t, nil)
	defer closeSession()
	initialize := session.InitializeResult()
	if initialize == nil || initialize.Capabilities == nil {
		t.Fatalf("initialize=%+v", initialize)
	}
	settings, ok := initialize.Capabilities.Extensions[contextEncodingExtension].(map[string]any)
	if !ok || settings["default"] != evidence.SchemaContextV1 {
		t.Fatalf("extension=%#v", initialize.Capabilities.Extensions[contextEncodingExtension])
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "token_budget": 1200, "mode": "source"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if !bytesContain(raw, []byte(`"schema":"focalspan.context.v1"`)) {
		t.Fatalf("default structured=%s", raw)
	}
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "code_context" && tool.Name != "code_expand" && tool.Name != "code_impact" {
			continue
		}
		schema, err := json.Marshal(tool.OutputSchema)
		if err != nil || !bytesContain(schema, []byte(evidence.SchemaContextV1)) || !bytesContain(schema, []byte(evidence.SchemaContextV2)) || !bytesContain(schema, []byte(`"oneOf"`)) {
			t.Fatalf("%s output schema=%s err=%v", tool.Name, schema, err)
		}
	}
}

func TestServerProvidesCodeWorkInstructions(t *testing.T) {
	session, closeSession := newContextV2Session(t, nil)
	defer closeSession()

	initialize := session.InitializeResult()
	if initialize == nil {
		t.Fatal("initialize result is nil")
	}
	for _, want := range []string{
		"Use FocalSpan proactively",
		"code_context",
		"focused mode",
		"code_expand",
		"handles",
		"code_impact",
		"non-code tasks",
		"unavailable",
	} {
		if !strings.Contains(initialize.Instructions, want) {
			t.Fatalf("instructions=%q, missing %q", initialize.Instructions, want)
		}
	}
}

func TestMalformedContextV2CapabilityFallsBackToV1(t *testing.T) {
	for _, malformed := range []any{"focalspan.context.v2", map[string]any{"accept": "focalspan.context.v2"}, map[string]any{"accept": []any{17}}} {
		session, closeSession := newContextV2Session(t, contextV2Capabilities(malformed))
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "token_budget": 1200}})
		if err != nil || result.IsError {
			closeSession()
			t.Fatalf("capability=%#v result=%+v err=%v", malformed, result, err)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		closeSession()
		if !bytesContain(raw, []byte(`"schema":"focalspan.context.v1"`)) {
			t.Fatalf("capability=%#v structured=%s", malformed, raw)
		}
	}
}

func TestNegotiatedContextV2CoversAllEvidenceTools(t *testing.T) {
	capabilities := contextV2Capabilities(map[string]any{"accept": []string{evidence.SchemaContextV2}})
	session, closeSession := newContextV2Session(t, capabilities)
	defer closeSession()
	contextRaw, packet := callPacket(t, session, "code_context", map[string]any{"query": "ValidateToken", "token_budget": 1200, "mode": "source"})
	if !bytesContain(contextRaw, []byte(`"schema":"focalspan.context.v2"`)) || len(packet.Evidence) == 0 {
		t.Fatalf("context=%s packet=%+v", contextRaw, packet)
	}
	_, _ = callPacket(t, session, "code_expand", map[string]any{"handles": []string{packet.Evidence[0].Handle}, "relation": "self", "token_budget": 1200})
	_, _ = callPacket(t, session, "code_impact", map[string]any{"token_budget": 1200})
}
