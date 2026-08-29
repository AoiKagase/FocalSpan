package csharp

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorSupportsCSharpOnly(t *testing.T) {
	extractor := NewExtractor()
	if !extractor.Supports("Auth/TokenService.cs", "csharp") || !extractor.Supports("TokenService.CS", "") {
		t.Fatal("C# source was not supported")
	}
	if extractor.Supports("TokenService.ts", "typescript") {
		t.Fatal("C# extractor claimed TypeScript")
	}
}

func TestLexerRecognizesCSharpStringsAttributesAndPreprocessor(t *testing.T) {
	content := []byte("#if false\nclass Fake { string x = $\"{ }\"; }\n#endif\n/// <summary>{doc}</summary>\n[Fact]\nclass Real { string a = @\"verbatim \"\" text\"; string b = $\"value {x}\"; string c = $\"\"\"raw {x}\"\"\"; }\n")
	tokens, diagnostics, err := Lex(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	seen := map[TokenKind]bool{}
	for _, token := range tokens {
		if token.Kind == Identifier && token.Text == "Fake" && token.Active {
			t.Fatal("inactive C# declaration was active")
		}
		seen[token.Kind] = true
	}
	for _, kind := range []TokenKind{VerbatimString, InterpolatedString, RawString, XMLDocComment, Attribute} {
		if !seen[kind] {
			t.Fatalf("token kind %s missing: %+v", kind, tokens)
		}
	}
}

func TestExtractorBuildsCSharpHierarchyRelationsAndExactSpans(t *testing.T) {
	content := `using AuthService = App.Auth.TokenService;
namespace App.Auth;
public interface ITokenValidator { bool ValidateToken(string token); }
public partial class TokenService(string key) : ITokenValidator {
    [Fact]
    public bool ValidateToken(string token) { return Helper(token); }
    public bool Helper(string token) => token.Length > 0;
    public string Status => $"value: {key}";
    public event EventHandler Changed;
}

`
	file := model.SourceFile{Path: "Auth/TokenService.cs", Language: "csharp", Content: []byte(content)}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findSymbol(got.Symbols, "Auth/TokenService.cs", "compilation_unit")
	ns := findSymbol(got.Symbols, "App.Auth", "namespace")
	validator := findSymbol(got.Symbols, "App.Auth.ITokenValidator", "interface")
	service := findSymbol(got.Symbols, "App.Auth.TokenService", "class")
	validate := findSymbol(got.Symbols, "App.Auth.TokenService.ValidateToken", "test")
	helper := findSymbol(got.Symbols, "App.Auth.TokenService.Helper", "method")
	if owner.Handle == "" || ns.Handle == "" || validator.Handle == "" || service.Handle == "" || validate.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if !hasRelation(got.Relations, ns.Handle, service.Handle, "contains") || !hasRelation(got.Relations, service.Handle, validate.Handle, "contains") {
		t.Fatalf("contains=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, owner.Handle, "App.Auth.TokenService", "imports") {
		t.Fatalf("using import=%+v", got.Relations)
	}
	if !hasRelation(got.Relations, service.Handle, validator.Handle, "references") {
		t.Fatalf("interface reference=%+v", got.Relations)
	}
	if !hasRelation(got.Relations, validate.Handle, helper.Handle, "tests") {
		t.Fatalf("test relation=%+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if string(file.Content[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("source mismatch=%+v", chunk)
		}
		if strings.Contains(chunk.Kind, "-outline") && strings.Contains(chunk.Content, "ValidateToken(string") {
			t.Fatalf("type outline duplicated method=%q", chunk.Content)
		}
	}
}

func TestExtractorRetainsCrossFileInterfaceReferenceAsUnresolved(t *testing.T) {
	file := model.SourceFile{Path: "Auth/TokenService.cs", Language: "csharp", Content: []byte("namespace App.Auth;\npublic partial class TokenService(string key) : ITokenValidator { }\n")}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	var class model.Symbol
	for _, symbol := range got.Symbols {
		if symbol.Kind == "class" && symbol.Name == "TokenService" {
			class = symbol
			break
		}
	}
	if class.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if !hasUnresolved(got.Relations, class.Handle, "ITokenValidator", "references") {
		t.Fatalf("relations=%+v", got.Relations)
	}
}

func TestExtractorRecoversMalformedSourceAndKeepsStableHandles(t *testing.T) {
	valid := []byte("namespace App; public class Service { public bool ValidateToken() { return true; } }\n")
	first, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Auth/Service.cs", Language: "csharp", Content: valid})
	if err != nil {
		t.Fatal(err)
	}
	shifted, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Auth/Service.cs", Language: "csharp", Content: append([]byte("// moved\n"), valid...)})
	if err != nil {
		t.Fatal(err)
	}
	find := func(symbols []model.Symbol) model.Symbol {
		for _, symbol := range symbols {
			if symbol.Name == "ValidateToken" {
				return symbol
			}
		}
		return model.Symbol{}
	}
	firstSymbol, shiftedSymbol := find(first.Symbols), find(shifted.Symbols)
	if firstSymbol.Handle == "" || shiftedSymbol.Handle == "" || firstSymbol.Handle != shiftedSymbol.Handle {
		t.Fatalf("stable symbols first=%+v shifted=%+v", firstSymbol, shiftedSymbol)
	}
	malformed, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Auth/Broken.cs", Language: "csharp", Content: []byte("namespace App { public class Broken { public bool ValidateToken() { return true; }")})
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range malformed.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.StartByte < 0 || chunk.EndByte > len("namespace App { public class Broken { public bool ValidateToken() { return true; }") || chunk.StartByte >= chunk.EndByte {
			t.Fatalf("invalid recovered chunk=%+v", chunk)
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
