package jsts_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/jsts"
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

func TestF34SemicolonlessImportDoesNotReadNextString(t *testing.T) {
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", audit50Fixture(t, "F34", "source.js"))
	for _, r := range got.Relations {
		if r.Kind == "imports" && r.UnresolvedTo == "not-a-module" {
			t.Fatalf("unrelated string imported: %+v", r)
		}
	}
}

func TestF35TypeScriptObjectAliasExists(t *testing.T) {
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.ts", "typescript", audit50Fixture(t, "F35", "source.ts"))
	s := wholeAuditSymbol(t, got, "User")
	if s.Kind != "type" {
		t.Fatalf("alias=%+v", s)
	}
}

func TestF36SingleParameterArrowExists(t *testing.T) {
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", audit50Fixture(t, "F36", "source.js"))
	s := wholeAuditSymbol(t, got, "double")
	if s.Kind != "arrow_function" {
		t.Fatalf("arrow=%+v", s)
	}
}

func TestF34MultilineAndSideEffectImportBoundaries(t *testing.T) {
	for _, source := range []string{"import {\n f\n} from './a'\nconst message='not-a-module';\nfunction real(){}\n", "import './a'\nconst message='not-a-module';\nfunction real(){}\n"} {
		got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", source)
		real := false
		for _, relation := range got.Relations {
			if relation.Kind == "imports" {
				if relation.UnresolvedTo == "not-a-module" {
					t.Fatalf("following literal imported: %+v", relation)
				}
				if relation.UnresolvedTo == "a" || relation.UnresolvedTo == "./a" || relation.UnresolvedTo == "a.js" {
					real = true
				}
			}
		}
		if !real {
			t.Fatal("real import missing")
		}
		wholeAuditSymbol(t, got, "real")
	}
}
func TestF35NestedObjectAliasAndFollowingDeclaration(t *testing.T) {
	source := "type User = { name: string; nested: {age: number}; };\nfunction real() {}\n"
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.ts", "typescript", source)
	user := wholeAuditSymbol(t, got, "User")
	wholeAuditSymbol(t, got, "real")
	if user.Kind != "type" || user.EndLine != 1 || user.EndByte != strings.Index(source, "\n") {
		t.Fatalf("type span: %+v", user)
	}
}
func TestF36SingleParameterAsyncAndBlockArrows(t *testing.T) {
	for _, source := range []string{"const double = value => { return value*2; };\nfunction real(){}\n", "const double = async value => value*2;\nfunction real(){}\n", "const double = (value) => value*2;\nfunction real(){}\n"} {
		got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", source)
		if wholeAuditSymbol(t, got, "double").Kind != "arrow_function" {
			t.Fatal("arrow kind")
		}
		wholeAuditSymbol(t, got, "real")
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

func TestF35UnionTupleAndGenericAliasSpans(t *testing.T) {
	for _, rhs := range []string{"{a:string;} | {b:number;}", "[{a:string;}, number]", "Box<{a:string;}>"} {
		source := "type User = " + rhs + ";\nfunction real(){}\n"
		got := wholeAuditExtract(t, jsts.NewExtractor(), "x.ts", "typescript", source)
		alias := wholeAuditSymbol(t, got, "User")
		wholeAuditSymbol(t, got, "real")
		if alias.Kind != "type" || alias.EndByte != strings.Index(source, "\n") {
			t.Fatalf("incomplete alias: %+v", alias)
		}
	}
}
func TestF36ArrowSpanAndOrdinaryVariable(t *testing.T) {
	source := "const double = value => value * 2;\nconst ordinary = 2;\n"
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", source)
	arrow := wholeAuditSymbol(t, got, "double")
	wholeAuditNoSymbol(t, got, "ordinary")
	if arrow.EndByte != strings.Index(source, "\n") || arrow.StartByte != 0 {
		t.Fatalf("incomplete arrow: %+v", arrow)
	}
}

func TestF34ReviewStringNamedImport(t *testing.T) {
	got := wholeAuditExtract(t, jsts.NewExtractor(), "x.js", "javascript", "import { \"a-b\" as ab } from './module.js'\nconst message='not-a-module';\nfunction caller(){ ab(); }\n")
	module, alias := false, false
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			if relation.Source == "jsts:import" && relation.UnresolvedTo == "module.js" {
				module = true
			}
			if relation.Source == "jsts:import-symbol" && relation.UnresolvedTo == "a-b" {
				alias = true
			}
			if relation.UnresolvedTo == "not-a-module" || relation.Source == "jsts:import" && relation.UnresolvedTo == "a-b" {
				t.Fatalf("non-module imported: %+v", relation)
			}
		}
	}
	if !module || !alias {
		t.Fatalf("module=%v alias=%v relations=%+v", module, alias, got.Relations)
	}
	wholeAuditSymbol(t, got, "caller")
}
