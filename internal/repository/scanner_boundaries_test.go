package repository

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestScannerRejectsSecretCaseVariants(t *testing.T) {
	for _, name := range []string{".ENV", "Credentials.json", "ID_RSA"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, name), []byte("DUMMY=private"), 0600); err != nil {
				t.Fatal(err)
			}
			files, _, err := NewScanner(root, config.Default()).Scan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 0 {
				t.Fatalf("secret indexed: %+v", files)
			}
			cfg := config.Default()
			cfg.Include = []string{name}
			files, _, err = NewScanner(root, cfg).Scan(context.Background())
			if err != nil || len(files) != 1 {
				t.Fatalf("explicit include: %+v %v", files, err)
			}
		})
	}
}
