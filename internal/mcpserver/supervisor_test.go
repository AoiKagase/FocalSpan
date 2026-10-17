package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSupervisorWorkerHelper(t *testing.T) {
	if os.Getenv("FOCALSPAN_SUPERVISOR_HELPER") != "1" {
		return
	}
	if os.Getenv("FOCALSPAN_SUPERVISOR_VARIANT") == "exit" {
		os.Exit(17)
	}
	if os.Getenv("FOCALSPAN_SUPERVISOR_VARIANT") == "hang" {
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	root := os.Getenv("FOCALSPAN_SUPERVISOR_ROOT")
	if os.Getenv("FOCALSPAN_SUPERVISOR_VARIANT") == "wrong-root" {
		root = filepath.Join(root, "other")
	}
	service, err := app.New(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if os.Getenv("FOCALSPAN_SUPERVISOR_VARIANT") != "skip-index" {
		if _, err := service.Index(context.Background(), true); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	server := New(service, false)
	server.sdk.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			if params, ok := request.GetParams().(*mcp.CallToolParamsRaw); ok && method == "tools/call" && params.Name == "code_context" && strings.Contains(string(params.Arguments), "RPC_ERROR_MARKER") {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "worker RPC failure", Data: json.RawMessage(`{"field":"query"}`)}
			}
			return next(ctx, method, request)
		}
	})
	if os.Getenv("FOCALSPAN_SUPERVISOR_VARIANT") == "incompatible" {
		mcp.AddTool(server.sdk, &mcp.Tool{Name: "incompatible_tool"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return summaryResult("unexpected"), nil, nil
		})
	}
	err = server.Run(context.Background())
	server.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func supervisorFixture(t *testing.T, capabilities *mcp.ClientCapabilities) (*Supervisor, *mcp.ClientSession, *atomic.Int32, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\nfunc ValidateToken() string { return \"SUPERVISOR_SOURCE_MARKER\" }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	variant := new(atomic.Int32)
	factory := func() *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSupervisorWorkerHelper$")
		value := ""
		if variant.Load() == 1 {
			value = "incompatible"
		} else if variant.Load() == 2 {
			value = "exit"
		} else if variant.Load() == 3 {
			value = "hang"
		} else if variant.Load() == 4 {
			value = "wrong-root"
		} else if variant.Load() == 6 {
			value = "skip-index"
		}
		cmd.Env = append(os.Environ(), "FOCALSPAN_SUPERVISOR_HELPER=1", "FOCALSPAN_SUPERVISOR_ROOT="+root, "FOCALSPAN_SUPERVISOR_VARIANT="+value)
		cmd.Stderr = os.Stderr
		return cmd
	}
	ctx, cancel := context.WithCancel(context.Background())
	supervisor, err := NewSupervisor(ctx, factory)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- supervisor.Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "supervisor-test", Version: "1"}, &mcp.ClientOptions{Capabilities: capabilities})
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		supervisor.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		session.Close()
		cancel()
		<-done
		if err := supervisor.Close(); err != nil {
			t.Error(err)
		}
	})
	return supervisor, session, variant, root
}

func callSupervisor(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("%s: result=%+v err=%v", name, result, err)
	}
	return result
}

