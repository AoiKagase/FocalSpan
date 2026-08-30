package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	ClientName = "codex"

	ScopeProject = "project"
	ScopeUser    = "user"

	StateAbsent         = "absent"
	StateManagedMatch   = "managed_match"
	StateManagedDrift   = "managed_drift"
	StateUnmanagedMatch = "unmanaged_match"
	StateConflict       = "conflict"
	StateInvalidConfig  = "invalid_config"
	StateCodexNotFound  = "codex_not_found"
	StateUnknown        = "unknown"
)

var EnabledTools = []string{"code_context", "code_expand", "code_impact", "code_restart", "code_status"}

var ErrCodexNotFound = errors.New("Codex CLI was not found on PATH")

type RegistrationSpec struct {
	Command           string   `json:"command"`
	Args              []string `json:"args"`
	Enabled           bool     `json:"enabled"`
	StartupTimeoutSec int      `json:"startup_timeout_sec"`
	ToolTimeoutSec    int      `json:"tool_timeout_sec"`
	EnabledTools      []string `json:"enabled_tools"`
}

func (s RegistrationSpec) clone() RegistrationSpec {
	s.Args = append([]string(nil), s.Args...)
	s.EnabledTools = append([]string(nil), s.EnabledTools...)
	return s
}

func (s RegistrationSpec) identityEqual(other RegistrationSpec) bool {
	if s.Command != other.Command || len(s.Args) != len(other.Args) {
		return false
	}
	for i := range s.Args {
		if s.Args[i] != other.Args[i] {
			return false
		}
	}
	return true
}

type RegistrationStatus struct {
	Client      string            `json:"client"`
	Scope       string            `json:"scope"`
	State       string            `json:"state"`
	Name        string            `json:"name"`
	Root        string            `json:"root,omitempty"`
	RootMode    string            `json:"root_mode,omitempty"`
	ConfigPath  string            `json:"config_path,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Expected    *RegistrationSpec `json:"expected,omitempty"`
	Managed     bool              `json:"managed"`
	Matches     bool              `json:"matches"`
	Diagnostics []string          `json:"diagnostics,omitempty"`
}

type OperationResult struct {
	Client      string   `json:"client"`
	Scope       string   `json:"scope"`
	Action      string   `json:"action"`
	State       string   `json:"state,omitempty"`
	Name        string   `json:"name"`
	Root        string   `json:"root,omitempty"`
	RootMode    string   `json:"root_mode,omitempty"`
	ConfigPath  string   `json:"config_path,omitempty"`
	Command     string   `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	Block       string   `json:"block,omitempty"`
	Argv        []string `json:"argv,omitempty"`
	DryRun      bool     `json:"dry_run,omitempty"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

type Request struct {
	Root          string
	MigrationRoot string
	Scope         string
	Name          string
	Command       string
	CodexCommand  string
	NoAutoUpdate  bool
	DryRun        bool
	Force         bool
}

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

type Service struct {
	runner       CommandRunner
	commandIsSet bool
}

func NewService(runner CommandRunner) *Service {
	return &Service{runner: runner, commandIsSet: runner != nil}
}

func validateRequest(req Request) error {
	if req.Scope != ScopeProject && req.Scope != ScopeUser {
		return fmt.Errorf("invalid scope %q: must be project or user", req.Scope)
	}
	if err := ValidateName(req.Name); err != nil {
		return err
	}
	if req.Scope == ScopeProject && req.Root == "" {
		return errors.New("repository root is required")
	}
	if strings.ContainsAny(req.Root, "\x00\r\n") {
		return errors.New("repository root contains NUL or a newline")
	}
	if strings.ContainsAny(req.MigrationRoot, "\x00\r\n") {
		return errors.New("migration root contains NUL or a newline")
	}
	if strings.ContainsAny(req.CodexCommand, "\x00\r\n") {
		return errors.New("Codex command contains NUL or a newline")
	}
	return nil
}
