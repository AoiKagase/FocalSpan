package template_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	templateextract "github.com/focalspan/focalspan/internal/extract/template"
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

func TestF39PHPStringDoesNotCloseTemplateRegion(t *testing.T) {
	got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", audit50Fixture(t, "F39", "source.tpl"))
	wholeAuditNoSymbol(t, got, "fake")
	wholeAuditSymbol(t, got, "real")
}

func TestF46SmartyAttributeAcceptsSpaceBeforeEquals(t *testing.T) {
	got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", audit50Fixture(t, "F46", "source.tpl"))
	for _, r := range got.Relations {
		if r.Kind == "imports" && r.UnresolvedTo == "partials/header.tpl" {
			return
		}
	}
	t.Fatalf("spaced include attribute target incorrect: %+v", got.Relations)
}

type wholeAuditCancelGate struct {
	context.Context
	cancel context.CancelFunc
	after  int
	checks int
	fired  bool
}

func TestF39PHPQuotedAndCommentDelimiters(t *testing.T) {
	for _, body := range []string{"$x='?>{block name=\"fake\"}{/block}';", "$x=\"?>{block name='fake'}{/block}\";", "/* ?>{block name='fake'}{/block} */", "$x=<<<TEXT\n?>{block name='fake'}{/block}\nTEXT;\n"} {
		got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", "<?php "+body+" ?>\n{block name='real'}ok{/block}\n")
		wholeAuditNoSymbol(t, got, "fake")
		wholeAuditSymbol(t, got, "real")
	}
	got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", "<?php // ?>{block name='real'}ok{/block}")
	wholeAuditSymbol(t, got, "real")
}
func TestF46IndependentAssignmentSpacing(t *testing.T) {
	for _, attribute := range []string{"file='partials/header.tpl'", "file = 'partials/header.tpl'", "file= 'partials/header.tpl'", "file ='partials/header.tpl'"} {
		got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", "{include "+attribute+"}")
		count := 0
		for _, relation := range got.Relations {
			if relation.Kind == "imports" {
				count++
				if relation.UnresolvedTo != "partials/header.tpl" {
					t.Fatalf("attribute=%s relation=%+v", attribute, relation)
				}
			}
		}
		if count != 1 {
			t.Fatalf("imports=%d", count)
		}
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

func TestF39ReviewHeredocSuffix(t *testing.T) {
	source := "<?php echo <<<TEXT\n?> {include file='fake.tpl'}\nTEXT; ?>\n{include file='real.tpl'}"
	got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", source)
	found := false
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			if relation.UnresolvedTo == "fake.tpl" {
				t.Fatalf("PHP payload became import: %+v", relation)
			}
			if relation.UnresolvedTo == "real.tpl" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("following Smarty import missing")
	}
	regions, err := templateextract.Scan(context.Background(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, region := range regions {
		if region.Kind == templateextract.KindPHPBlock && region.EndByte != strings.Index(source, "TEXT; ?>")+len("TEXT; ?>") {
			t.Fatalf("PHP region has wrong end: %+v", region)
		}
	}
}

func TestF39ReviewHeredocNonASCIIContinuation(t *testing.T) {
	for _, continuation := range []string{"é", "suffix", "_", "1"} {
		source := "<?php echo <<<TEXT\nTEXT" + continuation + " ?> {include file='fake.tpl'}\nTEXT; ?>\n{include file='real.tpl'}"
		got := wholeAuditExtract(t, templateextract.NewExtractor(), "x.tpl", "smarty", source)
		found := false
		for _, relation := range got.Relations {
			if relation.Kind == "imports" {
				if relation.UnresolvedTo == "fake.tpl" {
					t.Fatalf("identifier continuation exposed PHP body: %+v", relation)
				}
				if relation.UnresolvedTo == "real.tpl" {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("real Smarty include missing")
		}
		regions, err := templateextract.Scan(context.Background(), []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		for _, region := range regions {
			if region.Kind == templateextract.KindPHPBlock && region.EndByte != strings.Index(source, "TEXT; ?>")+len("TEXT; ?>") {
				t.Fatalf("premature PHP region end: %+v", region)
			}
		}
	}
}
