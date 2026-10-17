package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type f05CancelOnTemp struct {
	context.Context
	path   string
	cancel context.CancelFunc
}

func (c *f05CancelOnTemp) Err() error {
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(c.path), ".config.toml.focalspan-*"))
	if len(matches) > 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestF05ForceUpdatePreservesOriginalOnFailure(t *testing.T) {
	for _, mode := range []string{"replace", "cancel", "external edit", "missing table"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			path := filepath.Join(home, "config.toml")
			original := "# keep heading\r\ntitle = \"\"\"\r\n[mcp_servers.fake]\r\n\"\"\"\r\n[mcp_servers.\"focalspan-test\"]\r\ncommand = \"other\"\r\nargs = [\"serve\"]\r\nnotes = \"\"\"\r\n[mcp_servers.fake_inside_string]\r\n\"\"\"\r\n# keep comment\r\n[mcp_servers.keep]\r\ncommand = \"keep\"\r\nargs = []\r\n[mcp_servers.\"focalspan-test\".env]\r\nAUDIT = \"not-a-secret\"\r\n"
			if mode == "missing table" {
				original = "unrelated = true\n"
			}
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			command := testExecutable(t, "focalspan.exe")
			runner := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
				switch args[1] {
				case "get":
					return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", []string{"serve"})}, nil
				case "remove":
					_ = os.WriteFile(path, nil, 0600)
					return CommandResult{}, nil
				default:
					return CommandResult{ExitCode: 1}, errors.New("add fails")
				}
			}}
			var ctx context.Context = context.Background()
			var cancel context.CancelFunc
			var concurrent *auditConcurrentEditContext
			if mode == "cancel" {
				ctx, cancel = context.WithCancel(context.Background())
				defer cancel()
				ctx = &f05CancelOnTemp{Context: ctx, path: path, cancel: cancel}
			}
			if mode == "external edit" {
				concurrent = &auditConcurrentEditContext{Context: ctx, path: path}
				ctx = concurrent
			}
			result, err := NewService(runner).Install(ctx, Request{Scope: ScopeUser, Name: "focalspan-test", Command: command, Force: true})
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			switch mode {
			case "replace":
				if err != nil {
					t.Fatalf("atomic force update failed: %v", err)
				}
				if result.Action != "updated" {
					t.Fatalf("action %q", result.Action)
				}
				var doc map[string]any
				if err := toml.Unmarshal(got, &doc); err != nil {
					t.Fatal(err)
				}
				server, ok := findServer(doc, "focalspan-test")
				if !ok || server["command"] != command {
					t.Fatalf("new registration missing: %v", server)
				}
				if _, ok := server["env"]; ok {
					t.Error("old target environment override survived force replacement")
				}
				if !bytes.Contains(got, []byte("[mcp_servers.keep]\r\ncommand = \"keep\"\r\nargs = []\r\n")) || !bytes.Contains(got, []byte("# keep comment\r\n")) {
					t.Error("unrelated bytes changed")
				}
				if bytes.Contains(bytes.ReplaceAll(got, []byte("\r\n"), nil), []byte("\n")) {
					t.Error("CRLF was not preserved")
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) || string(got) != original {
					t.Fatalf("cancel must preserve old bytes: %v, unchanged=%v", err, string(got) == original)
				}
			case "external edit":
				var conflict *ConfigConflictError
				if !errors.As(err, &conflict) || !concurrent.edited || string(got) != "user_setting = \"concurrent edit\"\n" {
					t.Fatalf("external edit not preserved: %v edited=%v bytes=%q", err, concurrent.edited, got)
				}
			case "missing table":
				var conflict *ConfigConflictError
				if !errors.As(err, &conflict) || string(got) != original {
					t.Fatalf("unproven CLI registration modified: %v", err)
				}
			}
			matches, _ := filepath.Glob(filepath.Join(home, ".config.toml.focalspan-*"))
			if len(matches) != 0 {
				t.Errorf("owned temporary files remain: %v", matches)
			}
			if mode == "replace" && len(runner.calls) != 1 {
				t.Errorf("force update must not have remove/add gap: %+v", runner.calls)
			}
			if strings.Contains(result.State, "not-a-secret") {
				t.Error("secret-like value leaked")
			}
		})
	}
}

