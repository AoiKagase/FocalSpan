package cpp

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorSupportsCAndCPlusPlusExtensions(t *testing.T) {
	extractor := NewExtractor()
	for _, item := range []struct{ path, language string }{
		{"main.c", "c"}, {"main.h", "cpp"}, {"main.cxx", "cpp"}, {"main.ipp", "cpp"}, {"main.cppm", "cpp"},
	} {
		if !extractor.Supports(item.path, item.language) {
			t.Errorf("Supports(%q, %q)=false", item.path, item.language)
		}
	}
	if extractor.Supports("main.cs", "csharp") {
		t.Fatal("C++ extractor claimed C#")
	}
}

func TestLexerMasksCommentsStringsRawStringsAndInactiveIfZero(t *testing.T) {
	content := []byte("#if 0\nvoid fake() { }\n#if 1\nvoid nested() {}\n#endif\n#endif\n// { fake() }\nconst char* s = R\"tag(raw } text)tag\";\nvoid real() { /* { */ }\n")
	tokens, diagnostics, err := Lex(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range tokens {
		if token.Active && (token.Text == "fake" || token.Text == "nested") {
			t.Fatalf("inactive declaration token was active: %+v", token)
		}
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	seenRaw := false
	for _, token := range tokens {
		if token.Kind == RawString {
			seenRaw = true
		}
	}
	if !seenRaw {
		t.Fatalf("raw string was not tokenized: %+v", tokens)
	}
}

func TestExtractorBuildsSymbolsRelationsAndExactSourceSpans(t *testing.T) {
	content := `#include "include/auth/token_service.hpp"
#include <string>
namespace app { namespace auth {
class TokenService {
public:
    bool ValidateToken(const std::string& token) const;
    bool Helper() const { return true; }
};
bool TokenService::ValidateToken(const std::string& token) const {
    return Helper() && unknown->Check(token);
}

func TestExtractorRecoversMalformedSourceAndKeepsStableHandles(t *testing.T) {
	valid := []byte("namespace app { bool ValidateToken() { return true; } }\n")
	first, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth.cpp", Language: "cpp", Content: valid})
	if err != nil {
		t.Fatal(err)
	}
	shifted, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth.cpp", Language: "cpp", Content: append([]byte("// moved\n"), valid...)})
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
	malformed, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/broken.cpp", Language: "cpp", Content: []byte("namespace app { bool Broken() { return true;\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range malformed.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.StartByte < 0 || chunk.EndByte > len("namespace app { bool Broken() { return true;\n") || chunk.StartByte >= chunk.EndByte {
			t.Fatalf("invalid recovered chunk=%+v", chunk)
		}
	}
}
}
}
TEST(TokenServiceTest, RejectsExpiredToken) { app::auth::TokenService service; service.ValidateToken("expired"); }
`
	file := model.SourceFile{Path: "src/token_service.cpp", Language: "cpp", Content: []byte(content)}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findQualified(got.Symbols, "src/token_service.cpp", "translation_unit")
	auth := findQualified(got.Symbols, "app::auth", "namespace")
	service := findQualified(got.Symbols, "app::auth::TokenService", "class")
	validate := findQualified(got.Symbols, "app::auth::TokenService::ValidateToken", "method")
	if owner.Handle == "" || service.Handle == "" || validate.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if !hasRelation(got.Relations, auth.Handle, service.Handle, "contains") || !hasRelation(got.Relations, service.Handle, validate.Handle, "contains") {
		t.Fatalf("contains relations=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, owner.Handle, "include/auth/token_service.hpp", "imports") || !hasUnresolved(got.Relations, owner.Handle, "string", "imports") {
		t.Fatalf("include relations=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, validate.Handle, "unknown->Check", "calls") && !hasUnresolved(got.Relations, validate.Handle, "unknown::Check", "calls") {
		t.Fatalf("unresolved call relations=%+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.StartByte < 0 || chunk.EndByte > len(content) || chunk.StartByte >= chunk.EndByte {
			t.Fatalf("invalid chunk=%+v", chunk)
		}
		if string(content[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("chunk source mismatch=%+v", chunk)
		}
	}
	for _, chunk := range got.Chunks {
		if chunk.SymbolName == "TokenService" && strings.HasSuffix(chunk.Kind, "-outline") {
			if strings.Contains(chunk.Content, "ValidateToken(const") {
				t.Fatalf("class outline contains method body/header: %+v", chunk)
			}
		}
	}
}

func findQualified(symbols []model.Symbol, qualified, kind string) model.Symbol {
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
