package ruby

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"strings"
	"testing"
)

func TestR06RubyStringRegions(t *testing.T) {
	for _, literal := range []string{"\"\ndef phantom\nend\n\"", "'\ndef phantom\nend\n'", "%q{\ndef phantom\nend\n}", "%Q{\ndef phantom\nend\n}", "<<~BODY\ndef phantom\nend\nBODY"} {
		source := "text = " + literal + "\ndef target\nend\ndef real\n  text = \"target()\"\nend\n"
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.rb", Language: "ruby", Content: []byte(source)})
		if e != nil {
			t.Fatal(e)
		}
		names := map[string]string{}
		for _, s := range x.Symbols {
			names[s.Handle] = s.Name
			if s.Name == "phantom" {
				t.Errorf("literal declaration leaked: %+v", s)
			}
		}
		for _, r := range x.Relations {
			if names[r.FromHandle] == "real" && names[r.ToHandle] == "target" && r.Kind == "calls" {
				t.Errorf("literal call leaked: %+v", r)
			}
		}
		for _, c := range x.Chunks {
			if c.EndByte > c.StartByte && c.Content != source[c.StartByte:c.EndByte] {
				t.Errorf("source bytes changed: %+v", c)
			}
		}
	}
}

func TestR06RubyLiteralConsumers(t *testing.T) {
	source, e := os.ReadFile("../../../testdata/regressions/residual11/ruby-strings.rb")
	if e != nil {
		t.Fatal(e)
	}
	source = []byte(strings.ReplaceAll(string(source), "\n", "\r\n"))
	x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.rb", Language: "ruby", Content: source})
	if e != nil {
		t.Fatal(e)
	}
	foundTest, foundReal, foundImport := false, false, false
	for _, s := range x.Symbols {
		if s.Name == "works" && s.Kind == "test" {
			foundTest = true
		}
		if s.Name == "real" {
			foundReal = true
		}
	}
	for _, r := range x.Relations {
		if r.Kind == "imports" && r.UnresolvedTo == "dep" {
			foundImport = true
		}
	}
	if !foundTest || !foundReal || !foundImport {
		t.Errorf("test=%v real=%v import=%v: %+v", foundTest, foundReal, foundImport, x)
	}
	for _, c := range x.Chunks {
		if c.EndByte > c.StartByte && c.Content != string(source[c.StartByte:c.EndByte]) {
			t.Errorf("CRLF source changed: %+v", c)
		}
	}
}

func TestR07RubyEndlessMethods(t *testing.T) {
	for _, tc := range []struct {
		first string
		end   int
	}{
		{"def first = 1\n", 1}, {"def first(x = 1) = x\n", 1}, {"def self.first(x) = x\n", 1},
		{"def first = [\n  1,\n  2\n]\n", 4}, {"def first = begin\n  value()\nend\n", 3},
		{"def first = \"one\ntwo\"\n", 2}, {"def first =\n  1\n", 2},
	} {
		source := tc.first + "def second\n  2\nend\n"
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.rb", Language: "ruby", Content: []byte(source)})
		if e != nil {
			t.Fatal(e)
		}
		first, second := false, false
		for _, s := range x.Symbols {
			if s.Name == "first" {
				first = true
				if s.EndLine != tc.end {
					t.Errorf("%q first ends at %d want %d", tc.first, s.EndLine, tc.end)
				}
			}
			if s.Name == "second" {
				second = true
				if s.ParentHandle != "" || s.QualifiedName != "second" {
					t.Errorf("second is not independent: %+v", s)
				}
			}
		}
		if !first || !second {
			t.Errorf("declarations missing: %+v", x.Symbols)
		}
	}
	for _, first := range []string{"def first(x = 1)\n  x\nend\n", "def first x = 1\n  x\nend\n", "def value=(x)\n  @value = x\nend\n"} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.rb", Language: "ruby", Content: []byte(first + "def second\nend\n")})
		if e != nil {
			t.Fatal(e)
		}
		for _, s := range x.Symbols {
			if s.StartLine == 1 && s.Kind != "module" && s.EndLine != 3 {
				t.Errorf("normal method misread as endless: %+v", s)
			}
		}
	}
}
