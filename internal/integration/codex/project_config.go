package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type projectObservation struct {
	content       []byte
	exists        bool
	managed       bool
	managedBounds managedBounds
	server        map[string]any
	serverFound   bool
}

func InstallProject(ctx context.Context, configPath, name string, spec RegistrationSpec, dryRun bool) (OperationResult, error) {
	if err := ValidateName(name); err != nil {
		return OperationResult{}, err
	}
	block, err := BuildManagedBlock(name, spec)
	if err != nil {
		return OperationResult{}, err
	}
	obs, err := inspectProject(configPath, name)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Client: ClientName, Scope: ScopeProject, Name: name, ConfigPath: configPath, Command: spec.Command, Args: append([]string(nil), spec.Args...), Block: block, DryRun: dryRun}
	switch {
	case obs.managed && obs.serverFound && tableMatches(obs.server, spec):
		result.Action, result.State = "unchanged", StateManagedMatch
	case obs.managed:
		result.Action, result.State = "update", StateManagedDrift
	case obs.serverFound && tableMatches(obs.server, spec):
		result.Action, result.State = "unchanged", StateUnmanagedMatch
	case obs.serverFound:
		result.Action, result.State = "conflict", StateConflict
	default:
		result.Action, result.State = "create", StateAbsent
	}
	if result.Action == "conflict" {
		if dryRun {
			return result, nil
		}
		return OperationResult{}, fmt.Errorf("unmanaged MCP server %q already exists in %s; choose another --name or resolve it manually (--force does not remove unmanaged settings)", name, configPath)
	}
	if dryRun || result.Action == "unchanged" {
		return result, nil
	}

	newContent := appendManagedBlock(obs.content, block, result.Action == "update", obs.managedBounds)
	if err := validateTOML(newContent); err != nil {
		return OperationResult{}, fmt.Errorf("validate generated Codex config: %w", err)
	}
	if err := ensureConfigDirectory(filepath.Dir(configPath)); err != nil {
		return OperationResult{}, err
	}
	mode := os.FileMode(0o600)
	if obs.exists {
		info, statErr := os.Stat(configPath)
		if statErr != nil {
			return OperationResult{}, fmt.Errorf("stat Codex config: %w", statErr)
		}
		mode = info.Mode().Perm()
	}
	if err := atomicWrite(configPath, newContent, mode); err != nil {
		return OperationResult{}, fmt.Errorf("write Codex config: %w", err)
	}
	return result, nil
}

func StatusProject(configPath, root, name string, spec RegistrationSpec) RegistrationStatus {
	status := RegistrationStatus{
		Client: ClientName, Scope: ScopeProject, Name: name, Root: root, ConfigPath: configPath,
		Command: spec.Command, Args: append([]string(nil), spec.Args...), Expected: specPtr(spec),
		Diagnostics: []string{"Codex loads project-local .codex/config.toml only for trusted projects."},
	}
	obs, err := inspectProject(configPath, name)
	if err != nil {
		status.State = StateInvalidConfig
		status.Diagnostics = append(status.Diagnostics, err.Error())
		return status
	}
	status.Managed = obs.managed
	if obs.serverFound {
		status.Command, status.Args = tableIdentity(obs.server)
	}
	switch {
	case obs.managed && obs.serverFound && tableMatches(obs.server, spec):
		status.State, status.Matches = StateManagedMatch, true
	case obs.managed:
		status.State = StateManagedDrift
	case obs.serverFound && tableMatches(obs.server, spec):
		status.State, status.Matches = StateUnmanagedMatch, true
	case obs.serverFound:
		status.State = StateConflict
	default:
		status.State = StateAbsent
	}
	return status
}

func UninstallProject(ctx context.Context, configPath, name string, dryRun bool) (OperationResult, error) {
	if err := ValidateName(name); err != nil {
		return OperationResult{}, err
	}
	obs, err := inspectProject(configPath, name)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Client: ClientName, Scope: ScopeProject, Name: name, ConfigPath: configPath, Action: "unchanged", State: StateAbsent, DryRun: dryRun}
	if !obs.managed {
		return result, nil
	}
	result.Action, result.State = "removed", StateAbsent
	if dryRun {
		return result, nil
	}
	newContent := appendManagedBlock(obs.content, "", true, obs.managedBounds)
	if err := validateTOML(newContent); err != nil {
		return OperationResult{}, fmt.Errorf("validate Codex config after uninstall: %w", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		return OperationResult{}, fmt.Errorf("stat Codex config: %w", err)
	}
	if err := atomicWrite(configPath, newContent, info.Mode().Perm()); err != nil {
		return OperationResult{}, fmt.Errorf("write Codex config: %w", err)
	}
	return result, nil
}

