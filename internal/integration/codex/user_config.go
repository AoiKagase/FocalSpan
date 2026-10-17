package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/pelletier/go-toml/v2"
	"os"
	"strings"
)

func buildUserBlock(name string, spec RegistrationSpec) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := validateTOMLValues(spec); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\ncommand = %s\nargs = [", quoteTOML(name), quoteTOML(spec.Command))
	for i, arg := range spec.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteTOML(arg))
	}
	b.WriteString("]\nenabled = true\n")
	return b.String(), nil
}

// replaceUserTable removes only the named server's table sections. Standalone
// headers are parsed as TOML, so quoted keys and child tables have the same
// identity as the complete document. Headers inside strings are never parsed.
func replaceUserTable(content []byte, name, block string) ([]byte, error) {
	var b strings.Builder
	var state byte
	owned, found := false, false
	for _, line := range strings.SplitAfter(string(content), "\n") {
		before := state
		trimmed := strings.TrimSpace(line)
		if before == 0 && strings.HasPrefix(trimmed, "[") {
			var doc map[string]any
			if err := toml.Unmarshal([]byte(trimmed+"\n__focalspan_boundary__ = true\n"), &doc); err == nil {
				_, owned = findServer(doc, name)
				found = found || owned
			}
		}
		state = tomlStringState(line, state)
		if before == 0 && (trimmed == managedBeginMarker(name) || trimmed == managedEndMarker(name)) {
			continue
		}
		if !owned || before == 0 && (trimmed == "" || strings.HasPrefix(trimmed, "#")) {
			b.WriteString(line)
		}
	}
	if !found {
		return nil, errors.New("cannot safely replace an inline Codex server table; expand the named server to its own table")
	}
	return appendManagedBlock([]byte(b.String()), block, false, managedBounds{}), nil
}
func replaceUserRegistration(ctx context.Context, configPath, name string, spec RegistrationSpec, observed userRegistration) (OperationResult, error) {
	if err := ctx.Err(); err != nil {
		return OperationResult{}, err
	}
	lease, err := indexlock.AcquireConfiguration(ctx, configPath+".focalspan.lock")
	if err != nil {
		return OperationResult{}, err
	}
	defer lease.Close()
	obs, err := inspectProject(configPath, name)
	if err != nil {
		return OperationResult{}, err
	}
	expected := observed.observedConfig
	if expected == nil || !obs.exists || !obs.serverFound || obs.exists != expected.exists || !bytes.Equal(obs.content, expected.content) {
		return OperationResult{}, &ConfigConflictError{Path: configPath}
	}
	command, args := tableIdentity(obs.server)
	if command != observed.Command || !stringSlicesEqual(args, observed.Args) {
		return OperationResult{}, &ConfigConflictError{Path: configPath}
	}
	block, err := buildUserBlock(name, spec)
	if err != nil {
		return OperationResult{}, err
	}
	content, err := replaceUserTable(obs.content, name, block)
	if err != nil {
		return OperationResult{}, err
	}
	if err := validateTOML(content); err != nil {
		return OperationResult{}, fmt.Errorf("validate updated global Codex config: %w", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		return OperationResult{}, err
	}
	if err := atomicWrite(ctx, configPath, content, info.Mode().Perm(), obs); err != nil {
		return OperationResult{}, err
	}
	return OperationResult{Action: "updated", State: StateManagedMatch, Block: block}, nil
}
