package app

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var _ = os.WriteFile
var _ = filepath.Join
var _ = config.Default
var _ = model.Symbol{}
var _ = reflect.DeepEqual
var _ = strings.Contains
var _ = json.Marshal
var _ = evidence.ModeSource

func cppAppAuditService(t *testing.T, files map[string]string) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	for path, src := range files {
		if e := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); e != nil {
			t.Fatal(e)
		}
		writeAppFile(t, filepath.Join(root, path), src)
	}
	s, e := NewWithConfig(root, config.Default())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	return s, root
}
func cppAppAuditGraph(t *testing.T, s *Service) (map[string]model.Symbol, []model.Relation) {
	t.Helper()
	syms, e := s.Store.Symbols(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	rs, e := s.Store.Relations(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	by := map[string]model.Symbol{}
	for _, v := range syms {
		by[v.Handle] = v
	}
	for _, r := range rs {
		t.Logf("EDGE %s:%s -> %s:%s unresolved=%s kind=%s confidence=%v source=%s", by[r.FromHandle].FilePath, by[r.FromHandle].QualifiedName, by[r.ToHandle].FilePath, by[r.ToHandle].QualifiedName, r.UnresolvedTo, r.Kind, r.Confidence, r.Source)
	}
	return by, rs
}
