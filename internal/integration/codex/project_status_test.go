package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectStatusComparesFullRegistration(t *testing.T) {
	for _, tc := range []struct{ field, value string }{{"enabled", "false"}, {"enabled_tools", "[\"wrong_tool\"]"}, {"startup_timeout_sec", "31"}, {"tool_timeout_sec", "61"}, {"command", "\"D:\\\\Other\\\\focalspan.exe\""}, {"args", "[\"serve\", \"--root\", \"different\"]"}} {
		t.Run(tc.field, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".codex", "config.toml")
			spec := testRegistrationSpec()
			spec.Args = []string{"serve", "--root", root}
			if _, err := InstallProject(context.Background(), path, "focalspan", spec, false); err != nil {
				t.Fatal(err)
			}
			if before := StatusProject(path, root, "focalspan", spec); !before.Matches {
				t.Fatalf("invalid normal fixture: %+v", before)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.HasPrefix(line, tc.field+" = ") {
					lines[i] = tc.field + " = " + tc.value
				}
			}
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
				t.Fatal(err)
			}
			status := StatusProject(path, root, "focalspan", spec)
			if status.Matches || status.State != StateManagedDrift {
				t.Fatalf("drift reported matching: %+v", status)
			}
		})
	}
}