func TestSupervisorRestartsWorkerWithoutReplacingHostSession(t *testing.T) {
	supervisor, session, _, root := supervisorFixture(t, nil)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 6 {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	previous := supervisor.worker.command.Process.Pid
	restarted := callSupervisor(t, session, "code_restart_process", map[string]any{})
	raw, _ := json.Marshal(restarted.StructuredContent)
	var result WorkerRestartResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Restarted || result.PreviousPID != previous || result.WorkerPID == previous || result.WorkerPID <= 0 {
		t.Fatalf("restart=%s", raw)
	}
	status := callSupervisor(t, session, "code_status", map[string]any{})
	raw, _ = json.Marshal(status.StructuredContent)
	if !strings.Contains(string(raw), rootJSON(t, root)) {
		t.Fatalf("startup root changed: %s", raw)
	}
	callSupervisor(t, session, "code_restart", map[string]any{})
	if supervisor.worker.command.Process.Pid != result.WorkerPID {
		t.Fatal("code_restart replaced a process instead of reopening the service")
	}
	callSupervisor(t, session, "code_status", map[string]any{})
}

func rootJSON(t *testing.T, root string) string {
	t.Helper()
	b, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSupervisorPreservesEvidenceNegotiationAndWireFidelity(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprint(v2), func(t *testing.T) {
			var caps *mcp.ClientCapabilities
			if v2 {
				caps = contextV2Capabilities(map[string]any{"accept": []string{evidence.SchemaContextV2}})
			}
			_, session, _, _ := supervisorFixture(t, caps)
			for round := 0; round < 2; round++ {
				result := callSupervisor(t, session, "code_context", map[string]any{"query": "ValidateToken", "mode": "source", "token_budget": 1200})
				raw, _ := json.Marshal(result)
				structured, _ := json.Marshal(result.StructuredContent)
				content, _ := json.Marshal(result.Content)
				if strings.Count(string(raw), "SUPERVISOR_SOURCE_MARKER") != 1 || strings.Contains(string(content), "SUPERVISOR_SOURCE_MARKER") {
					t.Fatalf("source fidelity: %s", raw)
				}
				var packet evidence.Packet
				if v2 {
					var err error
					packet, err = evidence.DecodeContextV2(structured)
					if err != nil || len(packet.Evidence) == 0 {
						t.Fatalf("v2: %s %v", structured, err)
					}
				} else {
					if !strings.Contains(string(structured), evidence.SchemaContextV1) {
						t.Fatalf("v1 default changed: %s", structured)
					}
					if err := json.Unmarshal(structured, &packet); err != nil {
						t.Fatal(err)
					}
				}
				if err := evidence.Validate(packet); err != nil {
					t.Fatalf("forwarded Evidence invariant: %v", err)
				}
				summary := ""
				for _, entry := range result.Content {
					if text, ok := entry.(*mcp.TextContent); ok {
						summary += text.Text
					}
				}
				used := budget.NewEstimator().Estimate(string(structured) + "\n" + summary)
				if used > packet.Budget.Limit || used != packet.Budget.Used {
					t.Fatalf("forwarded wire budget: measured=%d declared=%d limit=%d", used, packet.Budget.Used, packet.Budget.Limit)
				}
				for _, forbidden := range []string{`"score"`, `"weight"`, `"token_savings"`} {
					if strings.Contains(string(raw), forbidden) {
						t.Fatalf("debug field leaked: %s", raw)
					}
				}
				if round == 0 {
					callSupervisor(t, session, "code_restart_process", map[string]any{})
				}
			}
		})
	}
}

func TestSupervisorFailedCandidatesKeepOldWorker(t *testing.T) {
	supervisor, session, variant, _ := supervisorFixture(t, nil)
	original := supervisor.worker.command.Process.Pid
	for _, value := range []int32{1, 2, 4} {
		variant.Store(value)
		if _, err := supervisor.Restart(context.Background()); err == nil {
			t.Fatal("failed candidate reported successful restart")
		}
		if supervisor.worker.command.Process.Pid != original {
			t.Fatal("old worker replaced on candidate failure")
		}
		callSupervisor(t, session, "code_status", map[string]any{})
	}
	variant.Store(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := supervisor.Restart(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled restart: %v", err)
	}
	if supervisor.worker.command.Process.Pid != original {
		t.Fatal("cancelled restart replaced worker")
	}
	callSupervisor(t, session, "code_restart_process", map[string]any{})
}

func TestSupervisorRejectsArbitraryRestartArguments(t *testing.T) {
	_, session, _, _ := supervisorFixture(t, nil)
	for _, args := range []map[string]any{{"executable": "other.exe"}, {"root": "elsewhere"}} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_restart_process", Arguments: args})
		if err == nil && !result.IsError {
			t.Fatalf("unexpected restart argument accepted: %v", args)
		}
	}
}

func TestSupervisorLegacyCapabilitiesAndErrorFidelity(t *testing.T) {
	supervisor, session, _, _ := supervisorFixture(t, contextV2Capabilities(map[string]any{"accept": []string{evidence.SchemaContextV2}}))
	var frontend *mcp.ServerSession
	for serverSession := range supervisor.sdk.Sessions() {
		frontend = serverSession
		break
	}
	if frontend == nil {
		t.Fatal("no frontend session")
	}
	// An older client supplies no per-request client-capability metadata.
	arguments := json.RawMessage(`{"query":"ValidateToken","mode":"source","token_budget":1200}`)
	result, err := supervisor.forward(context.Background(), &mcp.CallToolRequest{Session: frontend, Params: &mcp.CallToolParamsRaw{Name: "code_context", Arguments: arguments}})
	if err != nil || result.IsError {
		t.Fatalf("legacy forward: %+v %v", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if _, err := evidence.DecodeContextV2(raw); err != nil {
		t.Fatalf("lost initialize-time capabilities: %s %v", raw, err)
	}
	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "   "}})
	if err == nil && !invalid.IsError {
		t.Fatal("worker error turned into success")
	}
	invalid, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "root": "elsewhere"}})
	if err == nil && !invalid.IsError {
		t.Fatal("worker input schema rejection was lost")
	}
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "RPC_ERROR_MARKER"}})
	var protocolError *jsonrpc.Error
	if !errors.As(err, &protocolError) || protocolError.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("worker JSON-RPC error code changed: %+v, %v", protocolError, err)
	}
	data, _ := json.Marshal(protocolError.Data)
	if protocolError.Message != "worker RPC failure" || !strings.Contains(string(data), "query") {
		t.Fatalf("worker JSON-RPC error details changed: %+v", protocolError)
	}
}

