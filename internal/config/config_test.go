package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultAndWarnUnknownKey(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(`{"default_token_budget":1200,"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultTokenBudget != 1200 || len(warnings) != 1 || !strings.Contains(warnings[0], "unknown") {
		t.Fatalf("cfg=%+v warnings=%v", cfg, warnings)
	}
}

func TestLoadRejectsWrongType(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(`{"workers":"many"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("error=%v", err)
	}
}

func TestWriteDefaultDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefault(root, false); err == nil {
		t.Fatal("expected existing config error")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "keep" {
		t.Fatalf("config overwritten: %q", b)
	}
}

func TestLoadLanguageOverrides(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(`{"language_overrides":{"**/*.inc":"pawn","scripts/**/*.bas":"vb6"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LanguageOverrides["**/*.inc"] != "pawn" || cfg.LanguageOverrides["scripts/**/*.bas"] != "vb6" {
		t.Fatalf("language overrides not loaded: %+v", cfg.LanguageOverrides)
	}
}

func TestValidateRejectsInvalidLanguageOverrides(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  string
		lang string
	}{
		{"unknown language", "**/*.inc", "made-up"},
		{"invalid glob", "**/[.inc", "php"},
		{"parent escape", "../outside/*.inc", "php"},
		{"absolute path", `C:\outside\*.inc`, "php"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.LanguageOverrides = map[string]string{tt.key: tt.lang}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate accepted invalid override %q=%q", tt.key, tt.lang)
			}
		})
	}
}
