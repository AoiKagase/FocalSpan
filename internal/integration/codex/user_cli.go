package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const codexCommandTimeout = 30 * time.Second

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	command := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	return result, err
}

type userRegistration struct {
	observedConfig    *projectObservation
	executionSettings map[string]json.RawMessage
	executionDrift    bool
	cwd               string
	Command           string
	Args              []string
}

func (s *Service) codexRunner(req Request) (CommandRunner, string, error) {
	name := req.CodexCommand
	if name == "" {
		name = "codex"
	}
	if s.commandIsSet {
		return s.runner, name, nil
	}
	found, err := exec.LookPath(name)
	if err != nil {
		return nil, "", ErrCodexNotFound
	}
	return execRunner{}, found, nil
}

func (s *Service) userGet(ctx context.Context, req Request, name string) (userRegistration, bool, string, error) {
	runner, codexCommand, err := s.codexRunner(req)
	if err != nil {
		return userRegistration{}, false, codexCommand, err
	}
	commandCtx, cancel := context.WithTimeout(ctx, codexCommandTimeout)
	defer cancel()
	result, runErr := runner.Run(commandCtx, codexCommand, "mcp", "get", name, "--json")
	if commandCtx.Err() != nil {
		return userRegistration{}, false, codexCommand, commandCtx.Err()
	}
	if runErr != nil || result.ExitCode != 0 {
		if isMissingRegistration(result, runErr) {
			return userRegistration{}, false, codexCommand, nil
		}
		return userRegistration{}, false, codexCommand, commandFailure("get", name, result, runErr)
	}
	if err := commandContextError(commandCtx, runErr); err != nil {
		return userRegistration{}, false, codexCommand, err
	}
	registration, err := parseUserRegistration(result.Stdout)
	if err != nil {
		return userRegistration{}, false, codexCommand, err
	}
	return registration, true, codexCommand, nil
}

