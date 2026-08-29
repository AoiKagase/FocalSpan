package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

type CodeRestartInput struct{}

type Server struct {
	service    *app.Service
	autoUpdate bool
	sdk        *mcp.Server
	mu         sync.RWMutex
	restartMu  sync.Mutex
}

func New(service *app.Service, autoUpdate bool) *Server {
	s := &Server{service: service, autoUpdate: autoUpdate}
	s.sdk = mcp.NewServer(&mcp.Implementation{Name: "focalspan", Version: "0.1.0"}, nil)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_context", Description: "Return relevant repository source spans within a token budget."}, s.codeContext)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_expand", Description: "Expand stable code handles through a supported relation."}, s.codeExpand)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_impact", Description: "Return syntax-only impact candidates for Git changes."}, s.codeImpact)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_restart", Description: "Reload FocalSpan configuration and reopen the repository index."}, s.codeRestart)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_status", Description: "Return read-only index status."}, s.codeStatus)
	return s
}

// Restart reloads the repository configuration and replaces the app service.
// Active MCP calls finish before the old service is closed.
func (s *Server) Restart(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.restartMu.Lock()
	defer s.restartMu.Unlock()

	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return fmt.Errorf("MCP server has no application service")
	}
	root := s.service.Root
	s.mu.RUnlock()

	replacement, err := app.New(root)
	if err != nil {
		return fmt.Errorf("restart service: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = replacement.Close()
		return err
	}

	s.mu.Lock()
	previous := s.service
	s.service = replacement
	closeErr := previous.Close()
	s.mu.Unlock()
	if closeErr != nil {
		return fmt.Errorf("close previous service during restart: %w", closeErr)
	}
	return nil
}

func (s *Server) Run(ctx context.Context, transports ...mcp.Transport) error {
	if len(transports) > 0 && transports[0] != nil {
		return s.sdk.Run(ctx, transports[0])
	}
	return s.sdk.Run(ctx, &mcp.StdioTransport{})
}

// Close releases the currently active application service. It is safe to call
// after Restart; the replacement service is the one that owns the open DB.
func (s *Server) Close() error {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	s.mu.Lock()
	service := s.service
	s.service = nil
	if service == nil {
		s.mu.Unlock()
		return nil
	}
	err := service.Close()
	s.mu.Unlock()
	return err
}

func (s *Server) codeContext(ctx context.Context, _ *mcp.CallToolRequest, in CodeContextInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, model.ContextBundle{}, fmt.Errorf("query is required")
	}
	auto := s.autoUpdate
	if in.AutoUpdate != nil {
		auto = *in.AutoUpdate
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, model.ContextBundle{}, fmt.Errorf("MCP server is closed")
	}
	bundle, err := s.service.Query(ctx, app.QueryRequest{Query: in.Query, TokenBudget: in.TokenBudget, Mode: in.Mode, ChangedOnly: in.ChangedOnly, Paths: in.Paths, NoUpdate: !auto})
	s.mu.RUnlock()
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("query: %s; items: %d; estimated: %d", in.Query, len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeExpand(ctx context.Context, _ *mcp.CallToolRequest, in CodeExpandInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	if len(in.Handles) == 0 {
		return nil, model.ContextBundle{}, fmt.Errorf("handles are required")
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, model.ContextBundle{}, fmt.Errorf("MCP server is closed")
	}
	bundle, err := s.service.Expand(ctx, in.Handles, in.Relation, in.TokenBudget)
	s.mu.RUnlock()
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("relation: %s; items: %d; estimated: %d", in.Relation, len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeImpact(ctx context.Context, _ *mcp.CallToolRequest, in CodeImpactInput) (*mcp.CallToolResult, model.ContextBundle, error) {
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, model.ContextBundle{}, fmt.Errorf("MCP server is closed")
	}
	bundle, err := s.service.Impact(ctx, in.BaseRef, in.HeadRef, in.TokenBudget)
	s.mu.RUnlock()
	if err != nil {
		return nil, model.ContextBundle{}, userError(err)
	}
	return summaryResult(withTokenSavings(fmt.Sprintf("impact candidates: %d; estimated: %d", len(bundle.Items), bundle.EstimatedTokens), bundle)), bundle, nil
}

func (s *Server) codeStatus(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, model.Status, error) {
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, model.Status{}, fmt.Errorf("MCP server is closed")
	}
	status, err := s.service.Status(ctx)
	s.mu.RUnlock()
	if err != nil {
		return nil, model.Status{}, userError(err)
	}
	return summaryResult(fmt.Sprintf("files: %d; symbols: %d; chunks: %d", status.FileCount, status.SymbolCount, status.ChunkCount)), status, nil
}

type CodeRestartResult struct {
	Restarted bool   `json:"restarted"`
	Root      string `json:"root"`
}

func (s *Server) codeRestart(ctx context.Context, _ *mcp.CallToolRequest, _ CodeRestartInput) (*mcp.CallToolResult, CodeRestartResult, error) {
	if err := s.Restart(ctx); err != nil {
		return nil, CodeRestartResult{}, userError(err)
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, CodeRestartResult{}, userError(fmt.Errorf("MCP server is closed"))
	}
	root := s.service.Root
	s.mu.RUnlock()
	return summaryResult("FocalSpan service restarted"), CodeRestartResult{Restarted: true, Root: root}, nil
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
