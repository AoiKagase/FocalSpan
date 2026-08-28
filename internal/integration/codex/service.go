package codex

import (
	"context"
	"errors"
	"fmt"
)

func (s *Service) Install(ctx context.Context, req Request) (OperationResult, error) {
	if err := validateRequest(req); err != nil {
		return OperationResult{}, err
	}
	spec, warning, err := s.registrationSpec(req)
	if err != nil {
		return OperationResult{}, err
	}
	if req.Scope == ScopeProject {
		result, err := InstallProject(ctx, ProjectConfigPath(req.Root), req.Name, spec, req.DryRun)
		if warning != "" {
			result.Diagnostics = append(result.Diagnostics, warning)
		}
		return result, err
	}
	return s.installUser(ctx, req, spec, warning)
}

func (s *Service) installUser(ctx context.Context, req Request, spec RegistrationSpec, warning string) (OperationResult, error) {
	configPath, err := UserConfigPath()
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Client: ClientName, Scope: ScopeUser, Name: req.Name, Root: req.Root, ConfigPath: configPath, Command: spec.Command, Args: append([]string(nil), spec.Args...), DryRun: req.DryRun}
	codexCommand := req.CodexCommand
	if codexCommand == "" {
		codexCommand = "codex"
	}
	result.Argv = userArgv(codexCommand, req.Name, spec)
	if warning != "" {
		result.Diagnostics = append(result.Diagnostics, warning)
	}
	if req.DryRun {
		result.Action, result.State = "create", StateAbsent
		if _, _, err := s.codexRunner(req); err != nil && !errors.Is(err, ErrCodexNotFound) {
			return OperationResult{}, err
		} else if errors.Is(err, ErrCodexNotFound) {
			result.Diagnostics = append(result.Diagnostics, "Codex CLI was not found; the command above was not executed")
		}
		return result, nil
	}
	current, found, resolvedCodex, err := s.userGet(ctx, req, req.Name)
	if errors.Is(err, ErrCodexNotFound) {
		return OperationResult{}, fmt.Errorf("%w; expected argv: %s", ErrCodexNotFound, formatArgv(result.Argv))
	}
	if err != nil {
		return OperationResult{}, err
	}
	if found && sameUserRegistration(current, spec) {
		result.Action, result.State = "unchanged", StateManagedMatch
		return result, nil
	}
	if found && !req.Force {
		result.Action, result.State = "conflict", StateConflict
		return OperationResult{}, fmt.Errorf("Codex MCP server %q already has a different registration; use --force to replace it", req.Name)
	}
	if found {
		if err := s.userRemove(ctx, req, req.Name, resolvedCodex); err != nil {
			return OperationResult{}, err
		}
	}
	if err := s.userAdd(ctx, req, req.Name, spec, resolvedCodex); err != nil {
		return OperationResult{}, err
	}
	result.Action, result.State = "updated", StateManagedMatch
	if !found {
		result.Action, result.State = "create", StateManagedMatch
	}
	return result, nil
}

func (s *Service) Status(ctx context.Context, req Request) (RegistrationStatus, error) {
	if err := validateRequest(req); err != nil {
		return RegistrationStatus{}, err
	}
	spec, warning, err := s.registrationSpecForStatus(req)
	if err != nil {
		return RegistrationStatus{}, err
	}
	configPath := ProjectConfigPath(req.Root)
	if req.Scope == ScopeProject {
		status := StatusProject(configPath, req.Root, req.Name, spec)
		if warning != "" {
			status.Diagnostics = append(status.Diagnostics, warning)
		}
		return status, nil
	}
	configPath, err = UserConfigPath()
	if err != nil {
		return RegistrationStatus{}, err
	}
	status := RegistrationStatus{Client: ClientName, Scope: ScopeUser, Name: req.Name, Root: req.Root, ConfigPath: configPath, Expected: specPtr(spec)}
	current, found, _, err := s.userGet(ctx, req, req.Name)
	if errors.Is(err, ErrCodexNotFound) {
		status.State = StateCodexNotFound
		status.Diagnostics = append(status.Diagnostics, "Codex CLI was not found on PATH")
		if warning != "" {
			status.Diagnostics = append(status.Diagnostics, warning)
		}
		return status, nil
	}
	if err != nil {
		status.State = StateUnknown
		status.Diagnostics = append(status.Diagnostics, err.Error())
		return status, nil
	}
	if !found {
		status.State = StateAbsent
	} else {
		status.Command, status.Args = current.Command, append([]string(nil), current.Args...)
		status.Matches = sameUserRegistration(current, spec)
		if status.Matches {
			status.State, status.Managed = StateManagedMatch, true
		} else {
			status.State = StateManagedDrift
		}
	}
	if warning != "" {
		status.Diagnostics = append(status.Diagnostics, warning)
	}
	return status, nil
}

