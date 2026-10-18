package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestR01ShadowedCallsStayUnlinked(t *testing.T) {
	source, e := os.ReadFile("../../testdata/regressions/residual11/jsts-shadow.ts")
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	if e = os.WriteFile(filepath.Join(root, "a.ts"), source, 0600); e != nil {
		t.Fatal(e)
	}
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	symbols, e := s.Store.Symbols(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	for _, v := range symbols {
		names[v.Handle] = v.Name
	}
	relations, e := s.Store.Relations(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range relations {
		if names[r.FromHandle] == "caller" && r.Kind != "contains" && names[r.ToHandle] == "target" {
			t.Errorf("shadowed target resolved after linking: %+v", r)
		}
	}
}
