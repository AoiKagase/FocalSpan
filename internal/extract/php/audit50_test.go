package php_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/php"
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

func TestF38PHPLineCommentStillHonorsClosingTag(t *testing.T) {
	got := wholeAuditExtract(t, php.NewExtractor(), "x.php", "php", audit50Fixture(t, "F38", "source.php"))
	wholeAuditNoSymbol(t, got, "fake")
}

func TestF38ClosingTagCommentFormsAndResume(t *testing.T) {
	for _, comment := range []string{"// comment", "# comment"} {
		got := wholeAuditExtract(t, php.NewExtractor(), "x.php", "php", "<?php "+comment+" ?>\nfunction fake() {}\n<?php function real() {}\n")
		wholeAuditNoSymbol(t, got, "fake")
		if wholeAuditSymbol(t, got, "real").StartLine != 3 {
			t.Fatal("resumed PHP position")
		}
	}
	got := wholeAuditExtract(t, php.NewExtractor(), "x.php", "php", "<?php /* ?> */ function real() {}")
	wholeAuditSymbol(t, got, "real")
}

func audit50Fixture(t *testing.T, id, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "audit50", id, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
