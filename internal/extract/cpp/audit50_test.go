package cpp_test

import (
	"context"
	"fmt"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"strings"
	"testing"
)

func wholeAuditExtract(t *testing.T, e extract.Extractor, path, language, source string) model.Extraction {
	t.Helper()
	got, err := e.Extract(context.Background(), model.SourceFile{Path: path, Language: language, Content: []byte(source)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	audit50SourceInvariants(t, got, source)
	for _, variant := range []string{strings.ReplaceAll(source, "\n", "\r\n"), "\xef\xbb\xbf" + source} {
		other, err := e.Extract(context.Background(), model.SourceFile{Path: path, Language: language, Content: []byte(variant)})
		if err != nil {
			t.Fatal(err)
		}
		audit50SourceInvariants(t, other, variant)
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

func TestF43ConstantFalseElifStaysInactive(t *testing.T) {
	t.Run("cpp", func(t *testing.T) {
		got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", audit50Fixture(t, "F43/cpp-1.cpp"))
		wholeAuditNoSymbol(t, got, "fake")
		wholeAuditSymbol(t, got, "real")
	})
	t.Run("csharp", func(t *testing.T) {
		got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", audit50Fixture(t, "F43/cpp-2.cs"))
		wholeAuditNoSymbol(t, got, "Fake")
		wholeAuditSymbol(t, got, "Real")
	})
}

func TestF26CPPForwardDeclarationStopsAtSemicolon(t *testing.T) {
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", audit50Fixture(t, "F26/cpp-1.cpp"))
	a := wholeAuditSymbol(t, got, "A")
	f := wholeAuditSymbol(t, got, "f")
	if a.EndLine != 1 || f.Kind != "function" {
		t.Fatalf("A=%+v f=%+v", a, f)
	}
}

func TestF44CPPReceiverOwnershipIsCaseSensitive(t *testing.T) {
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", audit50Fixture(t, "F44/cpp-1.cpp"))
	a := wholeAuditSymbol(t, got, "a")
	f := wholeAuditSymbol(t, got, "f")
	if f.ParentHandle != a.Handle {
		t.Fatalf("method=%+v lowercase receiver=%+v", f, a)
	}
}

func TestF42CPPNamespacedFreeFunctionIsFunction(t *testing.T) {
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", audit50Fixture(t, "F42/cpp-1.cpp"))
	f := wholeAuditSymbol(t, got, "f")
	if f.Kind != "function" {
		t.Fatalf("namespaced free function=%+v", f)
	}
}

func audit50SourceInvariants(t *testing.T, got model.Extraction, source string) {
	t.Helper()
	for _, symbol := range got.Symbols {
		if symbol.StartByte < 0 || symbol.EndByte < symbol.StartByte || symbol.EndByte > len(source) {
			t.Fatalf("invalid symbol range: %+v", symbol)
		}
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte < 0 || chunk.EndByte < chunk.StartByte || chunk.EndByte > len(source) {
			t.Fatalf("invalid chunk range: %+v", chunk)
		}
		if !strings.Contains(chunk.Kind, "outline") && chunk.Content != source[chunk.StartByte:chunk.EndByte] {
			t.Fatalf("chunk source differs: kind=%s bytes=%d:%d", chunk.Kind, chunk.StartByte, chunk.EndByte)
		}
	}
}

func TestF43ConstantBranchSelectionAndNesting(t *testing.T) {
	for _, fixture := range []struct {
		name, path, language, declaration string
		extractor                         extract.Extractor
	}{
		{"cpp", "x.cpp", "cpp", "void %s() {}", cpp.NewExtractor()},
		{"csharp", "x.cs", "csharp", "class %s {}", csharp.NewExtractor()},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			for _, source := range []string{
				"#if 0\n#elif 0\n" + fmt.Sprintf(fixture.declaration, "Fake") + "\n#else\n" + fmt.Sprintf(fixture.declaration, "Real") + "\n#endif\n",
				"#if 1\n" + fmt.Sprintf(fixture.declaration, "Real") + "\n#elif 1\n" + fmt.Sprintf(fixture.declaration, "Fake") + "\n#else\n" + fmt.Sprintf(fixture.declaration, "AlsoFake") + "\n#endif\n",
				"#if 0\n#if 1\n" + fmt.Sprintf(fixture.declaration, "Fake") + "\n#endif\n#else\n" + fmt.Sprintf(fixture.declaration, "Real") + "\n#endif\n",
			} {
				got := wholeAuditExtract(t, fixture.extractor, fixture.path, fixture.language, source)
				wholeAuditSymbol(t, got, "Real")
				wholeAuditNoSymbol(t, got, "Fake")
				wholeAuditNoSymbol(t, got, "AlsoFake")
			}
		})
	}
}

func TestF26ForwardDeclarationHasExactSemicolonEnd(t *testing.T) {
	source := "struct A;\nvoid f() {}\n"
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", source)
	a := wholeAuditSymbol(t, got, "A")
	if a.EndByte != len("struct A;") {
		t.Fatalf("forward range=%d", a.EndByte)
	}
}
func TestF42ClassQualifiedMemberRemainsMethod(t *testing.T) {
	source := "namespace demo { struct A { void f(); }; }\nvoid demo::A::f() {}\n"
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", source)
	for _, s := range got.Symbols {
		if s.Name == "f" && s.Kind != "method" {
			t.Fatalf("class member=%+v", s)
		}
	}
}

func audit50Fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/audit50/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReviewF43Regression(t *testing.T) {
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", audit50Fixture(t, "F43/review-x.cpp"))
	wholeAuditSymbol(t, got, "Real")
}

func TestReviewF43UnknownExpression(t *testing.T) {
	got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", "#if 1 && FEATURE\nvoid Maybe(){}\n#else\nvoid Alternate(){}\n#endif\n")
	wholeAuditSymbol(t, got, "Maybe")
	wholeAuditSymbol(t, got, "Alternate")
}

func TestReviewF43CommentedLiterals(t *testing.T) {
	for _, source := range []string{
		"#if 0 /* disabled branch */\nvoid Fake(){}\n#else\nvoid Real(){}\n#endif\n",
		"#if 0\nvoid Fake(){}\n#elif 0/* disabled */\nvoid AlsoFake(){}\n#else\nvoid Real(){}\n#endif\n",
		"#if 1/* true */\nvoid Real(){}\n#else\nvoid Fake(){}\n#endif\n",
	} {
		got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", source)
		wholeAuditSymbol(t, got, "Real")
		wholeAuditNoSymbol(t, got, "Fake")
		wholeAuditNoSymbol(t, got, "AlsoFake")
	}
}
