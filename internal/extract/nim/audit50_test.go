package nim_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/nim"
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

func TestF28NimTripleStringIsNotDeclaration(t *testing.T) {
	got := wholeAuditExtract(t, nim.NewExtractor(), "x.nim", "nim", "let doc = \"\"\"\nproc fake() = discard\n\"\"\"\nproc real() = discard\n")
	wholeAuditNoSymbol(t, got, "fake")
	wholeAuditSymbol(t, got, "real")
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

func TestF28TripleStringHeaderRetainsOriginalSource(t *testing.T) {
	source := "let doc = \"\"\"\nproc fake() = discard\n\"\"\"\nproc real() = discard\n"
	got := wholeAuditExtract(t, nim.NewExtractor(), "x.nim", "nim", source)
	doc := wholeAuditSymbol(t, got, "doc")
	if doc.Signature != "let doc = \"\"\"" {
		t.Fatalf("source header changed: %q", doc.Signature)
	}
}

func TestF28MultilineNonCodeDoesNotCreateDeclarationsOrRelations(t *testing.T) {
	for _, literal := range []string{"\"\"\"\nproc fake() = target()\nimport phantom\n\"\"\"", "#[ outer\n#[ nested ]#\nproc fake() = target()\nimport phantom\n]#"} {
		source := "proc target() = discard\nlet doc = " + literal + "\nproc real() = discard\n"
		got := wholeAuditExtract(t, nim.NewExtractor(), "x.nim", "nim", source)
		wholeAuditNoSymbol(t, got, "fake")
		wholeAuditSymbol(t, got, "real")
		for _, r := range got.Relations {
			if r.Kind == "imports" && r.UnresolvedTo == "phantom" || r.Kind == "calls" && (r.UnresolvedTo == "fake" || r.ToHandle == wholeAuditSymbol(t, got, "target").Handle) {
				t.Fatalf("non-code relation: %+v", r)
			}
		}
	}
}
