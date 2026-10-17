package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type auditConcurrentEditContext struct {
	context.Context
	path   string
	edited bool
}

func (c *auditConcurrentEditContext) Err() error {
	if !c.edited {
		matches, _ := filepath.Glob(filepath.Join(filepath.Dir(c.path), ".config.toml.focalspan-*"))
		if len(matches) > 0 {
			c.edited = true
			_ = os.WriteFile(c.path, []byte("user_setting = \"concurrent edit\"\n"), 0600)
		}
	}
	return nil
}
func TestAuditConcurrentConfigEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("user_setting = \"initial\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &auditConcurrentEditContext{Context: context.Background(), path: path}
	spec := RegistrationSpec{Command: "focalspan.exe", Args: []string{"mcp"}, Enabled: true, StartupTimeoutSec: 30, ToolTimeoutSec: 60, EnabledTools: append([]string(nil), EnabledTools...)}
	_, err := InstallProject(ctx, path, "focalspan", spec, false)
	if !ctx.edited {
		t.Fatal("concurrent edit was not injected")
	}
	data, _ := os.ReadFile(path)
	if err == nil && !containsAudit(string(data), "concurrent edit") {
		t.Fatalf("concurrent user edit silently overwritten: %s", data)
	}
}
func containsAudit(s, w string) bool {
	for i := 0; i+len(w) <= len(s); i++ {
		if s[i:i+len(w)] == w {
			return true
		}
	}
	return false
}

type pausedConfigContext struct {
	context.Context
	dir     string
	entered chan struct{}
	release chan struct{}
	paused  bool
}

func (c *pausedConfigContext) Err() error {
	if !c.paused {
		matches, _ := filepath.Glob(filepath.Join(c.dir, ".config.toml.focalspan-*"))
		if len(matches) > 0 {
			c.paused = true
			close(c.entered)
			<-c.release
		}
	}
	return c.Context.Err()
}
func TestProjectWritesSerialize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	spec := RegistrationSpec{Command: "focalspan.exe", Args: []string{"mcp"}, Enabled: true, StartupTimeoutSec: 30, ToolTimeoutSec: 60, EnabledTools: append([]string(nil), EnabledTools...)}
	first := &pausedConfigContext{Context: context.Background(), dir: dir, entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := InstallProject(first, path, "first", spec, false); done <- err }()
	select {
	case <-first.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first writer did not reach commit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err := InstallProject(ctx, path, "second", spec, false)
	cancel()
	close(first.release)
	if err == nil {
		t.Error("second writer bypassed configuration lock")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
