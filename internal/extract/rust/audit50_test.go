package rust_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/rust"
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

func TestF25RustCharacterDoesNotOpenScope(t *testing.T) {
	got := wholeAuditExtract(t, rust.NewExtractor(), "x.rs", "rust", "fn first() { let brace = '{'; }\nfn second() {}\n")
	first := wholeAuditSymbol(t, got, "first")
	second := wholeAuditSymbol(t, got, "second")
	if first.EndLine != 1 || second.ParentHandle == first.Handle {
		t.Fatalf("first=%+v second=%+v", first, second)
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

func TestF25CharacterEscapesDoNotChangeScopeAndLifetimesRemainCode(t *testing.T) {
	for _, literal := range []string{"'{'", "'}'", "' '", "'\\u{7b}'", "'\\''", "b'{'"} {
		source := "fn first<'a>(s: &'a str) { let value = " + literal + "; }\nfn second() {}\n"
		got := wholeAuditExtract(t, rust.NewExtractor(), "x.rs", "rust", source)
		first := wholeAuditSymbol(t, got, "first")
		second := wholeAuditSymbol(t, got, "second")
		if first.EndLine != 1 || second.ParentHandle == first.Handle {
			t.Fatalf("literal=%s first=%+v second=%+v", literal, first, second)
		}
	}
}

func TestF25CharacterTokensIncludeEntireEscapedOrUnicodeLiteral(t *testing.T) {
	for _, source := range []string{"'\\''", "'\\\\'", "'あ'", "'\\n'", "'{'"} {
		tokens, _, err := rust.Lex(context.Background(), []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		if len(tokens) != 1 || tokens[0].Kind != rust.Character || tokens[0].Text != source {
			t.Fatalf("literal %s tokens=%+v", source, tokens)
		}
	}
}
