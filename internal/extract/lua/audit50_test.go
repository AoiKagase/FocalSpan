package lua_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/lua"
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

func TestF31LuaNonCodeRequireDoesNotImport(t *testing.T) {
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", audit50Fixture(t, "F31", "source.lua"))
	real := false
	for _, r := range got.Relations {
		if r.Kind == "imports" {
			if r.UnresolvedTo == "fake" || r.UnresolvedTo == "also_fake" {
				t.Fatalf("non-code import: %+v", r)
			}
			if r.UnresolvedTo == "real" {
				real = true
			}
		}
	}
	if !real {
		t.Fatalf("real import missing: %+v", got.Relations)
	}
}

func TestF37LuaElseifDoesNotOpenExtraScope(t *testing.T) {
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", audit50Fixture(t, "F37", "source.lua"))
	f := wholeAuditSymbol(t, got, "f")
	g := wholeAuditSymbol(t, got, "g")
	if f.EndLine != 5 || g.ParentHandle == f.Handle {
		t.Fatalf("f=%+v g=%+v", f, g)
	}
}

func TestF31KeywordBoundariesAndLongNonCode(t *testing.T) {
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", "local x = prerequire('fake')\n--[=[require('comment')]=]\nlocal note = [=[require('string')]=]\nlocal real = require('real')\n")
	for _, relation := range got.Relations {
		if relation.Kind == "imports" && relation.UnresolvedTo != "real" {
			t.Fatalf("non-require import: %+v", relation)
		}
	}
}
func TestF37NestedElseifClosesExactlyOnce(t *testing.T) {
	source := "function f()\n if a then\n elseif(b) then\n  if c then\n  elseif d then\n  end\n else\n end\nend\nfunction g()\nend\n"
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", source)
	f := wholeAuditSymbol(t, got, "f")
	g := wholeAuditSymbol(t, got, "g")
	if f.EndLine != 9 || g.ParentHandle == f.Handle || g.EndLine != 11 {
		t.Fatalf("scope drift f=%+v g=%+v", f, g)
	}
}

func audit50Fixture(t *testing.T, id, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "audit50", id, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestF31OnlyLiteralRequireArgumentImports(t *testing.T) {
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", "local x=require(variable); local note='fake'\nlocal y=require('prefix' .. suffix)\nlocal z=require -- comment\n ('real')\nlocal a=require [=[long.real]=]\n")
	targets := map[string]bool{}
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			targets[relation.UnresolvedTo] = true
		}
	}
	if len(targets) != 2 || !targets["real"] || !targets["long.real"] {
		t.Fatalf("literal require imports=%v", targets)
	}
}

func TestF37ReviewInlineElseif(t *testing.T) {
	source := "function f()\n if a then x() elseif b then x() elseif c then x() end\nend\nfunction g()\nend\n"
	got := wholeAuditExtract(t, lua.NewExtractor(), "x.lua", "lua", source)
	f := wholeAuditSymbol(t, got, "f")
	g := wholeAuditSymbol(t, got, "g")
	if f.EndLine != 3 || g.ParentHandle == f.Handle {
		t.Fatalf("inline branch scope f=%+v g=%+v", f, g)
	}
}
