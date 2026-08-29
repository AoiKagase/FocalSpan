package codex

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	managedBeginPrefix = "# BEGIN FOCALSPAN MANAGED MCP: "
	managedEndPrefix   = "# END FOCALSPAN MANAGED MCP: "
)

func managedBeginMarker(name string) string { return managedBeginPrefix + name }
func managedEndMarker(name string) string   { return managedEndPrefix + name }

func BuildManagedBlock(name string, spec RegistrationSpec) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := validateTOMLValues(spec); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", managedBeginMarker(name))
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", name)
	fmt.Fprintf(&b, "command = %s\n", quoteTOML(spec.Command))
	b.WriteString("args = [")
	for i, arg := range spec.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteTOML(arg))
	}
	b.WriteString("]\n")
	fmt.Fprintf(&b, "enabled = %t\n", spec.Enabled)
	fmt.Fprintf(&b, "startup_timeout_sec = %d\n", spec.StartupTimeoutSec)
	fmt.Fprintf(&b, "tool_timeout_sec = %d\n", spec.ToolTimeoutSec)
	b.WriteString("enabled_tools = [")
	for i, tool := range spec.EnabledTools {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteTOML(tool))
	}
	b.WriteString("]\n")
	fmt.Fprintf(&b, "%s\n", managedEndMarker(name))
	return b.String(), nil
}

func validateTOMLValues(spec RegistrationSpec) error {
	values := append([]string{spec.Command}, spec.Args...)
	values = append(values, spec.EnabledTools...)
	for _, value := range values {
		if !utf8.ValidString(value) {
			return errors.New("MCP registration contains invalid UTF-8")
		}
		if strings.IndexByte(value, 0) >= 0 {
			return errors.New("MCP registration contains NUL")
		}
		if strings.ContainsAny(value, "\r\n") {
			return errors.New("MCP registration value contains a newline")
		}
	}
	if spec.StartupTimeoutSec <= 0 || spec.ToolTimeoutSec <= 0 {
		return errors.New("MCP registration timeouts must be positive")
	}
	if !stringSlicesEqual(spec.EnabledTools, EnabledTools) {
		return errors.New("MCP registration must expose exactly the five FocalSpan tools")
	}
	return nil
}

func quoteTOML(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

type managedBounds struct {
	start int
	end   int
}

func findManagedBounds(content, name string) (managedBounds, bool, error) {
	begin := managedBeginMarker(name)
	end := managedEndMarker(name)
	var foundBegin, foundEnd []int
	for offset := 0; offset < len(content); {
		next := strings.IndexByte(content[offset:], '\n')
		lineEnd := len(content)
		if next >= 0 {
			lineEnd = offset + next + 1
		}
		line := strings.TrimSuffix(content[offset:lineEnd], "\n")
		line = strings.TrimSuffix(line, "\r")
		if line == begin {
			foundBegin = append(foundBegin, offset)
		}
		if line == end {
			foundEnd = append(foundEnd, offset)
		}
		if next < 0 {
			break
		}
		offset = lineEnd
	}
	if len(foundBegin) == 0 && len(foundEnd) == 0 {
		return managedBounds{}, false, nil
	}
	if len(foundBegin) != 1 || len(foundEnd) != 1 || foundEnd[0] < foundBegin[0] {
		return managedBounds{}, false, fmt.Errorf("invalid FocalSpan managed block markers for server %q", name)
	}
	endOffset := foundEnd[0]
	if newline := strings.IndexByte(content[endOffset:], '\n'); newline >= 0 {
		endOffset += newline + 1
	} else {
		endOffset = len(content)
	}
	return managedBounds{start: foundBegin[0], end: endOffset}, true, nil
}
