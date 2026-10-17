package ruby_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/ruby"
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

func TestF29RubyHeredocIsNotDeclaration(t *testing.T) {
	got := wholeAuditExtract(t, ruby.NewExtractor(), "x.rb", "ruby", audit50Fixture(t, "F29", "source.rb"))
	wholeAuditNoSymbol(t, got, "fake")
	wholeAuditSymbol(t, got, "real")
}

func TestF29HeredocVariantsKeepRealCoordinates(t *testing.T) {
	for _, opener := range []string{"<<TEXT", "<<-TEXT", "<<~TEXT", "<<'TEXT'", "<<~\"TEXT\""} {
		source := "text = " + opener + "\nclass Fake\nend\nTEXT\ndef real\nend\n"
		got := wholeAuditExtract(t, ruby.NewExtractor(), "x.rb", "ruby", source)
		wholeAuditNoSymbol(t, got, "Fake")
		real := wholeAuditSymbol(t, got, "real")
		if real.StartLine != 5 || real.EndLine != 6 || real.StartByte != strings.Index(source, "def real") {
			t.Fatalf("real coordinates: %+v", real)
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

func TestF29BlockCommentsAndHeredocHideRelations(t *testing.T) {
	source := "=begin\nclass Fake\nend\nrequire 'fake'\n=end\ntext = <<~TEXT\ndef phantom\n helper()\nend\nrequire 'phantom'\nTEXT\ndef helper\nend\ndef real\n helper()\nend\n"
	got := wholeAuditExtract(t, ruby.NewExtractor(), "x.rb", "ruby", source)
	wholeAuditNoSymbol(t, got, "Fake")
	wholeAuditNoSymbol(t, got, "phantom")
	helper := wholeAuditSymbol(t, got, "helper")
	real := wholeAuditSymbol(t, got, "real")
	found := false
	for _, relation := range got.Relations {
		if relation.Kind == "imports" {
			t.Fatalf("noncode import: %+v", relation)
		}
		if relation.FromHandle == real.Handle && relation.ToHandle == helper.Handle && relation.Kind == "calls" {
			found = true
		}
	}
	if !found {
		t.Fatal("real call missing")
	}
}

func TestF29ReviewArgumentHeredoc(t *testing.T) {
	for _, opener := range []string{"puts(<<~TEXT)", "puts(<<-'TEXT')", "puts(<<\"TEXT\")"} {
		source := opener + "\ndef fake\nend\nTEXT\ndef real\nend\n"
		got := wholeAuditExtract(t, ruby.NewExtractor(), "x.rb", "ruby", source)
		wholeAuditNoSymbol(t, got, "fake")
		real := wholeAuditSymbol(t, got, "real")
		if real.StartLine != 5 || real.EndLine != 6 {
			t.Fatalf("following declaration moved: %+v", real)
		}
	}
}

func TestF29ReviewQuotedHeredocMarkers(t *testing.T) {
	for _, marker := range []string{"END-TEXT", "END TEXT"} {
		for _, opener := range []string{"text = <<~'" + marker + "'", "puts(<<~'" + marker + "')"} {
			source := opener + "\ndef fake\nend\n" + marker + "\ndef real\nend\n"
			got := wholeAuditExtract(t, ruby.NewExtractor(), "x.rb", "ruby", source)
			wholeAuditNoSymbol(t, got, "fake")
			real := wholeAuditSymbol(t, got, "real")
			if real.StartLine != 5 || real.EndLine != 6 {
				t.Fatalf("quoted heredoc following declaration: %+v", real)
			}
		}
	}
}
