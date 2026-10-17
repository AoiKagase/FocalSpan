package resx_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/resx"
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

func TestF32RESXCommentAndCDATAAreNotResources(t *testing.T) {
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", audit50Fixture(t, "F32", "source.resx"))
	wholeAuditNoSymbol(t, got, "Fake")
	wholeAuditNoSymbol(t, got, "AlsoFake")
	wholeAuditSymbol(t, got, "Real")
}

func TestF33RESXStandardFileRefImportsAsset(t *testing.T) {
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", audit50Fixture(t, "F33", "source.resx"))
	for _, r := range got.Relations {
		if r.Kind == "imports" && r.UnresolvedTo == "../Resources/logo.png" {
			return
		}
	}
	t.Fatalf("standard ResXFileRef import missing: %+v", got.Relations)
}

func TestF32CommentImportIsNotAnAsset(t *testing.T) {
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", "<root>\n<!-- <ResXFileRef>fake.png;type</ResXFileRef> -->\n<![CDATA[<ResXFileRef>also.png;type</ResXFileRef>]]>\n<data name=\"Real\"/>\n</root>")
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			t.Fatalf("nonmarkup asset: %+v", relation)
		}
	}
	if wholeAuditSymbol(t, got, "Real").StartLine != 4 {
		t.Fatal("resource line drift")
	}
}
func TestF33OnlyFileRefTypeImportsValue(t *testing.T) {
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", "<root>\n<data name=\"Text\"><value>text;not-an-asset</value></data>\n<data name=\"Logo\" type=\"System.Resources.ResXFileRef, System.Windows.Forms\"><value>images/a&amp;b.png;System.Drawing.Bitmap</value></data>\n</root>")
	count := 0
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			count++
			if relation.UnresolvedTo != "images/a&b.png" {
				t.Fatalf("bad asset: %+v", relation)
			}
		}
	}
	if count != 1 {
		t.Fatalf("imports=%d", count)
	}
	wholeAuditSymbol(t, got, "Text")
	wholeAuditSymbol(t, got, "Logo")
}

func TestF33EmptyFileRefDoesNotBorrowNextValue(t *testing.T) {
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", "<root><data name=\"Empty\" type=\"System.Resources.ResXFileRef, System.Windows.Forms\"/><data name=\"Text\"><value>text;not-an-asset</value></data></root>")
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			t.Fatalf("borrowed next resource value: %+v", relation)
		}
	}
}

func TestF32NamespaceEscapesAndMalformedMarkup(t *testing.T) {
	source := "<r:root xmlns:r=\"urn:resources\">\r\n<r:data name=\"A&amp;B\"><r:value>ok</r:value></r:data>\r\n</r:root>"
	got := wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", source)
	symbol := wholeAuditSymbol(t, got, "A&B")
	if symbol.StartLine != 2 || source[symbol.StartByte:symbol.EndByte] != "A&amp;B" {
		t.Fatalf("escaped source span: %+v", symbol)
	}
	got = wholeAuditExtract(t, resx.NewExtractor(), "x.resx", "dotnet-resource", "<root><broken attr=\"<data name='Fake'/>\"/></root>")
	wholeAuditNoSymbol(t, got, "Fake")
}

func audit50Fixture(t *testing.T, id, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "audit50", id, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
