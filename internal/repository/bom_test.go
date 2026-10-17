package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestScannerPreservesUTF8BOMBytesAndIdentity(t *testing.T) {
	root := t.TempDir()
	raw := []byte("\xef\xbb\xbfpackage auth\r\nfunc ValidateToken() {}\r\n")
	if err := os.WriteFile(filepath.Join(root, "auth.go"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := NewScanner(root, testConfig()).Scan(context.Background())
	if err != nil || len(files) != 1 || len(diagnostics) != 0 {
		t.Fatalf("scan files=%+v diagnostics=%+v err=%v", files, diagnostics, err)
	}
	file := files[0]
	digest := sha256.Sum256(raw)
	if !bytes.Equal(file.Content, raw) || file.SizeBytes != int64(len(raw)) || file.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("scanner changed original bytes/identity: %+v", file)
	}
}
