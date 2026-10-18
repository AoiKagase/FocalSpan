package cpp_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func cppAuditExtract(t *testing.T, source string) model.Extraction {
	t.Helper()
	x, e := cpp.NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/main.cpp", Language: "cpp", Content: []byte(source)})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range x.Chunks {
		if c.EndByte > c.StartByte && c.Content != source[c.StartByte:c.EndByte] {
			t.Errorf("source mismatch: %+v", c)
		}
	}
	return x
}
func cppAuditSymbol(t *testing.T, x model.Extraction, name string) model.Symbol {
	t.Helper()
	for _, s := range x.Symbols {
		if s.QualifiedName == name {
			return s
		}
	}
	t.Errorf("symbol %s missing", name)
	return model.Symbol{}
}
func cppAuditNoCall(t *testing.T, x model.Extraction, from, to string) {
	t.Helper()
	f := cppAuditSymbol(t, x, from)
	target := cppAuditSymbol(t, x, to)
	for _, r := range x.Relations {
		if r.FromHandle == f.Handle && r.ToHandle == target.Handle && r.Kind == "calls" {
			t.Errorf("false definite call: %+v", r)
		}
	}
}
