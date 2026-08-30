package generic

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCLikeIgnoresBracesInStringsAndComments(t *testing.T) {
	content := "class Service {\n  string text = \"}\";\n  /* { ignored */\n  int Validate() { return 1; }\n};\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "service.cs", Language: "csharp", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Chunks) < 2 || !strings.Contains(got.Chunks[0].Content, "Validate") || got.Chunks[0].EndLine != 5 {
		t.Fatalf("chunks=%+v", got.Chunks)
	}
	if !hasGenericSymbol(got.Symbols, "Service", "class") || !hasGenericSymbol(got.Symbols, "Validate", "method") {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
}

func TestCSharpExtractionSplitsMethodsPropertiesAndRecords(t *testing.T) {
	content := `namespace Demo {
public record User(string Name);
public sealed class TokenService {
    public string CurrentToken { get; private set; }
    public bool Validate(string token) {
        return token == CurrentToken;
    }
    private void Reset() { CurrentToken = ""; }
}
}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "TokenService.cs", Language: "csharp", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"TokenService", "class"}, {"CurrentToken", "property"}, {"Validate", "method"}, {"Reset", "method"},
	} {
		if !hasGenericSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %s in %+v", want.kind, want.name, got.Symbols)
		}
	}
}

func TestJavaScriptExtractionSplitsFunctionsClassesAndArrowFunctions(t *testing.T) {
	content := `export function validateToken(token) {
    return token.length > 0;
}
const normalizeToken: (token: string) => string = (token) => token.trim();
class TokenService {
    validate(options = { enabled: true }) {
        return validateToken(options.token);
    }
}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "token-service.ts", Language: "typescript", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"validateToken", "function"}, {"normalizeToken", "arrow_function"}, {"TokenService", "class"}, {"validate", "method"},
	} {
		if !hasGenericSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %s in %+v", want.kind, want.name, got.Symbols)
		}
	}
	for _, symbol := range got.Symbols {
		if symbol.Name == "validate" && symbol.EndLine != 8 {
			t.Fatalf("method range=%+v, want method body through line 8", symbol)
		}
	}
}

func TestMarkdownBoundaries(t *testing.T) {
	md := "# Auth\nintro\n\n## Expired tokens\nreject expired\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "README.md", Language: "markdown", Content: []byte(md)})
	if err != nil || len(got.Chunks) != 2 || got.Chunks[1].StartLine != 4 {
		t.Fatalf("markdown chunks=%+v err=%v", got.Chunks, err)
	}
}

func TestStructuredExtractionSupportsMultilineDeclarations(t *testing.T) {
	csharp := `public class Service
{
    public string CurrentToken
    {
        get;
    }
    public bool Validate(
        string token)
    {
        return token == CurrentToken;
    }
}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(csharp)})
	if err != nil {
		t.Fatal(err)
	}
	if !hasGenericSymbol(got.Symbols, "CurrentToken", "property") || !hasGenericSymbol(got.Symbols, "Validate", "method") {
		t.Fatalf("C# symbols=%+v", got.Symbols)
	}
	for _, symbol := range got.Symbols {
		if symbol.Name == "Validate" && symbol.EndLine != 11 {
			t.Fatalf("C# method range=%+v, want body through line 11", symbol)
		}
	}

	javascript := `const validate = (
    token
) => {
    return token.length > 0;
}`
	got, err = NewExtractor().Extract(context.Background(), model.SourceFile{Path: "validate.js", Language: "javascript", Content: []byte(javascript)})
	if err != nil {
		t.Fatal(err)
	}
	if !hasGenericSymbol(got.Symbols, "validate", "arrow_function") {
		t.Fatalf("JavaScript symbols=%+v", got.Symbols)
	}
}

func TestJavaScriptExtractionFindsObjectArrowProperties(t *testing.T) {
	content := `const handlers = {
    validateToken: (token) => {
        return token.length > 0;
    },
};`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "handlers.js", Language: "javascript", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	if !hasGenericSymbol(got.Symbols, "validateToken", "arrow_function") {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	for _, symbol := range got.Symbols {
		if symbol.Name == "validateToken" && (symbol.StartLine != 2 || symbol.EndLine != 4) {
			t.Fatalf("arrow property range=%+v", symbol)
		}
	}
}

func TestFallbackWindowsOverlapAndLineSafety(t *testing.T) {
	lines := make([]string, 0, 175)
	for i := 0; i < 175; i++ {
		lines = append(lines, "line")
	}
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "notes.txt", Language: "text", Content: []byte(strings.Join(lines, "\n"))})
	if err != nil || len(got.Chunks) < 2 {
		t.Fatalf("chunks=%+v err=%v", got.Chunks, err)
	}
	for _, chunk := range got.Chunks {
		if chunk.EndLine-chunk.StartLine+1 > 160 || !strings.HasSuffix(chunk.Content, "line") {
			t.Fatalf("unsafe chunk=%+v", chunk)
		}
	}
}

func TestFallbackWindowsAssignDistinctSymbolHandlesToRepeatedSignatures(t *testing.T) {
	lines := make([]string, 151)
	for i := range lines {
		lines[i] = "entry-" + string(rune('a'+i%26)) + "-" + strings.Repeat("x", i%7)
	}
	lines[0] = "{"
	lines[70] = "{"
	content := strings.Join(lines, "\n")

	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{
		Path: "internal/discovery/testdata/rust/collections.json", Language: "config", Content: []byte(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(got.Symbols))
	for _, symbol := range got.Symbols {
		if seen[symbol.Handle] {
			t.Fatalf("duplicate symbol handle %q in %+v", symbol.Handle, got.Symbols)
		}
		seen[symbol.Handle] = true
	}
}

func hasGenericSymbol(symbols []model.Symbol, name, kind string) bool {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return true
		}
	}
	return false
}
