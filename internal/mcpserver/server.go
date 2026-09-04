package mcpserver

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CodeContextInput struct {
	Query        string   `json:"query" jsonschema:"the natural-language code question"`
	TokenBudget  int      `json:"token_budget,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	ChangedOnly  bool     `json:"changed_only,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	AutoUpdate   *bool    `json:"auto_update,omitempty"`
	KnownHandles []string `json:"known_handles,omitempty" jsonschema:"stable handles already present in the model context and not to be retransmitted"`
}

type CodeExpandInput struct {
	Handles      []string `json:"handles" jsonschema:"chunk or symbol handles"`
	Relation     string   `json:"relation,omitempty"`
	TokenBudget  int      `json:"token_budget,omitempty"`
	KnownHandles []string `json:"known_handles,omitempty" jsonschema:"stable handles already present in the model context and not to be retransmitted"`
}

type CodeImpactInput struct {
	BaseRef      string   `json:"base_ref,omitempty"`
	HeadRef      string   `json:"head_ref,omitempty"`
	TokenBudget  int      `json:"token_budget,omitempty"`
	KnownHandles []string `json:"known_handles,omitempty" jsonschema:"stable handles already present in the model context and not to be retransmitted"`
}

type CodeRestartInput struct{}

const contextEncodingExtension = "io.focalspan/context-encoding"

const serverInstructions = "Use FocalSpan proactively for code-related work without waiting for the user to mention it. For repository investigation, code changes, reviews, and debugging, call code_context early in focused mode, before broad file reads or ad-hoc search. Use code_expand with handles returned by FocalSpan for follow-up context, and use code_impact for Git change impact. Use code_status when index health is relevant. Do not call FocalSpan for non-code tasks. Obey user and higher-priority instructions; if FocalSpan is unavailable, stale, empty, or errors, continue with direct repository evidence and report the limitation."

type Server struct {
	service    *app.Service
	autoUpdate bool
	sdk        *mcp.Server
	mu         sync.RWMutex
	restartMu  sync.Mutex
}

func New(service *app.Service, autoUpdate bool) *Server {
	s := &Server{service: service, autoUpdate: autoUpdate}
	capabilities := &mcp.ServerCapabilities{Logging: &mcp.LoggingCapabilities{}}
	capabilities.AddExtension(contextEncodingExtension, map[string]any{
		"schemas": []string{evidence.SchemaContextV1, evidence.SchemaContextV2},
		"default": evidence.SchemaContextV1,
	})
	s.sdk = mcp.NewServer(&mcp.Implementation{Name: "focalspan", Version: "0.4.0"}, &mcp.ServerOptions{Capabilities: capabilities, Instructions: serverInstructions})
	outputSchema := contextOutputSchema()
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_context", Description: "Find and return a role-labeled packet of repository evidence for a code question. Call this before broad file reads; use handles and next actions for follow-up expansion.", OutputSchema: outputSchema}, s.codeContext)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_expand", Description: "Return new evidence related to stable handles. Pass known_handles to avoid retransmitting context already present in the conversation.", OutputSchema: outputSchema}, s.codeExpand)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_impact", Description: "Return syntax-based changed spans, dependents, and related tests for Git changes within a token budget. Results may omit unresolved dynamic relationships.", OutputSchema: outputSchema}, s.codeImpact)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_restart", Description: "Reload FocalSpan configuration and reopen the repository index."}, s.codeRestart)
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_status", Description: "Return read-only index status."}, s.codeStatus)
	return s
}

func contextOutputSchema() any {
	v1, err := jsonschema.ForType(reflect.TypeFor[evidence.Packet](), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("infer context v1 output schema: %v", err))
	}
	row := func(size int) map[string]any {
		return map[string]any{"type": "array", "minItems": size, "maxItems": size}
	}
	v2 := map[string]any{
		"type":                 "object",
		"required":             []string{"schema", "m", "b", "e"},
		"additionalProperties": false,
		"properties": map[string]any{
			"schema": map[string]any{"const": evidence.SchemaContextV2},
			"r":      map[string]any{"type": "string"},
			"i":      map[string]any{"type": "string"},
			"m":      map[string]any{"type": "string", "enum": []string{string(evidence.ModeOutline), string(evidence.ModeFocused), string(evidence.ModeSource)}},
			"b": map[string]any{
				"type": "array", "minItems": 4, "maxItems": 4,
				"prefixItems": []any{map[string]any{"type": "integer"}, map[string]any{"type": "integer"}, map[string]any{"type": "boolean"}, map[string]any{"type": "integer"}},
			},
			"p": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"e": map[string]any{"type": "array", "items": row(8)},
			"x": map[string]any{"type": "array", "items": row(4)},
			"l": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"n": map[string]any{"type": "array", "items": row(3)},
			"k": map[string]any{"type": "integer", "minimum": 0},
		},
	}
	v1Discriminator := map[string]any{
		"type":       "object",
		"required":   []string{"schema"},
		"properties": map[string]any{"schema": map[string]any{"const": evidence.SchemaContextV1}},
	}
	return map[string]any{"oneOf": []any{map[string]any{"allOf": []any{v1, v1Discriminator}}, v2}}
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

