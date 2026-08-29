package codex

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestBuildManagedBlockEscapesPathsAndIsDeterministic(t *testing.T) {
	spec := RegistrationSpec{
		Command:           `C:\Tools\Focal Span\focalspan.exe`,
		Args:              []string{"serve", "--root", `H:\source code\日本語\repo`, "--no-auto-update"},
		Enabled:           true,
		StartupTimeoutSec: 30,
		ToolTimeoutSec:    60,
		EnabledTools:      append([]string(nil), EnabledTools...),
	}

	first, err := BuildManagedBlock("focalspan", spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildManagedBlock("focalspan", spec)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("managed block is not deterministic")
	}
	if !strings.Contains(first, `command = "C:\\Tools\\Focal Span\\focalspan.exe"`) {
		t.Fatalf("command was not TOML escaped: %s", first)
	}
	if !strings.Contains(first, `args = ["serve", "--root", "H:\\source code\\日本語\\repo", "--no-auto-update"]`) {
		t.Fatalf("args were not TOML escaped: %s", first)
	}
	if strings.Count(first, "code_") != 5 {
		t.Fatalf("unexpected enabled tools: %s", first)
	}
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(first), &parsed); err != nil {
		t.Fatalf("generated block is invalid TOML: %v\n%s", err, first)
	}
}

func TestBuildManagedBlockRejectsUnsafeNameAndValues(t *testing.T) {
	spec := RegistrationSpec{Command: "focalspan", Args: []string{"serve", "--root", "/tmp/repo"}}
	for _, name := range []string{"", "bad.name", "bad\nname", strings.Repeat("x", 65)} {
		if _, err := BuildManagedBlock(name, spec); err == nil {
			t.Fatalf("name %q was accepted", name)
		}
	}
	spec.Command = "bad\x00command"
	if _, err := BuildManagedBlock("focalspan", spec); err == nil {
		t.Fatal("NUL-containing command was accepted")
	}
}
