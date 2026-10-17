package extract

import (
	"bytes"
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/php"
	"github.com/focalspan/focalspan/internal/model"
)

func TestRegistryBOMPreservesSymbolsAndOriginalBytePositions(t *testing.T) {
	registry := NewRegistry(goast.NewExtractor(), cpp.NewExtractor(), csharp.NewExtractor(), jsts.NewExtractor(), php.NewExtractor(), generic.NewExtractor())
	for _, tc := range []struct{ path, language, source, name, declaration string }{
		{"auth.go", "go", "package auth\nfunc ValidateToken() {}\n", "ValidateToken", "func ValidateToken"},
		{"auth.cpp", "cpp", "int ValidateToken() { return 1; }\n", "ValidateToken", "int ValidateToken"},
		{"auth.cs", "csharp", "public class TokenService {}\n", "TokenService", "class TokenService"},
		{"auth.ts", "typescript", "export function ValidateToken() { return true; }\n", "ValidateToken", "export function"},
		{"auth.php", "php", "<?php function ValidateToken() { return true; }\n", "ValidateToken", "function ValidateToken"},
		{"auth.ps1", "powershell", "function ValidateToken {\n    return $true\n}\n", "ValidateToken", "function ValidateToken"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			raw := []byte("\xef\xbb\xbf" + tc.source)
			got, err := registry.Extract(context.Background(), model.SourceFile{Path: tc.path, Language: tc.language, Content: raw})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, symbol := range got.Symbols {
				if symbol.Name == tc.name {
					found = true
					if want := bytes.Index(raw, []byte(tc.declaration)); symbol.StartByte != want {
						t.Errorf("symbol %s starts at %d, want original byte %d", tc.name, symbol.StartByte, want)
					}
				}
			}
			if !found {
				t.Fatalf("BOM lost first declaration %s: %+v", tc.name, got.Symbols)
			}
			if tc.language != "powershell" { // Generic end-range conventions are outside this fix.
				for _, chunk := range got.Chunks {
					if chunk.StartByte == 0 && chunk.EndByte == 0 {
						continue // Synthetic outlines deliberately have no source range.
					}
					if chunk.StartByte < 3 || chunk.EndByte > len(raw) || chunk.EndByte < chunk.StartByte || string(raw[chunk.StartByte:chunk.EndByte]) != chunk.Content {
						t.Errorf("chunk does not select original source bytes: %+v", chunk)
					}
				}
			}
		})
	}
}
