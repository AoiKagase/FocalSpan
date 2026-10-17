package python_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/python"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func wholeAuditExtract(t *testing.T, e extract.Extractor, path, language, source string) model.Extraction {
	t.Helper()
	got, err := e.Extract(context.Background(), model.SourceFile{Path: path, Language: language, Content: []byte(source)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, c := range got.Chunks {
		if c.EndByte > c.StartByte && (c.StartByte < 0 || c.EndByte > len(source) || c.Content != source[c.StartByte:c.EndByte]) {
			t.Fatalf("source fidelity: %+v", c)
		}
	}
	for _, s := range got.Symbols {
		if s.StartByte >= 0 && s.StartByte <= len(source) && s.StartLine != 1+strings.Count(source[:s.StartByte], "\n") {
			t.Fatalf("line fidelity: %+v", s)
		}
	}
	return got
}

func wholeAuditSymbol(t *testing.T, got model.Extraction, name string) model.Symbol {
	t.Helper()
	for _, s := range got.Symbols {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("missing symbol %q: %+v", name, got.Symbols)
	return model.Symbol{}
}

func wholeAuditNoSymbol(t *testing.T, got model.Extraction, name string) {
	t.Helper()
	for _, s := range got.Symbols {
		if s.Name == name {
			t.Fatalf("non-code symbol %q: %+v", name, s)
		}
	}
}

func TestF40PythonShadowedCallNotResolvedToGlobal(t *testing.T) {
	got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", audit50Fixture(t, "F40", "source.py"))
	target := wholeAuditSymbol(t, got, "target")
	caller := wholeAuditSymbol(t, got, "caller")
	for _, r := range got.Relations {
		if r.Kind == "calls" && r.FromHandle == caller.Handle && r.ToHandle == target.Handle {
			t.Fatalf("shadowed call resolved globally: %+v", r)
		}
	}
}

func TestF40ReceiverAndBindingsStayUnresolved(t *testing.T) {
	for _, body := range []string{"def caller(target):\n target()\n", "def caller():\n other.target()\n", "def caller():\n target = other\n target()\n"} {
		got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\n"+body)
		target := wholeAuditSymbol(t, got, "target")
		caller := wholeAuditSymbol(t, got, "caller")
		unresolved := false
		for _, relation := range got.Relations {
			if relation.FromHandle == caller.Handle && relation.Kind == "calls" {
				if relation.ToHandle == target.Handle {
					t.Fatalf("unrelated binding resolved: %+v", relation)
				}
				if relation.UnresolvedTo != "" {
					unresolved = true
				}
			}
		}
		if !unresolved {
			t.Fatal("uncertain call should survive unresolved")
		}
	}
	got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\ndef caller():\n target()\n")
	target := wholeAuditSymbol(t, got, "target")
	caller := wholeAuditSymbol(t, got, "caller")
	for _, relation := range got.Relations {
		if relation.FromHandle == caller.Handle && relation.ToHandle == target.Handle && relation.Kind == "calls" {
			return
		}
	}
	t.Fatal("ordinary call missing")
}

func TestF40MultilineParameterBinding(t *testing.T) {
	got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\ndef caller(\n target,\n):\n target()\n")
	target := wholeAuditSymbol(t, got, "target")
	caller := wholeAuditSymbol(t, got, "caller")
	for _, relation := range got.Relations {
		if relation.FromHandle == caller.Handle && relation.ToHandle == target.Handle && relation.Kind == "calls" {
			t.Fatalf("multiline parameter resolved globally: %+v", relation)
		}
	}
}

func TestF40ImportBindingAndCurrentReceiver(t *testing.T) {
	for _, statement := range []string{"import other as target", "from other import target", "from other import value as target"} {
		got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\ndef caller():\n "+statement+"\n target()\n")
		target := wholeAuditSymbol(t, got, "target")
		caller := wholeAuditSymbol(t, got, "caller")
		for _, relation := range got.Relations {
			if relation.FromHandle == caller.Handle && relation.ToHandle == target.Handle && relation.Kind == "calls" {
				t.Fatalf("import binding resolved globally: %+v", relation)
			}
		}
	}
	got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\nclass A:\n def target(self):\n  pass\n def caller(self):\n  self.target()\n")
	var method model.Symbol
	caller := wholeAuditSymbol(t, got, "caller")
	for _, symbol := range got.Symbols {
		if symbol.QualifiedName == "A.target" {
			method = symbol
		}
	}
	for _, relation := range got.Relations {
		if relation.FromHandle == caller.Handle && relation.ToHandle == method.Handle && relation.Kind == "calls" {
			return
		}
	}
	t.Fatal("current class receiver call missing")
}

func audit50Fixture(t *testing.T, id, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "audit50", id, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestF40NestedScopeBindingLifetime(t *testing.T) {
	got := wholeAuditExtract(t, python.NewExtractor(), "x.py", "python", "def target():\n pass\ndef outer():\n def inner():\n  target = other\n target()\n")
	target := wholeAuditSymbol(t, got, "target")
	outer := wholeAuditSymbol(t, got, "outer")
	for _, relation := range got.Relations {
		if relation.FromHandle == outer.Handle && relation.ToHandle == target.Handle && relation.Kind == "calls" {
			return
		}
	}
	t.Fatal("nested local incorrectly shadows enclosing scope")
}
