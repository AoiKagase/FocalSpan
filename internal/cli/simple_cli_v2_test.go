package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandHelpIsSuccessfulAndHidesInternalCommands(t *testing.T) {
	for _, args := range [][]string{{"setup", "--help"}, {"help", "mcp"}} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 0 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, errOut.String())
		}
		if !strings.Contains(out.String(), "usage:") {
			t.Fatalf("args=%v output=%q", args, out.String())
		}
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"--help"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	if strings.Contains(out.String(), " serve") || strings.Contains(out.String(), " eval") {
		t.Fatalf("internal command leaked into help: %q", out.String())
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package sample\n\nfunc Ready() bool { return true }\n")
	for attempt := 0; attempt < 2; attempt++ {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), []string{"setup", "--root", root, "--json"}, &out, &errOut); code != 0 {
			t.Fatalf("attempt=%d code=%d stderr=%q", attempt, code, errOut.String())
		}
	}
}

func TestSetupPreservesExistingValidConfiguration(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package sample\n\nfunc Ready() bool { return true }\n")
	configPath := filepath.Join(root, ".focalspan.json")
	original := []byte("{\n  \"default_token_budget\": 2048\n}\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"setup", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("setup overwrote existing config:\n%s", got)
	}
}

func TestStatusReportsInvalidConfigurationAsJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".focalspan.json"), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"status", "--root", root, "--json"}, &out, &errOut); code == 0 {
		t.Fatalf("status unexpectedly succeeded: %s", out.String())
	}
	if !strings.Contains(out.String(), `"config_valid": false`) || !strings.Contains(out.String(), `"diagnostics"`) {
		t.Fatalf("status JSON did not diagnose invalid config: stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestDoubleDashAllowsReservedWordQuery(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "status.go"), "package sample\n\nfunc Status() string { return \"ready\" }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"--root", root, "--", "status"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Status") {
		t.Fatalf("output=%q", out.String())
	}
}
