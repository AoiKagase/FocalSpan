package extract_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/lua"
	"github.com/focalspan/focalspan/internal/extract/pawn"
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
