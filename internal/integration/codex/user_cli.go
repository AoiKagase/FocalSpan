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
	Command string
	Args    []string
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
	var payload struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return userRegistration{}, false, codexCommand, errors.New("Codex returned an unreadable MCP registration")
	}
	if payload.Command == "" {
		return userRegistration{}, false, codexCommand, errors.New("Codex returned an MCP registration without a command")
	}
	return userRegistration{Command: payload.Command, Args: append([]string(nil), payload.Args...)}, true, codexCommand, nil
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
	return current.Command == expected.Command && stringSlicesEqual(current.Args, expected.Args)
}

func managedUserRegistration(current userRegistration) bool {
	return managedFocalSpanCommand(current.Command) && managedGlobalServeArgs(current.Args)
}

func managedGlobalServeArgs(args []string) bool {
	if len(args) != 1 && len(args) != 2 {
		return false
	}
	return args[0] == "serve" && (len(args) == 1 || args[1] == "--no-auto-update" || args[1] == "--auto-update=false")
}

func managedLegacyUserRegistration(current userRegistration, root string) bool {
	return managedFocalSpanCommand(current.Command) && managedServeArgs(current.Args, root)
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