func (s *Service) Uninstall(ctx context.Context, req Request) (OperationResult, error) {
	if err := validateRequest(req); err != nil {
		return OperationResult{}, err
	}
	spec, warning, err := s.registrationSpecForStatus(req)
	if err != nil {
		return OperationResult{}, err
	}
	if req.Scope == ScopeProject {
		result, err := UninstallProject(ctx, ProjectConfigPath(req.Root), req.Name, req.DryRun)
		if warning != "" {
			result.Diagnostics = append(result.Diagnostics, warning)
		}
		return result, err
	}
	configPath, err := UserConfigPath()
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Client: ClientName, Scope: ScopeUser, Name: req.Name, Root: req.Root, ConfigPath: configPath, Command: spec.Command, Args: append([]string(nil), spec.Args...), Action: "unchanged", State: StateAbsent, DryRun: req.DryRun}
	codexCommand := req.CodexCommand
	if codexCommand == "" {
		codexCommand = "codex"
	}
	result.Argv = []string{codexCommand, "mcp", "remove", req.Name}
	if warning != "" {
		result.Diagnostics = append(result.Diagnostics, warning)
	}
	if req.DryRun {
		result.Action = "remove"
		return result, nil
	}
	current, found, resolvedCodex, err := s.userGet(ctx, req, req.Name)
	if errors.Is(err, ErrCodexNotFound) {
		return OperationResult{}, fmt.Errorf("%w; expected argv: %s", ErrCodexNotFound, formatArgv(result.Argv))
	}
	if err != nil {
		return OperationResult{}, err
	}
	if !found {
		return result, nil
	}
	if !sameUserRegistration(current, spec) && !req.Force {
		return OperationResult{}, fmt.Errorf("Codex MCP server %q is not the expected FocalSpan registration; use --force to remove it", req.Name)
	}
	if err := s.userRemove(ctx, req, req.Name, resolvedCodex); err != nil {
		return OperationResult{}, err
	}
	result.Action, result.State = "removed", StateAbsent
	return result, nil
}

func (s *Service) Print(req Request) (OperationResult, error) {
	if err := validateRequest(req); err != nil {
		return OperationResult{}, err
	}
	spec, warning, err := s.registrationSpecForPrint(req)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Client: ClientName, Scope: req.Scope, Name: req.Name, Root: req.Root, Command: spec.Command, Args: append([]string(nil), spec.Args...), Diagnostics: nil}
	if warning != "" {
		result.Diagnostics = append(result.Diagnostics, warning)
	}
	if req.Scope == ScopeProject {
		result.ConfigPath = ProjectConfigPath(req.Root)
		result.Block, err = BuildManagedBlock(req.Name, spec)
		return result, err
	}
	result.ConfigPath, err = UserConfigPath()
	if err != nil {
		return OperationResult{}, err
	}
	codexCommand := req.CodexCommand
	if codexCommand == "" {
		codexCommand = "codex"
	}
	result.Argv = userArgv(codexCommand, req.Name, spec)
	return result, nil
}

func (s *Service) registrationSpec(req Request) (RegistrationSpec, string, error) {
	command, warning, err := ResolveExecutable(req.Command, req.DryRun)
	if err != nil {
		return RegistrationSpec{}, "", err
	}
	spec := makeSpec(req.Root, command, req.NoAutoUpdate)
	if err := validateTOMLValues(spec); err != nil {
		return RegistrationSpec{}, "", err
	}
	return spec, warning, nil
}

func (s *Service) registrationSpecForStatus(req Request) (RegistrationSpec, string, error) {
	command, warning, err := ResolveExecutable("", true)
	if err != nil {
		return RegistrationSpec{}, "", err
	}
	spec := makeSpec(req.Root, command, false)
	if err := validateTOMLValues(spec); err != nil {
		return RegistrationSpec{}, "", err
	}
	return spec, warning, nil
}

func (s *Service) registrationSpecForPrint(req Request) (RegistrationSpec, string, error) {
	command, warning, err := ResolveExecutable(req.Command, true)
	if err != nil {
		return RegistrationSpec{}, "", err
	}
	spec := makeSpec(req.Root, command, req.NoAutoUpdate)
	if err := validateTOMLValues(spec); err != nil {
		return RegistrationSpec{}, "", err
	}
	return spec, warning, nil
}

func makeSpec(root, command string, noAutoUpdate bool) RegistrationSpec {
	args := []string{"serve", "--root", root}
	if noAutoUpdate {
		args = append(args, "--no-auto-update")
	}
	spec := RegistrationSpec{Command: command, Args: args, Enabled: true, StartupTimeoutSec: 30, ToolTimeoutSec: 60, EnabledTools: append([]string(nil), EnabledTools...)}
	return spec
}
