package csharp_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/model"
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
		got := wholeAuditExtract(t, cpp.NewExtractor(), "x.cpp", "cpp", "#if 0\nvoid hidden() {}\n#elif 0\nvoid fake() {}\n#endif\nvoid real() {}\n")
		wholeAuditNoSymbol(t, got, "fake")
		wholeAuditSymbol(t, got, "real")
	})
	t.Run("csharp", func(t *testing.T) {
		got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", "#if false\nclass Hidden {}\n#elif false\nclass Fake {}\n#endif\nclass Real {}\n")
		wholeAuditNoSymbol(t, got, "Fake")
		wholeAuditSymbol(t, got, "Real")
	})
}

func TestF27CSharpVerbatimBackslashDoesNotMaskNextMethod(t *testing.T) {
	got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", "class A { string p = @\"C:\\\"; void F() {} }\n")
	wholeAuditSymbol(t, got, "F")
}

func TestF45CSharpNestedTypeKeepsOuterQualification(t *testing.T) {
	got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", "namespace N { class Outer { class Inner { void F() {} } } }\n")
	inner := wholeAuditSymbol(t, got, "Inner")
	f := wholeAuditSymbol(t, got, "F")
	if inner.QualifiedName != "N.Outer.Inner" || f.QualifiedName != "N.Outer.Inner.F" {
		t.Fatalf("inner=%+v method=%+v", inner, f)
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

func TestF27EscapedCharacterLiteralsKeepFollowingMethod(t *testing.T) {
	for _, literal := range []string{"'\\''", "'\\\\'"} {
		source := "class A { char c = " + literal + "; void F() {} }\n"
		got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", source)
		wholeAuditSymbol(t, got, "F")
	}
}

func TestF45SameNestedNamesUnderDifferentOuterTypesRemainDistinct(t *testing.T) {
	source := "namespace N { class First { class Inner { void F() {} } } class Second { class Inner { void F() {} } } }\n"
	got := wholeAuditExtract(t, csharp.NewExtractor(), "x.cs", "csharp", source)
	names := map[string]bool{}
	for _, s := range got.Symbols {
		names[s.QualifiedName] = true
	}
	for _, name := range []string{"N.First.Inner", "N.First.Inner.F", "N.Second.Inner", "N.Second.Inner.F"} {
		if !names[name] {
			t.Fatalf("missing %s: %+v", name, got.Symbols)
		}
	}
}
