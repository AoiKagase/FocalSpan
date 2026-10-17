//go:build linux

package linker

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestF21LinuxDistinctCaseFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Foo.ts", "foo.ts"} {
		if err := os.WriteFile(filepath.Join(root, "src", name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "src"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("case-sensitive acceptance unavailable: %+v %v", entries, err)
	}
	s, err := store.Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, entry := range entries {
		path := "src/" + entry.Name()
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || string(content) != entry.Name() {
			t.Fatalf("file identity %s: %s %v", path, content, err)
		}
		fsAuditCoreSeed(t, s, path, entry.Name(), entry.Name(), nil)
	}
	fsAuditCoreSeed(t, s, "src/main.ts", "main", "main", []model.Relation{{FromHandle: "main", UnresolvedTo: "./Foo.ts", Kind: "imports", Confidence: 1}})
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	rels, err := s.Relations(context.Background())
	if err != nil || len(rels) != 1 || rels[0].ToHandle != "Foo.ts" {
		t.Fatalf("linux case result=%+v %v", rels, err)
	}
}
