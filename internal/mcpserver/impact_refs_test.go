package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCodeImpactExplicitRefReturnsGitError(t *testing.T) {
	session, closeSession := newContextV2Session(t, nil)
	defer closeSession()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "code_impact",
		Arguments: map[string]any{
			"base_ref": "refs/heads/focalspan-missing-ref",
			"head_ref": "HEAD",
		},
	})
	if err != nil {
		t.Fatalf("MCP transport failure: %v", err)
	}
	if !result.IsError {
		t.Fatalf("invalid Git reference returned success: %+v", result)
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && strings.Contains(text.Text, "read Git changes") {
			return
		}
	}
	t.Fatalf("tool error omitted Git failure context: %+v", result.Content)
}
