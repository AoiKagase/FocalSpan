package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newCPP20Session(t *testing.T, capabilities *mcp.ClientCapabilities) (*mcp.ClientSession, func()) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.cpp"), []byte("namespace n{struct Item{};Item make(){return {};}}\r\n"), 0o600); err != nil {
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

func TestCPP20_MCPSourceAndWire(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		var caps *mcp.ClientCapabilities
		if v2 {
			caps = contextV2Capabilities(map[string]any{"accept": []string{evidence.SchemaContextV2}})
		}
		session, closeSession := newCPP20Session(t, caps)
		t.Cleanup(closeSession)
		r, e := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "make", "mode": "source", "token_budget": 1600}})
		if e != nil || r.IsError {
			t.Fatal(e, r)
		}
		raw, e := json.Marshal(r.StructuredContent)
		if e != nil {
			t.Fatal(e)
		}
		var p evidence.Packet
		if v2 {
			p, e = evidence.DecodeContextV2(raw)
		} else {
			e = json.Unmarshal(raw, &p)
		}
		if e != nil {
			t.Fatal(e)
		}
		if e = evidence.Validate(p); e != nil {
			t.Fatal(e)
		}
		if n := budget.NewEstimator().Estimate(string(raw) + "\n" + evidence.Summary(p)); n != p.Budget.Used || n > p.Budget.Limit {
			t.Fatal("wire budget", n, p.Budget)
		}
		found := false
		text, _ := json.Marshal(r.Content)
		for _, v := range p.Evidence {
			if strings.Contains(v.Source, "Item make()") {
				found = true
			}
			if v.Source != "" && bytes.Contains(text, []byte(v.Source)) {
				t.Fatal("source repeated in summary")
			}
		}
		if !found {
			t.Fatal("C++ source omitted")
		}
		if len(raw) == 0 {
			t.Fatal("empty wire")
		}
	}
}
