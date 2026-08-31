package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestManagedServeArgsAcceptsBinaryAndAutoUpdateVariants(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	for _, args := range [][]string{
		{"serve", "--root", root},
		{"serve", "--root", root, "--auto-update=false"},
		{"serve", "--root", root, "--no-auto-update"},
	} {
		if !managedLegacyUserRegistration(userRegistration{Command: filepath.Join(t.TempDir(), "focalspan.exe"), Args: args}, root) {
			t.Fatalf("managed registration rejected: %v", args)
		}
	}
	if managedLegacyUserRegistration(userRegistration{Command: "other", Args: []string{"serve", "--root", filepath.Join(root, "other")}}, root) {
		t.Fatal("registration for another root was accepted")
	}
	if managedLegacyUserRegistration(userRegistration{Command: "other", Args: []string{"serve", "--root", root}}, root) {
		t.Fatal("non-FocalSpan command was accepted")
	}
}

func TestManagedGlobalRegistrationHasNoRoot(t *testing.T) {
	command := filepath.Join(t.TempDir(), "focalspan.exe")
	for _, args := range [][]string{{"serve"}, {"serve", "--auto-update=false"}, {"serve", "--no-auto-update"}} {
		if !managedUserRegistration(userRegistration{Command: command, Args: args}) {
			t.Fatalf("managed global registration rejected: %v", args)
		}
	}
	if managedUserRegistration(userRegistration{Command: command, Args: []string{"serve", "--root", "/repo"}}) {
		t.Fatal("root-bound registration was accepted as global")
	}
}

func TestCommandFailureIncludesExitCodeAndStderr(t *testing.T) {
	err := commandFailure("add", "focalspan-test", CommandResult{ExitCode: 7, Stderr: []byte("invalid option")}, nil)
	if !strings.Contains(err.Error(), "exit 7") || !strings.Contains(err.Error(), "invalid option") {
		t.Fatalf("error=%q", err)
	}
}

func TestMissingRegistrationRecognizesCurrentCodexMessage(t *testing.T) {
	result := CommandResult{
		ExitCode: 1,
		Stderr:   []byte("Error: No MCP server named 'focalspan-FocalSpan-01e544cb' found.\n"),
	}
	if !isMissingRegistration(result, errors.New("exit status 1")) {
		t.Fatal("current Codex missing-registration message was not recognized")
	}
}

func TestUserGetParsesCurrentCodexStdioRegistration(t *testing.T) {
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, _ []string) (CommandResult, error) {
		return CommandResult{Stdout: []byte(`{
			"name": "focalspan",
			"enabled": true,
			"transport": {
				"type": "stdio",
				"command": "C:\\Tools\\focalspan.exe",
				"args": ["serve"],
				"env": null,
				"env_vars": [],
				"cwd": null
			}
		}`)}, nil
	}}

	registration, exists, _, err := NewService(fake).userGet(
		context.Background(),
		Request{Scope: ScopeUser, Name: "focalspan"},
		"focalspan",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("userGet() exists = false, want true")
	}
	want := userRegistration{Command: `C:\Tools\focalspan.exe`, Args: []string{"serve"}}
	if !reflect.DeepEqual(registration, want) {
		t.Fatalf("userGet() registration = %+v, want %+v", registration, want)
	}
}

func TestParseUserRegistrationDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "unreadable JSON",
			payload: `{"transport":`,
			wantErr: "Codex returned an unreadable MCP registration",
		},
		{
			name:    "null transport",
			payload: `{"name":"focalspan","transport":null}`,
			wantErr: "Codex returned an MCP registration with a null transport",
		},
		{
			name:    "unsupported transport",
			payload: `{"name":"focalspan","transport":{"type":"streamable_http","url":"https://example.invalid/secret","env":{"TOKEN":"do-not-expose"}}}`,
			wantErr: `Codex returned an unsupported MCP transport "streamable_http"`,
		},
		{
			name:    "empty stdio command",
			payload: `{"name":"focalspan","transport":{"type":"stdio","command":"","args":["serve"],"env":{"TOKEN":"do-not-expose"}}}`,
			wantErr: "Codex returned an MCP registration without a command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUserRegistration([]byte(tt.payload))
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("parseUserRegistration() error = %v, want %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "do-not-expose") || strings.Contains(err.Error(), "example.invalid") {
				t.Fatalf("parseUserRegistration() exposed registration data: %v", err)
			}
		})
	}
}

