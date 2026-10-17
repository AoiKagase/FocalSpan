package benchcli

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestF06OutputTempOwnership(t *testing.T) {
	t.Run("unowned temporary", func(t *testing.T) {
		root := t.TempDir()
		out := filepath.Join(root, "report.json")
		tmp := out + ".tmp"
		if err := os.WriteFile(tmp, []byte("sentinel"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := writeOutput(out, []byte("report"), false); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(tmp)
		if err != nil || string(got) != "sentinel" {
			t.Fatalf("unowned tmp changed: %q, %v", got, err)
		}
	})
	t.Run("one winner without force", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "report.json")
		start := make(chan struct{})
		wins := make(chan string, 8)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				data := fmt.Sprintf("report-%d", i)
				if writeOutput(out, []byte(data), false) == nil {
					wins <- data
				}
			}(i)
		}
		close(start)
		wg.Wait()
		close(wins)
		var winner []string
		for v := range wins {
			winner = append(winner, v)
		}
		if len(winner) != 1 {
			t.Fatalf("successful writers=%d, want one", len(winner))
		}
		got, err := os.ReadFile(out)
		if err != nil || string(got) != winner[0] {
			t.Fatalf("winner lost: %q %v", got, err)
		}
	})
	t.Run("failed publish cleanup and force", func(t *testing.T) {
		root := t.TempDir()
		out := filepath.Join(root, "directory")
		if err := os.Mkdir(out, 0700); err != nil {
			t.Fatal(err)
		}
		if err := writeOutput(out, []byte("x"), true); err == nil {
			t.Fatal("cannot replace directory with report")
		}
		files, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Fatalf("owned tmp not cleaned up: %v", files)
		}
		file := filepath.Join(root, "report")
		if err := os.WriteFile(file, []byte("old"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := writeOutput(file, []byte("new"), false); err == nil {
			t.Fatal("existing output overwritten without force")
		}
		if err := writeOutput(file, []byte("new"), true); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(file)
		if string(got) != "new" {
			t.Fatalf("force output=%q", got)
		}
	})
}
