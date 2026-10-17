package vb_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/vb"
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

func TestF30VBStringIsNotDeclaration(t *testing.T) {
	got := wholeAuditExtract(t, vb.NewVBNetExtractor(), "x.vb", "vbnet", "Class A\nSub F()\nDim s As String = \"Class Fake\"\nEnd Sub\nEnd Class\n")
	wholeAuditNoSymbol(t, got, "Fake")
}

func TestF24VBConstructorClosesAtEndSub(t *testing.T) {
	got := wholeAuditExtract(t, vb.NewVBNetExtractor(), "x.vb", "vbnet", "Class A\nPublic Sub New()\nEnd Sub\nPublic Sub After()\nEnd Sub\nEnd Class\n")
	owner := wholeAuditSymbol(t, got, "A")
	ctor := wholeAuditSymbol(t, got, "New")
	after := wholeAuditSymbol(t, got, "After")
	if ctor.EndLine != 3 || after.ParentHandle != owner.Handle {
		t.Fatalf("constructor=%+v after=%+v class=%+v", ctor, after, owner)
	}
	for _, d := range got.Diagnostics {
		if d.Code == "vbnet_unclosed_block" {
			t.Fatalf("valid source reported unclosed block: %+v", d)
		}
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

func TestF30LiteralTextDoesNotCreateCalls(t *testing.T) {
	source := "Class A\nSub Target()\nEnd Sub\nSub F()\nDim s As String = \"Target()\"\nTarget()\nEnd Sub\nEnd Class\n"
	got := wholeAuditExtract(t, vb.NewVBNetExtractor(), "x.vb", "vbnet", source)
	target := wholeAuditSymbol(t, got, "Target")
	caller := wholeAuditSymbol(t, got, "F")
	calls := 0
	for _, r := range got.Relations {
		if r.FromHandle == caller.Handle && r.Kind == "calls" && r.ToHandle == target.Handle {
			calls++
		}
	}
	if calls != 1 {
		t.Fatalf("real call missing or duplicate: %d", calls)
	}
	literalOnly := strings.Replace(source, "\nTarget()\n", "\n", 1)
	got = wholeAuditExtract(t, vb.NewVBNetExtractor(), "x.vb", "vbnet", literalOnly)
	for _, r := range got.Relations {
		if r.Kind == "calls" && r.ToHandle == target.Handle {
			t.Fatalf("literal call: %+v", r)
		}
	}
}

func TestF24SemanticMethodKindRetainsFunctionTerminator(t *testing.T) {
	for _, name := range []string{"TestValue", "On_Changed"} {
		source := "Class A\nPublic Function " + name + "() As Integer\nEnd Function\nPublic Sub After()\nEnd Sub\nEnd Class\n"
		got := wholeAuditExtract(t, vb.NewVBNetExtractor(), "x.vb", "vbnet", source)
		method := wholeAuditSymbol(t, got, name)
		after := wholeAuditSymbol(t, got, "After")
		owner := wholeAuditSymbol(t, got, "A")
		if method.EndLine != 3 || after.ParentHandle != owner.Handle {
			t.Fatalf("semantic method terminator: method=%+v after=%+v", method, after)
		}
	}
}