func TestParseUserRegistrationAcceptsLegacyFlatJSON(t *testing.T) {
	payload := []byte(`{"command":"C:\\Tools\\focalspan.exe","args":["serve"]}`)

	got, err := parseUserRegistration(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := userRegistration{Command: `C:\Tools\focalspan.exe`, Args: []string{"serve"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseUserRegistration() = %+v, want %+v", got, want)
	}
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
	req := Request{Scope: ScopeUser, Name: "focalspan-test", Command: command, NoAutoUpdate: true}
	result, err := service.Install(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "create" {
		t.Fatalf("action=%q", result.Action)
	}
	want := []string{"mcp", "add", "focalspan-test", "--", command, "serve", "--auto-update=false"}
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
				return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", []string{"serve"})}, nil
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
				return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", []string{"serve"})}, nil
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
				return CommandResult{Stdout: currentCodexRegistrationJSON(t, command, []string{"serve"})}, nil
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
				return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", nil)}, nil
			}
			return CommandResult{}, nil
		}}
		service := NewService(fake)
		_, err := service.Install(context.Background(), Request{Scope: ScopeUser, Name: "focalspan-test", Command: command, MigrationRoot: "/repo"})
		if err == nil || len(fake.calls) != 1 {
			t.Fatalf("err=%v calls=%+v", err, fake.calls)
		}
	})
}

func TestUserInstallIgnoresAbsentLegacyRegistration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	command := testExecutable(t, "focalspan")
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		if args[1] == "get" {
			return CommandResult{ExitCode: 1}, errors.New("not found")
		}
		return CommandResult{}, nil
	}}
	result, err := NewService(fake).Install(context.Background(), Request{
		Scope: ScopeUser, Name: "focalspan", Command: command, MigrationRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "create" || len(fake.calls) != 3 || fake.calls[2].Args[2] != LegacyUserServerName(root) {
		t.Fatalf("result=%+v calls=%+v", result, fake.calls)
	}
}

func TestUserInstallMigratesOnlyManagedLegacyRegistrationAfterGlobalInstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	command := testExecutable(t, "focalspan")
	legacyName := LegacyUserServerName(root)
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		switch {
		case args[1] == "get" && args[2] == "focalspan":
			return CommandResult{ExitCode: 1}, errors.New("not found")
		case args[1] == "get" && args[2] == legacyName:
			return CommandResult{Stdout: currentCodexRegistrationJSON(t, command, []string{"serve", "--root", root})}, nil
		default:
			return CommandResult{}, nil
		}
	}}

	result, err := NewService(fake).Install(context.Background(), Request{
		Scope: ScopeUser, Name: "focalspan", Command: command, MigrationRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 4 || fake.calls[1].Args[1] != "add" || fake.calls[2].Args[2] != legacyName || fake.calls[3].Args[1] != "remove" {
		t.Fatalf("calls=%+v", fake.calls)
	}
	if !containsDiagnostic(result.Diagnostics, "migrated legacy") {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
}

func TestUserInstallLeavesUnmanagedLegacyRegistration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	command := testExecutable(t, "focalspan")
	legacyName := LegacyUserServerName(root)
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		switch {
		case args[1] == "get" && args[2] == "focalspan":
			return CommandResult{ExitCode: 1}, errors.New("not found")
		case args[1] == "get" && args[2] == legacyName:
			return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", []string{"serve"})}, nil
		default:
			return CommandResult{}, nil
		}
	}}
	result, err := NewService(fake).Install(context.Background(), Request{
		Scope: ScopeUser, Name: "focalspan", Command: command, MigrationRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 3 || !containsDiagnostic(result.Diagnostics, "not managed") {
		t.Fatalf("result=%+v calls=%+v", result, fake.calls)
	}
}

func TestUserInstallDryRunReportsLegacyMigrationWithoutCommands(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	command := testExecutable(t, "focalspan")
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, _ []string) (CommandResult, error) {
		t.Fatal("dry-run executed Codex command")
		return CommandResult{}, nil
	}}
	result, err := NewService(fake).Install(context.Background(), Request{
		Scope: ScopeUser, Name: "focalspan", Command: command, MigrationRoot: root, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 || !containsDiagnostic(result.Diagnostics, LegacyUserServerName(root)) {
		t.Fatalf("result=%+v calls=%+v", result, fake.calls)
	}
}

func TestUserInstallReportsLegacyRemovalFailureAfterGlobalInstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	command := testExecutable(t, "focalspan")
	legacyName := LegacyUserServerName(root)
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		switch {
		case args[1] == "get" && args[2] == "focalspan":
			return CommandResult{ExitCode: 1}, errors.New("not found")
		case args[1] == "get" && args[2] == legacyName:
			return CommandResult{Stdout: currentCodexRegistrationJSON(t, command, []string{"serve", "--root", root})}, nil
		case args[1] == "remove":
			return CommandResult{ExitCode: 1, Stderr: []byte("locked")}, errors.New("exit status 1")
		default:
			return CommandResult{}, nil
		}
	}}
	_, err := NewService(fake).Install(context.Background(), Request{
		Scope: ScopeUser, Name: "focalspan", Command: command, MigrationRoot: root,
	})
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err=%v calls=%+v", err, fake.calls)
	}
	if len(fake.calls) != 4 || fake.calls[1].Args[1] != "add" || fake.calls[3].Args[1] != "remove" {
		t.Fatalf("calls=%+v", fake.calls)
	}
}