func inspectProject(configPath, name string) (projectObservation, error) {
	content, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return projectObservation{}, nil
	}
	if err != nil {
		return projectObservation{}, fmt.Errorf("read Codex config: %w", err)
	}
	if err := validateTOML(content); err != nil {
		return projectObservation{}, fmt.Errorf("invalid Codex TOML: %w", err)
	}
	var document map[string]any
	if len(content) > 0 {
		if err := toml.Unmarshal(content, &document); err != nil {
			return projectObservation{}, fmt.Errorf("parse Codex TOML: %w", err)
		}
	}
	server, found := findServer(document, name)
	bounds, managed, err := findManagedBounds(string(content), name)
	if err != nil {
		return projectObservation{}, err
	}
	return projectObservation{content: content, exists: true, managed: managed, managedBounds: bounds, server: server, serverFound: found}, nil
}

func validateTOML(content []byte) error {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil
	}
	var document map[string]any
	return toml.Unmarshal(content, &document)
}

func findServer(document map[string]any, name string) (map[string]any, bool) {
	servers, ok := document["mcp_servers"].(map[string]any)
	if !ok {
		return nil, false
	}
	server, ok := servers[name].(map[string]any)
	return server, ok
}

func tableMatches(table map[string]any, spec RegistrationSpec) bool {
	if len(table) != 6 {
		return false
	}
	command, ok := table["command"].(string)
	if !ok || command != spec.Command {
		return false
	}
	args, ok := stringSlice(table["args"])
	if !ok || !stringSlicesEqual(args, spec.Args) {
		return false
	}
	enabled, ok := table["enabled"].(bool)
	if !ok || enabled != spec.Enabled {
		return false
	}
	startup, ok := integerValue(table["startup_timeout_sec"])
	if !ok || startup != int64(spec.StartupTimeoutSec) {
		return false
	}
	toolTimeout, ok := integerValue(table["tool_timeout_sec"])
	if !ok || toolTimeout != int64(spec.ToolTimeoutSec) {
		return false
	}
	tools, ok := stringSlice(table["enabled_tools"])
	return ok && stringSlicesEqual(tools, spec.EnabledTools)
}

func tableIdentity(table map[string]any) (string, []string) {
	command, _ := table["command"].(string)
	args, _ := stringSlice(table["args"])
	return command, args
}

func stringSlice(value any) ([]string, bool) {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...), true
	case []any:
		result := make([]string, len(values))
		for i, value := range values {
			stringValue, ok := value.(string)
			if !ok {
				return nil, false
			}
			result[i] = stringValue
		}
		return result, true
	default:
		return nil, false
	}
}

func integerValue(value any) (int64, bool) {
	switch value := value.(type) {
	case int:
		return int64(value), true
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case uint:
		return int64(value), uint64(value) <= uint64(^uint64(0)>>1)
	case uint8:
		return int64(value), true
	case uint16:
		return int64(value), true
	case uint32:
		return int64(value), true
	case uint64:
		return int64(value), value <= uint64(^uint64(0)>>1)
	default:
		return 0, false
	}
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func appendManagedBlock(content []byte, block string, replace bool, bounds managedBounds) []byte {
	newline := "\n"
	if bytes.Contains(content, []byte("\r\n")) && !bytes.Contains(bytes.ReplaceAll(content, []byte("\r\n"), nil), []byte("\n")) {
		newline = "\r\n"
	}
	block = strings.ReplaceAll(block, "\n", newline)
	if replace {
		result := make([]byte, 0, len(content)-bounds.end+bounds.start+len(block))
		result = append(result, content[:bounds.start]...)
		result = append(result, block...)
		result = append(result, content[bounds.end:]...)
		return result
	}
	if len(content) == 0 {
		return []byte(block)
	}
	result := append([]byte(nil), content...)
	if !bytes.HasSuffix(result, []byte("\n")) {
		result = append(result, newline...)
	}
	return append(result, block...)
}

func ensureConfigDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("Codex config parent is not a directory: %s", dir)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat Codex config directory: %w", err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return fmt.Errorf("create Codex config directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("set Codex config directory mode: %w", err)
		}
	}
	return nil
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".config.toml.focalspan-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func specPtr(spec RegistrationSpec) *RegistrationSpec {
	copy := spec.clone()
	return &copy
}
