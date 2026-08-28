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

func testConfig() config.Config {
	return config.Config{MaxFileBytes: 2 << 20, SecretExcludesEnabled: true, GenericChunkLines: 80, GenericChunkOverlap: 10, MaxCandidates: 200}
}
