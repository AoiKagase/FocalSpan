package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Supervisor owns the host-facing session. Only its worker executable is reloaded.
// The factory is fixed by startup configuration, never supplied by a tool caller.
type Supervisor struct {
	sdk       *mcp.Server
	factory   func() *exec.Cmd
	worker    *supervisedWorker
	contract  string
	mu        sync.RWMutex
	restartMu sync.Mutex
}

type supervisedWorker struct {
	command  *exec.Cmd
	session  *mcp.ClientSession
	tools    []*mcp.Tool
	contract string
}

type WorkerRestartResult struct {
	Restarted   bool `json:"restarted"`
	PreviousPID int  `json:"previous_pid"`
	WorkerPID   int  `json:"worker_pid"`
}

func NewSupervisor(ctx context.Context, factory func() *exec.Cmd) (*Supervisor, error) {
	if factory == nil {
		return nil, errors.New("worker command factory is required")
	}
	worker, err := startSupervisedWorker(ctx, factory)
	if err != nil {
		return nil, err
	}
	initialized := worker.session.InitializeResult()
	// The SDK can enrich advertised capabilities; do not mutate the captured
	// worker contract while constructing the host-facing SDK server.
	raw, err := json.Marshal(initialized.Capabilities)
	if err != nil {
		worker.close()
		return nil, err
	}
	var capabilities mcp.ServerCapabilities
	if err := json.Unmarshal(raw, &capabilities); err != nil {
		worker.close()
		return nil, err
	}
	s := &Supervisor{factory: factory, worker: worker, contract: worker.contract}
	s.sdk = mcp.NewServer(&mcp.Implementation{Name: "focalspan", Version: initialized.ServerInfo.Version}, &mcp.ServerOptions{Capabilities: &capabilities, Instructions: initialized.Instructions})
	for _, tool := range worker.tools {
		s.sdk.AddTool(tool, s.forward)
	}
	mcp.AddTool(s.sdk, &mcp.Tool{Name: "code_restart_process", Description: "Load the installed FocalSpan worker executable again without disconnecting this MCP session. Existing code_restart only reopens configuration and the index."}, s.restartTool)
	return s, nil
}

func startSupervisedWorker(ctx context.Context, factory func() *exec.Cmd) (*supervisedWorker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := factory()
	if command == nil {
		return nil, errors.New("worker factory returned no command")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "focalspan-supervisor", Version: "1"}, nil)
	session, err := client.Connect(startup, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize worker: %w", err)
	}
	worker := &supervisedWorker{command: command, session: session}
	fail := func(err error) (*supervisedWorker, error) { worker.close(); return nil, err }
	tools, err := session.ListTools(startup, nil)
	if err != nil {
		return fail(fmt.Errorf("list worker tools: %w", err))
	}
	worker.tools = tools.Tools
	sort.Slice(worker.tools, func(i, j int) bool { return worker.tools[i].Name < worker.tools[j].Name })
	expected := []string{"code_context", "code_expand", "code_impact", "code_restart", "code_status"}
	if len(worker.tools) != len(expected) || tools.NextCursor != "" {
		return fail(errors.New("incompatible worker tool catalog; reconnect the host for public API changes"))
	}
	for i, name := range expected {
		if worker.tools[i].Name != name {
			return fail(errors.New("incompatible worker tool names; reconnect the host"))
		}
	}
	initialized := session.InitializeResult()
	signature, err := json.Marshal(struct {
		Tools        []*mcp.Tool
		Capabilities *mcp.ServerCapabilities
		Instructions string
	}{worker.tools, initialized.Capabilities, initialized.Instructions})
	if err != nil {
		return fail(err)
	}
	if err := session.Ping(startup, nil); err != nil {
		return fail(fmt.Errorf("ping worker: %w", err))
	}
	status, err := session.CallTool(startup, &mcp.CallToolParams{Name: "code_status", Arguments: map[string]any{}})
	if err != nil {
		return fail(fmt.Errorf("worker health check: %w", err))
	}
	if status == nil || status.IsError {
		return fail(errors.New("worker health check returned an error"))
	}
	healthJSON, err := json.Marshal(status.StructuredContent)
	if err != nil {
		return fail(fmt.Errorf("encode worker health: %w", err))
	}
	var health struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal(healthJSON, &health); err != nil {
		return fail(fmt.Errorf("decode worker health: %w", err))
	}
	if health.Root == "" {
		return fail(errors.New("worker health check omitted repository root"))
	}
	// A compatible catalog is insufficient if a changed binary ignores --root.
	// Include the read-only health root in the immutable startup contract.
	worker.contract = string(signature) + "\n" + health.Root
	if err := startup.Err(); err != nil {
		return fail(err)
	}
	return worker, nil
}

