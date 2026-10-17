package repository

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestScannerRejectsNULAnywhere(t *testing.T) {
	for _, offset := range []int{8191, 8192, 12000} {
		t.Run(strconv.Itoa(offset), func(t *testing.T) {
			root := t.TempDir()
			content := strings.Repeat("a", offset) + "\x00suffix"
			if err := os.WriteFile(filepath.Join(root, "binary.txt"), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			files, _, err := NewScanner(root, config.Default()).Scan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 0 {
				t.Fatalf("NUL at %d indexed", offset)
			}
		})
	}
}