func TestSupervisorRecoversCrashedWorkerAndReapsProcesses(t *testing.T) {
	supervisor, session, _, _ := supervisorFixture(t, nil)
	old := supervisor.worker
	if err := old.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "code_status", Arguments: map[string]any{}})
	if err == nil && !result.IsError {
		t.Fatal("crashed worker returned successful status")
	}
	if supervisor.worker != old {
		t.Fatal("failed call was replayed using an implicit restart")
	}
	callSupervisor(t, session, "code_restart_process", map[string]any{})
	if old.command.ProcessState == nil {
		t.Fatal("crashed process was not reaped")
	}
	callSupervisor(t, session, "code_status", map[string]any{})
	active := supervisor.worker
	if err := supervisor.Close(); err != nil {
		t.Fatal(err)
	}
	if active.command.ProcessState == nil {
		t.Fatal("worker process was not reaped on close")
	}
}

func TestSupervisorRestartRespectsInFlightCallsAndCancellation(t *testing.T) {
	supervisor, session, _, _ := supervisorFixture(t, nil)
	old := supervisor.worker
	supervisor.mu.RLock()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, err := supervisor.Restart(ctx)
	cancel()
	supervisor.mu.RUnlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("restart must not swap under an active call: %v", err)
	}
	if supervisor.worker != old {
		t.Fatal("cancelled drain replaced the worker")
	}
	callSupervisor(t, session, "code_status", map[string]any{})
	callSupervisor(t, session, "code_restart_process", map[string]any{})
	if old.command.ProcessState == nil {
		t.Fatal("replaced worker was not reaped")
	}
}

func TestSupervisorCancelledStartupReapsCandidate(t *testing.T) {
	supervisor, session, variant, _ := supervisorFixture(t, nil)
	old := supervisor.worker
	factory := supervisor.factory
	var candidate *exec.Cmd
	supervisor.factory = func() *exec.Cmd { candidate = factory(); return candidate }
	variant.Store(3)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err := supervisor.Restart(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startup cancellation: %v", err)
	}
	if candidate == nil || candidate.ProcessState == nil {
		t.Fatal("cancelled candidate was not reaped")
	}
	if supervisor.worker != old {
		t.Fatal("cancelled startup replaced the old worker")
	}
	callSupervisor(t, session, "code_status", map[string]any{})
}

func TestSupervisorDrainsRealForwardedCall(t *testing.T) {
	supervisor, session, variant, root := supervisorFixture(t, nil)
	held, err := indexlock.Acquire(context.Background(), filepath.Join(root, ".focalspan"), indexlock.Writer, indexlock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	callCtx, cancelCall := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCall()
	done := make(chan error, 1)
	go func() {
		result, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "code_context", Arguments: map[string]any{"query": "ValidateToken", "auto_update": true, "mode": "source", "token_budget": 1200}})
		if err == nil && result.IsError {
			err = fmt.Errorf("forwarded call failed: %+v", result)
		}
		done <- err
	}()
	// The external writer lock holds the actual worker request. Wait until
	// that forwarded request owns its read lease, without scheduling sleeps.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for supervisor.mu.TryLock() {
		supervisor.mu.Unlock()
		select {
		case err := <-done:
			t.Fatalf("forwarded call did not wait: %v", err)
		case <-deadline.C:
			t.Fatal("forwarded request did not enter supervisor")
		case <-ticker.C:
		}
	}
	old := supervisor.worker
	variant.Store(6) // Candidate opens the prepared index without a test-only rebuild.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, err = supervisor.Restart(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || supervisor.worker != old {
		t.Fatalf("restart interrupted active forwarded call: %v", err)
	}
	held.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	callSupervisor(t, session, "code_restart_process", map[string]any{})
	callSupervisor(t, session, "code_status", map[string]any{})
}