func (w *supervisedWorker) close() error {
	if w == nil {
		return nil
	}
	return w.session.Close()
}

// waitSupervisorGate bounds admission, not the execution time of a tool call.
func waitSupervisorGate(ctx context.Context, try func() bool) error {
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := wait.Err(); err != nil {
			return err
		}
		if try() {
			return nil
		}
		select {
		case <-wait.Done():
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) forward(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := waitSupervisorGate(ctx, s.mu.TryRLock); err != nil {
		return nil, userError(err)
	}
	defer s.mu.RUnlock()
	if s.worker == nil {
		return nil, errors.New("FocalSpan supervisor is closed")
	}
	in := request.Params
	// Modern MCP requests carry capabilities per request. Older clients only
	// advertised them at initialize. Preserve either form explicitly, so the
	// worker negotiates Evidence encoding against the actual host client.
	meta := maps.Clone(in.Meta)
	if meta == nil {
		meta = map[string]any{}
	}
	if caps := request.ClientCapabilities(); caps != nil {
		meta[mcp.MetaKeyClientCapabilities] = struct {
			*mcp.ClientCapabilities
			Roots *mcp.RootCapabilities `json:"roots,omitempty"`
		}{caps, caps.RootsV2}
	}
	if info := request.ClientInfo(); info != nil {
		meta[mcp.MetaKeyClientInfo] = info
	}
	result, err := s.worker.session.CallTool(ctx, &mcp.CallToolParams{
		Meta: meta, Name: in.Name, Arguments: in.Arguments,
		InputResponses: in.InputResponses, RequestState: in.RequestState,
	})
	if err != nil {
		var protocolError *jsonrpc.Error
		if errors.As(err, &protocolError) {
			return nil, protocolError
		}
		return nil, fmt.Errorf("worker call failed; use code_restart_process to recover: %w", err)
	}
	return result, nil
}

func (s *Supervisor) Restart(ctx context.Context) (WorkerRestartResult, error) {
	if err := waitSupervisorGate(ctx, s.restartMu.TryLock); err != nil {
		return WorkerRestartResult{}, err
	}
	defer s.restartMu.Unlock()
	if err := waitSupervisorGate(ctx, s.mu.TryRLock); err != nil {
		return WorkerRestartResult{}, err
	}
	closed := s.worker == nil
	s.mu.RUnlock()
	if closed {
		return WorkerRestartResult{}, errors.New("FocalSpan supervisor is closed")
	}
	candidate, err := startSupervisedWorker(ctx, s.factory)
	if err != nil {
		return WorkerRestartResult{}, err
	}
	if candidate.contract != s.contract {
		candidate.close()
		return WorkerRestartResult{}, errors.New("incompatible worker contract; reconnect the host to load public API changes")
	}
	if err := waitSupervisorGate(ctx, s.mu.TryLock); err != nil {
		candidate.close()
		return WorkerRestartResult{}, err
	}
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		candidate.close()
		return WorkerRestartResult{}, err
	}
	previous := s.worker
	s.worker = candidate
	result := WorkerRestartResult{Restarted: true, PreviousPID: previous.command.Process.Pid, WorkerPID: candidate.command.Process.Pid}
	// CommandTransport closes stdin, waits, and terminates/kills an unresponsive
	// child. A previously crashed worker's exit status does not invalidate the
	// healthy replacement or trigger a replay of its interrupted calls.
	_ = previous.close()
	return result, nil
}

func (s *Supervisor) restartTool(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, WorkerRestartResult, error) {
	result, err := s.Restart(ctx)
	if err != nil {
		return nil, WorkerRestartResult{}, userError(err)
	}
	return summaryResult("FocalSpan worker process restarted; MCP session preserved"), result, nil
}

func (s *Supervisor) Run(ctx context.Context, transports ...mcp.Transport) error {
	if len(transports) > 0 && transports[0] != nil {
		return s.sdk.Run(ctx, transports[0])
	}
	return s.sdk.Run(ctx, &mcp.StdioTransport{})
}

func (s *Supervisor) Close() error {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	worker := s.worker
	s.worker = nil
	return worker.close()
}
