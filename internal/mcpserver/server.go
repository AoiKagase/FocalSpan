package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CodeContextInput struct {
	Query       string   `json:"query" jsonschema:"the natural-language code question"`
	TokenBudget int      `json:"token_budget,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	ChangedOnly bool     `json:"changed_only,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	AutoUpdate  *bool    `json:"auto_update,omitempty"`
}

type CodeExpandInput struct {
	Handles     []string `json:"handles" jsonschema:"chunk or symbol handles"`
	Relation    string   `json:"relation,omitempty"`
	TokenBudget int      `json:"token_budget,omitempty"`
}

type CodeImpactInput struct {
	BaseRef     string `json:"base_ref,omitempty"`
	HeadRef     string `json:"head_ref,omitempty"`
	TokenBudget int    `json:"token_budget,omitempty"`
}

type Server struct {
	service    *app.Service
	autoUpdate bool
	sdk        *mcp.Server
}

func New(service *app.Service, autoUpdate bool) *Server {
	s := &Server{service: service, autoUpdate: autoUpdate}
	s.sdk = mcp.NewServer(&mcp.Implementation{Name: "focalspan", Version: "0.1.0"}, nil)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_context", Description: "Return relevant repository source spans within a token budget."}, s.codeContext)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_expand", Description: "Expand stable code handles through a supported relation."}, s.codeExpand)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_impact", Description: "Return syntax-only impact candidates for Git changes."}, s.codeImpact)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_status", Description: "Return read-only index status."}, s.codeStatus)
	return s
}

func (s *Server) Run(ctx context.Context, transports ...mcp.Transport) error {
	if len(transports) > 0 && transports[0] != nil {
		return s.sdk.Run(ctx, transports[0])
	}
	return s.sdk.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) codeContext(ctx context.Context, _ *mcp.CallToolRequest, in CodeContextInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, model.ContextBundle{}, fmt.Errorf("query is required")
	}
	auto := s.autoUpdate
	if in.AutoUpdate != nil {
		auto = *in.AutoUpdate
	}
	bundle, err := s.service.Query(ctx, app.QueryRequest{Query: in.Query, TokenBudget: in.TokenBudget, Mode: in.Mode, ChangedOnly: in.ChangedOnly, Paths: in.Paths, NoUpdate: !auto})
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("query: %s; items: %d; estimated: %d", in.Query, len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeExpand(ctx context.Context, _ *mcp.CallToolRequest, in CodeExpandInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	if len(in.Handles) == 0 {
		return nil, model.ContextBundle{}, fmt.Errorf("handles are required")
	}
	bundle, err := s.service.Expand(ctx, in.Handles, in.Relation, in.TokenBudget)
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("relation: %s; items: %d; estimated: %d", in.Relation, len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeImpact(ctx context.Context, _ *mcp.CallToolRequest, in CodeImpactInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	bundle, err := s.service.Impact(ctx, in.BaseRef, in.HeadRef, in.TokenBudget)
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("impact candidates: %d; estimated: %d", len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeStatus(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, model.Status, error) {
	status, err := s.service.Status(ctx)
	if err != nil {
		return nil, model.Status{}, userError(err)
	}
	return summaryResult(fmt.Sprintf("files: %d; symbols: %d; chunks: %d", status.FileCount, status.SymbolCount, status.ChunkCount)), status, nil
}

func summaryResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func withTokenSavings(text string, bundle model.ContextBundle) string {
	if bundle.Savings == nil {
		return text
	}
	return fmt.Sprintf("%s; baseline: %d; saved: %d tokens (%.1f%%)", text, bundle.Savings.BaselineTokens, bundle.Savings.SavedTokens, bundle.Savings.SavingsRatio*100)
}

func userError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("request failed: %v", err)
}
