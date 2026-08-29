package jsts

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorSupportsJavaScriptTypeScriptJSXAndCommonJS(t *testing.T) {
	extractor := NewExtractor()
	for _, item := range []struct{ path, language string }{{"main.js", "javascript"}, {"main.jsx", "javascript"}, {"main.ts", "typescript"}, {"main.tsx", "typescript"}, {"main.mjs", ""}, {"main.cjs", ""}} {
		if !extractor.Supports(item.path, item.language) {
			t.Errorf("Supports(%q,%q)=false", item.path, item.language)
		}
	}
	if extractor.Supports("main.go", "go") {
		t.Fatal("JSTS extractor claimed Go")
	}
}

func TestLexerKeepsTemplateRegexAndJSXOutOfStructuralBraces(t *testing.T) {
	content := []byte("const text = `value ${call({ brace: true })}`;\nconst re = /\\{token\\}/gi;\nconst node = <Widget value={text}>child</Widget>;\nfunction real() { return text; }\n")
	tokens, diagnostics, err := Lex(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	seen := map[TokenKind]bool{}
	for _, token := range tokens {
		seen[token.Kind] = true
	}
	for _, kind := range []TokenKind{Template, RegexLiteral, JSX} {
		if !seen[kind] {
			t.Fatalf("kind %s missing: %+v", kind, tokens)
		}
	}
}

func TestExtractorBuildsJSTSModulesCallsTestsAndExactSpans(t *testing.T) {
	content := `import { validateToken as vt } from "./token-validator";
export { vt as validateToken };
export class TokenService {
    validateToken(token: string) { return vt(token); }
}

func TestExtractorRecoversMalformedSourceAndKeepsStableHandles(t *testing.T) {
	valid := []byte("export function validateToken(token: string) { return token.length > 0; }\n")
	first, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth.ts", Language: "typescript", Content: valid})
	if err != nil {
		t.Fatal(err)
	}
	shifted, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth.ts", Language: "typescript", Content: append([]byte("// moved\n"), valid...)})
	if err != nil {
		t.Fatal(err)
	}
	find := func(symbols []model.Symbol) model.Symbol {
		for _, symbol := range symbols {
			if symbol.Name == "validateToken" {
				return symbol
			}
		}
		return model.Symbol{}
	}
	firstSymbol, shiftedSymbol := find(first.Symbols), find(shifted.Symbols)
	if firstSymbol.Handle == "" || shiftedSymbol.Handle == "" || firstSymbol.Handle != shiftedSymbol.Handle {
		t.Fatalf("stable symbols first=%+v shifted=%+v", firstSymbol, shiftedSymbol)
	}
	malformed, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/broken.ts", Language: "typescript", Content: []byte("export function validateToken(token: string) { return token.length > 0;\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range malformed.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.StartByte < 0 || chunk.EndByte > len("export function validateToken(token: string) { return token.length > 0;\n") || chunk.StartByte >= chunk.EndByte {
			t.Fatalf("invalid recovered chunk=%+v", chunk)
		}
	}
}
export const normalize = (token: string) => token.trim();
test("expired token is rejected", () => { vt("expired"); });
`
	file := model.SourceFile{Path: "src/auth/token-service.ts", Language: "typescript", Content: []byte(content)}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findSymbol(got.Symbols, "src/auth/token-service.ts", "module")
	service := findSymbol(got.Symbols, "TokenService", "class")
	method := findSymbol(got.Symbols, "TokenService.validateToken", "method")
	testSymbol := findSymbol(got.Symbols, "expired token is rejected", "test")
	if owner.Handle == "" || service.Handle == "" || method.Handle == "" || testSymbol.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if !hasRelation(got.Relations, service.Handle, method.Handle, "contains") {
		t.Fatalf("contains=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, owner.Handle, "src/auth/token-validator", "imports") && !hasUnresolved(got.Relations, owner.Handle, "src/auth/token-validator.ts", "imports") {
		t.Fatalf("module import=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, owner.Handle, "validateToken", "imports") && !hasUnresolved(got.Relations, owner.Handle, "vt", "imports") {
		t.Fatalf("symbol import=%+v", got.Relations)
	}
	if !hasRelation(got.Relations, testSymbol.Handle, method.Handle, "tests") && !hasUnresolved(got.Relations, testSymbol.Handle, "vt", "tests") {
		t.Fatalf("test relation=%+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if string(file.Content[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("source mismatch=%+v", chunk)
		}
		if strings.HasSuffix(chunk.Kind, "-outline") && strings.Contains(chunk.Content, "return vt") {
			t.Fatalf("outline duplicated body=%q", chunk.Content)
		}
	}
}

func findSymbol(symbols []model.Symbol, qualified, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.QualifiedName == qualified && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}
func hasRelation(relations []model.Relation, from, to, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.ToHandle == to && relation.Kind == kind {
			return true
		}
	}
	return false
}
func hasUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}