func (s *Server) codeContext(ctx context.Context, request *mcp.CallToolRequest, in CodeContextInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	mode, err := normalizeMCPMode(in.Mode)
	if err != nil {
		return nil, nil, err
	}
	known, err := evidence.NormalizeKnownHandles(in.KnownHandles)
	if err != nil {
		return nil, nil, userError(err)
	}
	auto := s.autoUpdate
	if in.AutoUpdate != nil {
		auto = *in.AutoUpdate
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, nil, fmt.Errorf("MCP server is closed")
	}
	result, err := s.service.QueryEvidence(ctx, app.EvidenceQueryRequest{Query: in.Query, TokenBudget: in.TokenBudget, Mode: mode, ChangedOnly: in.ChangedOnly, Paths: in.Paths, NoUpdate: !auto, KnownHandles: known})
	s.mu.RUnlock()
	if err != nil {
		return nil, nil, userError(err)
	}
	output, summaryPacket := negotiatedContextOutput(request, result.Packet)
	return summaryResult(evidence.Summary(summaryPacket)), output, nil
}

func (s *Server) codeExpand(ctx context.Context, request *mcp.CallToolRequest, in CodeExpandInput) (*mcp.CallToolResult, any, error) {
	if len(in.Handles) == 0 {
		return nil, nil, fmt.Errorf("handles are required")
	}
	known, err := evidence.NormalizeKnownHandles(in.KnownHandles)
	if err != nil {
		return nil, nil, userError(err)
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, nil, fmt.Errorf("MCP server is closed")
	}
	result, err := s.service.ExpandEvidence(ctx, app.EvidenceExpandRequest{Handles: in.Handles, Relation: in.Relation, TokenBudget: in.TokenBudget, KnownHandles: known})
	s.mu.RUnlock()
	if err != nil {
		return nil, nil, userError(err)
	}
	output, summaryPacket := negotiatedContextOutput(request, result.Packet)
	return summaryResult(evidence.Summary(summaryPacket)), output, nil
}

func (s *Server) codeImpact(ctx context.Context, request *mcp.CallToolRequest, in CodeImpactInput) (*mcp.CallToolResult, any, error) {
	known, err := evidence.NormalizeKnownHandles(in.KnownHandles)
	if err != nil {
		return nil, nil, userError(err)
	}
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, nil, fmt.Errorf("MCP server is closed")
	}
	result, err := s.service.ImpactEvidence(ctx, app.EvidenceImpactRequest{BaseRef: in.BaseRef, HeadRef: in.HeadRef, TokenBudget: in.TokenBudget, KnownHandles: known})
	s.mu.RUnlock()
	if err != nil {
		return nil, nil, userError(err)
	}
	output, summaryPacket := negotiatedContextOutput(request, result.Packet)
	return summaryResult(evidence.Summary(summaryPacket)), output, nil
}

func negotiatedContextOutput(request *mcp.CallToolRequest, packet evidence.Packet) (any, evidence.Packet) {
	if !acceptsContextV2(request) {
		return packet, packet
	}
	raw, decoded, preferred, err := evidence.PreferContextV2(packet, nil)
	if err != nil || !preferred {
		return packet, packet
	}
	return raw, decoded
}

func acceptsContextV2(request *mcp.CallToolRequest) bool {
	if request == nil || request.ClientCapabilities() == nil {
		return false
	}
	settings, ok := request.ClientCapabilities().Extensions[contextEncodingExtension].(map[string]any)
	if !ok {
		return false
	}
	values, ok := settings["accept"]
	if !ok {
		return false
	}
	switch accepted := values.(type) {
	case []string:
		for _, value := range accepted {
			if value == evidence.SchemaContextV2 {
				return true
			}
		}
	case []any:
		for _, entry := range accepted {
			value, ok := entry.(string)
			if !ok {
				return false
			}
			if value == evidence.SchemaContextV2 {
				return true
			}
		}
	}
	return false
}

func normalizeMCPMode(value string) (evidence.Mode, error) {
	if value == "" {
		return evidence.ModeFocused, nil
	}
	mode := evidence.Mode(value)
	if mode != evidence.ModeOutline && mode != evidence.ModeFocused && mode != evidence.ModeSource {
		return "", fmt.Errorf("mode must be outline, focused, or source")
	}
	return mode, nil
}

func (s *Server) codeStatus(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, model.HealthStatus, error) {
	s.mu.RLock()
	if s.service == nil {
		s.mu.RUnlock()
		return nil, model.HealthStatus{}, fmt.Errorf("MCP server is closed")
	}
	status, err := s.service.Health(ctx)
	s.mu.RUnlock()
	if err != nil {
		return nil, model.HealthStatus{}, userError(err)
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
