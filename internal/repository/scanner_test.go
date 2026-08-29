package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/config"
)

func TestScannerFiltersBinaryAndSecrets(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"src/main.go": []byte("package src\r\n\r\nfunc Main() {}\r\n"),
		".env":        []byte("TOKEN=secret"),
		"image.bin":   {0, 1, 2, 0, 3},
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := testConfig()
	got, diagnostics, err := NewScanner(root, cfg).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "src/main.go" || got[0].SHA256 == "" {
		t.Fatalf("files=%+v diagnostics=%+v", got, diagnostics)
	}
}

func TestDetectLanguageContentPHPAndIncRules(t *testing.T) {
	tests := []struct {
		name, path, content, want string
	}{
		{"php", "index.php", "", "php"},
		{"phtml", "template.phtml", "", "php"},
		{"uppercase-family", "legacy.PHP5", "", "php"},
		{"inc-php", "auth.inc", "<?php echo 1;", "php"},
		{"inc-short-echo", "view.inc", "<?= $title ?>", "php"},
		{"inc-short-tag", "short.inc", "<? echo 1;", "php"},
		{"inc-xml", "xml.inc", "<?xml version=\"1.0\"?>", "text"},
		{"inc-plain", "plain.inc", "plain text", "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectLanguageContent(tt.path, []byte(tt.content)); got != tt.want {
				t.Fatalf("DetectLanguageContent(%q)=%q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectLanguageContentTemplates(t *testing.T) {
	tests := []struct {
		name, path, content, want string
	}{
		{"smarty block", "login.tpl", `{block name="content"}{/block}`, "smarty"},
		{"smarty variable", "header.tpl", `{$title}`, "smarty"},
		{"plain html", "plain.tpl", `<p>Hello</p>`, "template"},
		{"plain text", "mail.tpl", "Hello from a template", "template"},
		{"double curly tag", "double.tpl", `{{ user.name }}`, "template"},
		{"double curly Smarty-looking tag", "double-block.tpl", `{{block}}`, "template"},
		{"php only", "legacy.tpl", `<?php echo $title; ?>`, "php"},
		{"xml declaration", "document.tpl", `<?xml version="1.0"?><root/>`, "template"},
		{"smarty extension", "theme.SMARTY", `plain text`, "smarty"},
		{"uppercase tpl", "page.HTML.TPL", `<p>{$title}</p>`, "smarty"},
		{"javascript braces", "script.tpl", `<script>function f() { return {}; }</script>`, "template"},
		{"javascript string marker", "script-string.tpl", `<script>const marker = "{block}";</script>`, "template"},
		{"css braces", "style.tpl", `<style>.x { color: red; }</style>`, "template"},
		{"comment marker", "comment.tpl", `{* {block name="fake"} *}`, "smarty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectLanguageContent(tt.path, []byte(tt.content)); got != tt.want {
				t.Fatalf("DetectLanguageContent(%q)=%q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestScannerDetectsPHPIncContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "auth.inc"), []byte("<?PHP echo 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := NewScanner(root, testConfig()).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Language != "php" {
		t.Fatalf("files=%+v diagnostics=%+v", files, diagnostics)
	}
}

func testConfig() config.Config {
	return config.Config{MaxFileBytes: 2 << 20, SecretExcludesEnabled: true, GenericChunkLines: 80, GenericChunkOverlap: 10, MaxCandidates: 200}
}