func parseUserRegistration(data []byte) (userRegistration, error) {
	var payload struct {
		Command   string          `json:"command"`
		Args      []string        `json:"args"`
		Transport json.RawMessage `json:"transport"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return userRegistration{}, errors.New("Codex returned an unreadable MCP registration")
	}
	var transport map[string]json.RawMessage
	if len(payload.Transport) > 0 {
		if string(bytes.TrimSpace(payload.Transport)) == "null" {
			return userRegistration{}, errors.New("Codex returned an MCP registration with a null transport")
		}
		var stdio struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal(payload.Transport, &stdio); err != nil {
			return userRegistration{}, errors.New("Codex returned an unreadable MCP registration")
		}
		if stdio.Type != "stdio" {
			return userRegistration{}, fmt.Errorf("Codex returned an unsupported MCP transport %q", stdio.Type)
		}
		payload.Command, payload.Args = stdio.Command, stdio.Args
		if err := json.Unmarshal(payload.Transport, &transport); err != nil {
			return userRegistration{}, errors.New("Codex returned an unreadable MCP registration")
		}
	}
	registration, err := validatedUserRegistration(payload.Command, payload.Args)
	if err != nil {
		return userRegistration{}, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return userRegistration{}, errors.New("Codex returned an unreadable MCP registration")
	}
	registration.executionSettings = make(map[string]json.RawMessage)
	for _, level := range []struct {
		prefix string
		fields map[string]json.RawMessage
	}{{"", root}, {"transport.", transport}} {
		for _, key := range []string{"enabled", "cwd", "env", "env_vars", "startup_timeout_sec", "tool_timeout_sec", "enabled_tools", "disabled_tools"} {
			raw, exists := level.fields[key]
			if !exists {
				continue
			}
			registration.executionSettings[level.prefix+key] = append(json.RawMessage(nil), raw...)
			normal, cwd, err := defaultUserSetting(key, raw)
			if err != nil {
				return userRegistration{}, fmt.Errorf("Codex returned an invalid MCP execution setting %s", key)
			}
			registration.executionDrift = registration.executionDrift || !normal
			if cwd != "" {
				registration.cwd = cwd
			}
		}
	}
	return registration, nil
}

// Global CLI installation specifies no cwd, environment, timeouts or tool
// restrictions. Null and empty no-op values preserve legacy/default JSON.
func defaultUserSetting(key string, raw json.RawMessage) (bool, string, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		if key == "enabled" {
			return false, "", errors.New("enabled must be boolean")
		}
		return true, "", nil
	}
	switch key {
	case "enabled":
		var v bool
		err := json.Unmarshal(raw, &v)
		return v, "", err
	case "cwd":
		var v string
		err := json.Unmarshal(raw, &v)
		return v == "", v, err
	case "env":
		var v map[string]string
		err := json.Unmarshal(raw, &v)
		return len(v) == 0, "", err
	case "env_vars", "enabled_tools", "disabled_tools":
		var v []string
		err := json.Unmarshal(raw, &v)
		return key != "enabled_tools" && len(v) == 0, "", err
	default:
		var v float64
		err := json.Unmarshal(raw, &v)
		if err == nil && v < 0 {
			err = errors.New("negative timeout")
		}
		return false, "", err
	}
}

func validatedUserRegistration(command string, args []string) (userRegistration, error) {
	if command == "" {
		return userRegistration{}, errors.New("Codex returned an MCP registration without a command")
	}
	return userRegistration{Command: command, Args: append([]string(nil), args...)}, nil
}

func (s *Service) userAdd(ctx context.Context, req Request, name string, spec RegistrationSpec, codexCommand string) error {
	runner, resolved, err := s.codexRunner(req)
	if err != nil {
		return err
	}
	if codexCommand != "" {
		resolved = codexCommand
	}
	args := []string{"mcp", "add", name, "--", spec.Command}
	args = append(args, spec.Args...)
	commandCtx, cancel := context.WithTimeout(ctx, codexCommandTimeout)
	defer cancel()
	result, runErr := runner.Run(commandCtx, resolved, args...)
	if commandCtx.Err() != nil {
		return commandCtx.Err()
	}
	if runErr != nil || result.ExitCode != 0 {
		return commandFailure("add", name, result, runErr)
	}
	return nil
}

func (s *Service) userRemove(ctx context.Context, req Request, name string, codexCommand string) error {
	runner, resolved, err := s.codexRunner(req)
	if err != nil {
		return err
	}
	if codexCommand != "" {
		resolved = codexCommand
	}
	commandCtx, cancel := context.WithTimeout(ctx, codexCommandTimeout)
	defer cancel()
	result, runErr := runner.Run(commandCtx, resolved, "mcp", "remove", name)
	if commandCtx.Err() != nil {
		return commandCtx.Err()
	}
	if runErr != nil || result.ExitCode != 0 {
		return commandFailure("remove", name, result, runErr)
	}
	return nil
}

func commandContextError(ctx context.Context, runErr error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return runErr
}

func commandFailure(operation, name string, result CommandResult, runErr error) error {
	detail := strings.TrimSpace(string(result.Stderr))
	if len(detail) > 2048 {
		detail = detail[:2048] + "..."
	}
	message := fmt.Sprintf("Codex MCP %s failed for server %q (exit %d)", operation, name, result.ExitCode)
	if detail != "" {
		message += ": " + detail
	}
	if runErr != nil {
		message += ": " + runErr.Error()
	}
	return errors.New(message)
}

func isMissingRegistration(result CommandResult, runErr error) bool {
	if result.ExitCode == 0 && runErr == nil {
		return false
	}
	message := strings.ToLower(string(result.Stderr))
	if runErr != nil {
		message += " " + strings.ToLower(runErr.Error())
	}
	return strings.Contains(message, "not found") || strings.Contains(message, "does not exist") ||
		strings.Contains(message, "no mcp server named") ||
		(result.ExitCode != 0 && strings.TrimSpace(message) == "" && len(result.Stdout) == 0)
}

func sameUserRegistration(current userRegistration, expected RegistrationSpec) bool {
	return !current.executionDrift && current.Command == expected.Command && stringSlicesEqual(current.Args, expected.Args)
}

func managedUserRegistration(current userRegistration) bool {
	return !current.executionDrift && managedFocalSpanCommand(current.Command) && managedGlobalServeArgs(current.Args)
}

func managedGlobalServeArgs(args []string) bool {
	if len(args) != 1 && len(args) != 2 {
		return false
	}
	return args[0] == "serve" && (len(args) == 1 || args[1] == "--no-auto-update" || args[1] == "--auto-update=false")
}

func managedLegacyUserRegistration(current userRegistration, root string) bool {
	return !current.executionDrift && managedFocalSpanCommand(current.Command) && managedServeArgs(current.Args, root)
}

func managedFocalSpanCommand(command string) bool {
	base := filepath.Base(filepath.Clean(command))
	return strings.EqualFold(base, "focalspan") || strings.EqualFold(base, "focalspan.exe")
}

func managedServeArgs(args []string, root string) bool {
	if len(args) != 3 && len(args) != 4 {
		return false
	}
	if args[0] != "serve" || args[1] != "--root" || filepath.Clean(args[2]) != filepath.Clean(root) {
		return false
	}
	return len(args) == 3 || args[3] == "--no-auto-update" || args[3] == "--auto-update=false"
}

func userArgv(codexCommand, name string, spec RegistrationSpec) []string {
	args := []string{codexCommand, "mcp", "add", name, "--", spec.Command}
	return append(args, spec.Args...)
}

func formatArgv(argv []string) string {
	parts := make([]string, len(argv))
	for i, arg := range argv {
		if arg == "" || strings.ContainsAny(arg, " \t\r\n\"'") {
			parts[i] = strconv.Quote(arg)
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