func TestF05DryRunAndLockWait(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "config.toml")
	original := []byte("[mcp_servers.focalspan-test]\ncommand = \"other\"\nargs = [\"serve\"]\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", []string{"serve"})}, nil
	}}
	service := NewService(fake)
	req := Request{Scope: ScopeUser, Name: "focalspan-test", Command: testExecutable(t, "focalspan.exe"), Force: true, DryRun: true}
	result, err := service.Install(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) || result.Block == "" || len(result.Argv) != 0 || result.Action != "update" {
		t.Fatalf("dry-run mutated or omitted preview: %+v", result)
	}
	lease, err := indexlock.AcquireConfiguration(context.Background(), path+".focalspan.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	req.DryRun = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := service.Install(ctx, req); !errors.Is(err, indexlock.ErrBusy) {
		t.Fatalf("configuration lock was bypassed: %v", err)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Error("lock wait changed configuration")
	}
}

func TestF10GlobalExecutionDrift(t *testing.T) {
	command := testExecutable(t, "focalspan.exe")
	for _, tc := range []struct {
		name, level, key string
		value            any
		drift            bool
	}{
		{"default", "", "", nil, false},
		{"disabled", "root", "enabled", false, true},
		{"fixed cwd", "transport", "cwd", "/other", true},
		{"env", "transport", "env", map[string]string{"AUDIT": "fake"}, true},
		{"env vars", "transport", "env_vars", []string{"AUDIT"}, true},
		{"startup timeout", "root", "startup_timeout_sec", 1, true},
		{"tool timeout", "root", "tool_timeout_sec", 2, true},
		{"tool allow list", "root", "enabled_tools", []string{}, true},
		{"tool deny list", "root", "disabled_tools", []string{"code_context"}, true},
		{"empty env", "transport", "env", map[string]string{}, false},
		{"null timeout", "root", "startup_timeout_sec", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(currentCodexRegistrationJSON(t, command, []string{"serve"}), &doc); err != nil {
				t.Fatal(err)
			}
			if tc.level == "root" {
				doc[tc.key] = tc.value
			} else if tc.level == "transport" {
				doc["transport"].(map[string]any)[tc.key] = tc.value
			}
			payload, _ := json.Marshal(doc)
			runner := &fakeCommandRunner{fn: func(context.Context, string, []string) (CommandResult, error) {
				return CommandResult{Stdout: payload}, nil
			}}
			service := NewService(runner)
			req := Request{Scope: ScopeUser, Name: "focalspan-test", Command: command}
			status, err := service.Status(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if tc.drift {
				if status.State == StateManagedMatch || status.Matches {
					t.Errorf("execution drift reported match: %+v", status)
				}
			} else if status.State != StateManagedMatch {
				t.Errorf("normal registration not matched: %+v", status)
			}
			if tc.key == "cwd" && status.RootMode == "runtime_cwd" {
				t.Error("fixed cwd reported as runtime cwd")
			}
			result, err := service.Install(context.Background(), req)
			if tc.drift {
				if err == nil || result.Action == "unchanged" {
					t.Errorf("drift install accepted unchanged: %+v %v", result, err)
				}
			} else if err != nil || result.Action != "unchanged" {
				t.Errorf("normal install failed: %+v %v", result, err)
			}
		})
	}
}

func TestF10InvalidExecutionMetadata(t *testing.T) {
	for _, setting := range []string{`"enabled":null`, `"env":"bad"`, `"cwd":12`, `"enabled_tools":false`} {
		payload := []byte(`{"command":"focalspan","args":["serve"],` + setting + `}`)
		if _, err := parseUserRegistration(payload); err == nil {
			t.Errorf("invalid execution setting accepted: %s", setting)
		}
	}
}
func TestF10ForceRestoresDefaultExecution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "config.toml")
	command := testExecutable(t, "focalspan.exe")
	encoded, _ := json.Marshal(command)
	original := "[mcp_servers.focalspan-test]\ncommand = " + string(encoded) + "\nargs = [\"serve\"]\nenabled = false\ncwd = \"/other\"\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCommandRunner{fn: func(context.Context, string, []string) (CommandResult, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return CommandResult{}, err
		}
		var doc map[string]any
		if err := toml.Unmarshal(content, &doc); err != nil {
			return CommandResult{}, err
		}
		server, _ := findServer(doc, "focalspan-test")
		data, err := json.Marshal(map[string]any{"enabled": server["enabled"], "transport": map[string]any{"type": "stdio", "command": server["command"], "args": server["args"], "cwd": server["cwd"], "env": server["env"]}})
		return CommandResult{Stdout: data}, err
	}}
	service := NewService(fake)
	req := Request{Scope: ScopeUser, Name: "focalspan-test", Command: command, Force: true}
	before, err := service.Status(context.Background(), req)
	if err != nil || before.Matches {
		t.Fatalf("drift not visible: %+v %v", before, err)
	}
	if _, err := service.Install(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	after, err := service.Status(context.Background(), req)
	if err != nil || !after.Matches || after.RootMode != "runtime_cwd" {
		t.Fatalf("force did not restore defaults: %+v %v", after, err)
	}
}
