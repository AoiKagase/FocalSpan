package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditMarkersInsideTOMLString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte("note = '''\n# BEGIN FOCALSPAN MANAGED MCP: focalspan\nexample\n# END FOCALSPAN MANAGED MCP: focalspan\n'''\n[mcp_servers.focalspan]\ncommand = \"user-server\"\nargs = []\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	spec := RegistrationSpec{Command: "focalspan.exe", Args: []string{"mcp"}, Enabled: true, StartupTimeoutSec: 30, ToolTimeoutSec: 60, EnabledTools: append([]string(nil), EnabledTools...)}
	result, err := InstallProject(context.Background(), path, "focalspan", spec, false)
	if err == nil {
		t.Fatalf("unmanaged server incorrectly updated using markers inside a string: %+v", result)
	}
}

func TestMarkersOnlyOutsideTOMLStrings(t *testing.T) {
	for _, quote := range []string{"'''", `"""`} {
		content := "note = " + quote + "\n" + managedBeginMarker("focalspan") + "\nexample\n" + managedEndMarker("focalspan") + "\n" + quote + "\n" + managedBeginMarker("focalspan") + "\n[mcp_servers.focalspan]\ncommand = \"server\"\n" + managedEndMarker("focalspan") + "\n"
		if err := validateTOML([]byte(content)); err != nil {
			t.Fatal(err)
		}
		bounds, managed, err := findManagedBounds(content, "focalspan")
		if err != nil || !managed {
			t.Fatalf("outside marker missed: %v", err)
		}
		if bounds.start != strings.LastIndex(content, managedBeginMarker("focalspan")) {
			t.Fatalf("string marker selected: %+v", bounds)
		}
	}
}
