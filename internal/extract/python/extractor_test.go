package python

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexPythonTracksIndentationStringsPrefixesCommentsAndDecorators(t *testing.T) {
	source := []byte("@decorator\nasync def load(value: str):\n\t# comment\n\ttext = f\"value={value!r}\"\n\ttriple = r'''multi\nline'''\n")
	tokens, diagnostics, err := Lex(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("tab indentation policy diagnostic missing")
	}
	seen := map[TokenKind]bool{}
	for _, token := range tokens {
		seen[token.Kind] = true
		if token.StartByte < 0 || token.EndByte < token.StartByte || token.EndByte > len(source) || string(source[token.StartByte:token.EndByte]) != token.Text {
			t.Fatalf("invalid token=%+v", token)
		}
	}
	for _, kind := range []TokenKind{Indent, Dedent, Decorator, Comment, FString, TripleString} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestExtractorBuildsPythonDeclarationsAndRelations(t *testing.T) {
	source := `from .types import TokenClaims as Claims
from typing import Protocol

class TokenProtocol(Protocol):
    def validate(self, value: str) -> bool: ...

class TokenService(TokenProtocol):
    DEFAULT = "expired"

    @property
    def ready(self) -> bool:
        return True

    @classmethod
    async def build(cls, value: Claims):
        return cls()

    def validate_token(self, token: Claims) -> bool:
        def normalize(value):
            return value.strip()
        return validate(normalize(token))

def validate(value: str) -> bool:
    return bool(value)

Token = str
@pytest.fixture
def service():
    return TokenService()

@pytest.mark.parametrize("value", ["expired"])
def test_expired_token(service, value):
    assert not service.validate_token(value)
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth/token_service.py", Language: "python", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"token_service.py", "module"}, {"TokenProtocol", "protocol"}, {"TokenService", "class"},
		{"ready", "property"}, {"build", "async_method"}, {"validate_token", "method"},
		{"normalize", "nested_function"}, {"validate", "function"}, {"DEFAULT", "class_variable"},
		{"Token", "type_alias"}, {"service", "fixture"}, {"test_expired_token", "test"},
	} {
		if !pythonSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := pythonSymbolValue(got.Symbols, "token_service.py", "module")
	test := pythonSymbolValue(got.Symbols, "test_expired_token", "test")
	if owner.Handle == "" || test.Handle == "" || !pythonUnresolved(got.Relations, owner.Handle, ".types", "imports") {
		t.Fatalf("module/test relations missing: %+v", got.Relations)
	}
	if !pythonRelationKind(got.Relations, test.Handle, "tests") {
		t.Fatalf("test call relation missing: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte > 0 && (chunk.EndByte > len(source) || string(source[chunk.StartByte:chunk.EndByte]) != chunk.Content) {
			t.Fatalf("chunk mismatch=%+v", chunk)
		}
	}
}

func TestExtractorRecoversMalformedPythonAndHonorsCancellation(t *testing.T) {
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "broken.py", Language: "python", Content: []byte("def validate(value):\n    text = '''missing\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Symbols) == 0 || len(got.Diagnostics) == 0 {
		t.Fatalf("partial extraction missing: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewExtractor().Extract(ctx, model.SourceFile{Path: "cancel.py", Language: "python", Content: []byte("def main(): pass")}); err == nil {
		t.Fatal("expected cancellation")
	}
}

func pythonSymbol(symbols []model.Symbol, name, kind string) bool {
	return pythonSymbolValue(symbols, name, kind).Handle != ""
}
func pythonSymbolValue(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}
func pythonRelationKind(relations []model.Relation, from, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}
func pythonUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}
