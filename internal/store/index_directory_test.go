package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIndexDirectoryRejectsOutsideJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction")
	}
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "final", true: "parent"}[nested], func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			link := filepath.Join(root, "cache")
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
				t.Skipf("junction unavailable: %v %s", err, out)
			}
			defer os.Remove(link)
			directory := "cache"
			if nested {
				directory = "cache/index"
			}
			s, err := Open(root, directory)
			if err == nil {
				s.Close()
				t.Error("root-external index directory accepted")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("outside modified: %v", entries)
			}
		})
	}
}

func TestIndexDirectoryAllowsInsideJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction")
	}
	root := t.TempDir()
	target := filepath.Join(root, "state")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "cache")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v %s", err, out)
	}
	defer os.Remove(link)
	s, err := Open(root, "cache")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if filepath.Dir(s.DBPath()) != target {
		t.Fatalf("DB path=%s want target=%s", s.DBPath(), target)
	}
}
