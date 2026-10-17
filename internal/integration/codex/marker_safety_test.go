package codex

import (
	"context"
	"os"
	"path/filepath"
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