func containsDiagnostic(diagnostics []string, substring string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic, substring) {
			return true
		}
	}
	return false
}

func TestUserUninstallDoesNotRemoveMismatchedRegistrationWithoutForce(t *testing.T) {
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		if args[1] == "get" {
			return CommandResult{Stdout: currentCodexRegistrationJSON(t, "other", nil)}, nil
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

func TestUserStatusAndUninstallExposeRuntimeCWDMode(t *testing.T) {
	fake := &fakeCommandRunner{fn: func(_ context.Context, _ string, args []string) (CommandResult, error) {
		if args[1] == "get" {
			return CommandResult{ExitCode: 1}, errors.New("not found")
		}
		return CommandResult{}, nil
	}}
	service := NewService(fake)
	status, err := service.Status(context.Background(), Request{Scope: ScopeUser, Name: "focalspan"})
	if err != nil {
		t.Fatal(err)
	}
	if status.Root != "" || status.RootMode != "runtime_cwd" {
		t.Fatalf("status=%+v", status)
	}
	result, err := service.Uninstall(context.Background(), Request{Scope: ScopeUser, Name: "focalspan", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Root != "" || result.RootMode != "runtime_cwd" {
		t.Fatalf("result=%+v", result)
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

func currentCodexRegistrationJSON(t *testing.T, command string, args []string) []byte {
	t.Helper()
	if args == nil {
		args = []string{}
	}
	payload := struct {
		Name      string `json:"name"`
		Enabled   bool   `json:"enabled"`
		Transport struct {
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			EnvVars []string          `json:"env_vars"`
			CWD     *string           `json:"cwd"`
		} `json:"transport"`
	}{Name: "focalspan", Enabled: true}
	payload.Transport.Type = "stdio"
	payload.Transport.Command = command
	payload.Transport.Args = args
	payload.Transport.EnvVars = []string{}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
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
