package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeCommandCall struct {
	Name string
	Args []string
}

func TestCodexRunnerUsesExecRunnerForDiscoveredCommand(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	runner, command, err := NewService(nil).codexRunner(Request{CodexCommand: executable})
	if err != nil {
		t.Fatalf("codexRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("codexRunner() runner = nil, want executable command runner")
	}
	if command == "" {
		t.Fatal("codexRunner() command is empty")
	}
}

type fakeCommandRunner struct {
	calls []fakeCommandCall
	fn    func(context.Context, string, []string) (CommandResult, error)
}

func (f *fakeCommandRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	f.calls = append(f.calls, fakeCommandCall{Name: name, Args: append([]string(nil), args...)})
	return f.fn(ctx, name, args)
}

func TestUserInstallUsesSeparatedCodexArgv(t *testing.T) {
	command := testExecutable(t, "focal span.exe")
	fake := &fakeCommandRunner{}
	fake.fn = func(_ context.Context, _ string, args []string) (CommandResult, error) {
		if args[0] == "mcp" && args[1] == "get" {
			return CommandResult{ExitCode: 1}, errors.New("not found")
		}
		return CommandResult{}, nil
	}
	service := NewService(fake)
	req := Request{Scope: ScopeUser, Root: `C:\repo with spaces`, Name: "focalspan-test", Command: command, NoAutoUpdate: true}
	result, err := service.Install(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "create" {
		t.Fatalf("action=%q", result.Action)
	}
	want := []string{"mcp", "add", "focalspan-test", "--", command, "serve", "--root", `C:\repo with spaces`, "--no-auto-update"}
	if len(fake.calls) != 2 || fake.calls[0].Name != "codex" || !reflect.DeepEqual(fake.calls[1].Args, want) {
		t.Fatalf("calls=%+v", fake.calls)
	}
	for _, call := range fake.calls {
		if call.Name == "cmd" || call.Name == "sh" || call.Name == "powershell" {
			t.Fatal("shell command was used")
		}
	}
}

func TestUserForceRemovesBeforeAddingAndRemoveFailurePreventsAdd(t *testing.T) {
	command := testExecutable(t, "focalspan")
	t.Run("force", func(t *testing.T) {
		fake := &fakeCommandRunner{}
		fake.fn = func(_ context.Context, _ string, args []string) (CommandResult, error) {
			switch args[1] {
			case "get":
				return CommandResult{Stdout: []byte(`{"command":"other","args":["serve"]}`)}, nil
			case "remove":
				return CommandResult{}, nil
			default:
				return CommandResult{}, nil
			}
		}
		service := NewService(fake)
		req := Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test", Command: command, Force: true}
		if _, err := service.Install(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if len(fake.calls) != 3 || fake.calls[1].Args[1] != "remove" || fake.calls[2].Args[1] != "add" {
			t.Fatalf("calls=%+v", fake.calls)
		}
	})

	t.Run("remove failure", func(t *testing.T) {
		fake := &fakeCommandRunner{}
		fake.fn = func(_ context.Context, _ string, args []string) (CommandResult, error) {
			switch args[1] {
			case "get":
				return CommandResult{Stdout: []byte(`{"command":"other","args":["serve"]}`)}, nil
			case "remove":
				return CommandResult{ExitCode: 1}, errors.New("remove failed")
			default:
				return CommandResult{}, nil
			}
		}
		service := NewService(fake)
		req := Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test", Command: command, Force: true}
		if _, err := service.Install(context.Background(), req); err == nil {
			t.Fatal("remove failure was ignored")
		}
		if len(fake.calls) != 2 {
			t.Fatalf("add ran after remove failure: %+v", fake.calls)
		}
	})
}

func TestUserInstallIdenticalIsNoOpAndConflictNeedsForce(t *testing.T) {
	command := testExecutable(t, "focalspan")

	t.Run("identical", func(t *testing.T) {
		fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
			if args[1] == "get" {
				encodedCommand, _ := json.Marshal(command)
				payload := `{"command":` + string(encodedCommand) + `,"args":["serve","--root","/repo"]}`
				return CommandResult{Stdout: []byte(payload)}, nil
			}
			return CommandResult{}, nil
		}}
		service := NewService(fake)
		result, err := service.Install(context.Background(), Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test", Command: command})
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "unchanged" || len(fake.calls) != 1 {
			t.Fatalf("result=%+v calls=%+v", result, fake.calls)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
			if args[1] == "get" {
				return CommandResult{Stdout: []byte(`{"command":"other","args":[]}`)}, nil
			}
			return CommandResult{}, nil
		}}
		service := NewService(fake)
		_, err := service.Install(context.Background(), Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test", Command: command})
		if err == nil || len(fake.calls) != 1 {
			t.Fatalf("err=%v calls=%+v", err, fake.calls)
		}
	})
}

func TestUserUninstallDoesNotRemoveMismatchedRegistrationWithoutForce(t *testing.T) {
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		if args[1] == "get" {
			return CommandResult{Stdout: []byte(`{"command":"other","args":[]}`)}, nil
		}
		return CommandResult{}, nil
	}}
	service := NewService(fake)
	command := testExecutable(t, "focalspan")
	_, err := service.Uninstall(context.Background(), Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test", Command: command})
	if err == nil || len(fake.calls) != 1 {
		t.Fatalf("err=%v calls=%+v", err, fake.calls)
	}
}

func testExecutable(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("persistent test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUserCommandRunnerHonorsCancellation(t *testing.T) {
	fake := &fakeCommandRunner{fn: func(ctx context.Context, _ string, _ []string) (CommandResult, error) {
		<-ctx.Done()
		return CommandResult{}, ctx.Err()
	}}
	service := NewService(fake)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := service.userGet(ctx, Request{Scope: ScopeUser, Root: "/repo", Name: "focalspan-test"}, "focalspan-test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
