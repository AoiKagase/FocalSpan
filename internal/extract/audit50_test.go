package extract_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/lua"
	"github.com/focalspan/focalspan/internal/extract/nim"
	"github.com/focalspan/focalspan/internal/extract/pawn"
	"github.com/focalspan/focalspan/internal/extract/python"
	"github.com/focalspan/focalspan/internal/extract/resx"
	"github.com/focalspan/focalspan/internal/extract/ruby"
	"github.com/focalspan/focalspan/internal/extract/rust"
	"github.com/focalspan/focalspan/internal/extract/xaml"
	"github.com/focalspan/focalspan/internal/extract/zig"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestF01UnterminatedEscape(t *testing.T) {
	cases := []struct {
		name, path, lang string
		e                extract.Extractor
	}{
		{"cpp", "x.cpp", "cpp", cpp.NewExtractor()},
		{"csharp", "x.cs", "csharp", csharp.NewExtractor()},
		{"jsts", "x.ts", "typescript", jsts.NewExtractor()},
		{"lua", "x.lua", "lua", lua.NewExtractor()},
		{"pawn", "x.sma", "pawn", pawn.NewExtractor()},
		{"zig", "x.zig", "zig", zig.NewExtractor()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range [][]byte{{34, 92}, {34, 97, 92}, {34, 97, 92, 34, 34}} {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("input %q panicked: %v", input, r)
						}
					}()
					got, err := tc.e.Extract(context.Background(), model.SourceFile{Path: tc.path, Language: tc.lang, Content: input})
					if err != nil {
						t.Fatal(err)
					}
					for _, s := range got.Symbols {
						if s.StartByte < 0 || s.EndByte > len(input) {
							t.Errorf("invalid symbol range %+v", s)
						}
					}
					for _, c := range got.Chunks {
						if c.StartByte < 0 || c.EndByte > len(input) {
							t.Errorf("invalid chunk range %+v", c)
						}
					}
				}()
			}
		})
	}
}

type wholeAuditCancelGate struct {
	context.Context
	cancel context.CancelFunc
	after  int
	checks int
	fired  bool
}

func (c *wholeAuditCancelGate) Err() error {
	c.checks++
	if c.checks == c.after {
		c.fired = true
		c.cancel()
	}
	return c.Context.Err()
}

func TestF09BuilderCancellation(t *testing.T) {
	fixtures := []struct {
		path, language, source string
		e                      extract.Extractor
	}{
		{"x.cpp", "cpp", "void f() {}\n", cpp.NewExtractor()},
		{"x.cs", "csharp", "class A { void F() {} }\n", csharp.NewExtractor()},
		{"x.js", "javascript", "function f() {}\n", jsts.NewExtractor()},
		{"x.py", "python", "def f():\n pass\n", python.NewExtractor()},
		{"x.rb", "ruby", "def f\nend\n", ruby.NewExtractor()},
		{"x.rs", "rust", "fn f() {}\n", rust.NewExtractor()},
		{"x.nim", "nim", "proc f() = discard\n", nim.NewExtractor()},
		{"x.zig", "zig", "fn f() void {\n}\n", zig.NewExtractor()},
		{"x.xaml", "xaml", "<Window><Button x:Name=\"button\" /></Window>", xaml.NewExtractor()},
		{"x.resx", "dotnet-resource", "<root><data name=\"Key\" /></root>", resx.NewExtractor()},
	}
	for _, f := range fixtures {
		t.Run(f.language, func(t *testing.T) {
			for after := 1; after <= 2048; after++ {
				base, cancel := context.WithCancel(context.Background())
				gate := &wholeAuditCancelGate{Context: base, cancel: cancel, after: after}
				got, err := f.e.Extract(gate, model.SourceFile{Path: f.path, Language: f.language, Content: []byte(f.source)})
				fired := gate.fired
				cancel()
				if fired && err == nil {
					t.Fatalf("canceled at context check %d but returned success: symbols=%d chunks=%d", after, len(got.Symbols), len(got.Chunks))
				}
				if !fired {
					return
				}
			}
			t.Fatal("context-check sweep bound too small")
		})
	}
}
