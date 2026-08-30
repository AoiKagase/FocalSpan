package lua

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexLuaRecognizesCommentsStringsLongBracketsAndFunctions(t *testing.T) {
	source := []byte("-- line comment\n--[=[ long comment ]=]\nvalue = [=[hello]=]\nescaped = \"quote \\\" value\"\nfunction auth.validate(token)\n  return token ~= nil\nend\n")
	tokens, diagnostics, err := Lex(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	seen := map[TokenKind]bool{}
	for _, token := range tokens {
		seen[token.Kind] = true
		if token.StartByte < 0 || token.EndByte < token.StartByte || token.EndByte > len(source) || string(source[token.StartByte:token.EndByte]) != token.Text {
			t.Fatalf("invalid token=%+v", token)
		}
	}
	for _, kind := range []TokenKind{Comment, LongComment, String, LongString, Identifier, BlockKeyword} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestLexLuaReportsMalformedLongString(t *testing.T) {
	_, diagnostics, err := Lex(context.Background(), []byte("value = [=[unterminated\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "lua_unclosed_long_string" {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
}

func TestExtractorBuildsLuaDeclarationsAndRelations(t *testing.T) {
	source := `local M = {}

local function normalize_token(token)
  return string.lower(token)
end

function M.validate_token(token)
  return normalize_token(token) ~= ""
end

function M:authorize(request)
  return self:validate_token(request.token)
end

function plugin_init()
  require("auth.token_service")
  setmetatable(M, { __index = BaseService })
end

message = "not an import"

describe("token service", function()
  it("rejects expired tokens", function()
    assert.is_false(M.validate_token("expired"))
  end)
end)
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "lib/auth/token_service.lua", Language: "lua", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"token_service.lua", "module"}, {"normalize_token", "function"}, {"validate_token", "method"},
		{"authorize", "method"}, {"plugin_init", "function"}, {"rejects expired tokens", "test"},
	} {
		if !luaSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := luaSymbolValue(got.Symbols, "token_service.lua", "module")
	if owner.Handle == "" || !luaUnresolved(got.Relations, owner.Handle, "auth.token_service", "imports") {
		t.Fatalf("imports missing: %+v", got.Relations)
	}
	if luaChunkContains(got.Chunks, "import", "not an import") {
		t.Fatalf("non-require string became import chunk: %+v", got.Chunks)
	}
	validate := luaSymbolValue(got.Symbols, "validate_token", "method")
	if validate.Handle == "" || !luaRelationKind(got.Relations, validate.Handle, "calls") {
		t.Fatalf("method call relation missing: %+v", got.Relations)
	}
	if !luaRelationKind(got.Relations, owner.Handle, "references") {
		t.Fatalf("setmetatable reference missing: %+v", got.Relations)
	}
	if !luaRelationKind(got.Relations, luaSymbolValue(got.Symbols, "rejects expired tokens", "test").Handle, "tests") {
		t.Fatalf("test relation missing: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.StartByte > chunk.EndByte || chunk.EndByte > len(source) || string(source[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("invalid chunk=%+v", chunk)
		}
	}
}

func luaSymbol(symbols []model.Symbol, name, kind string) bool {
	return luaSymbolValue(symbols, name, kind).Handle != ""
}

func luaSymbolValue(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func luaRelationKind(relations []model.Relation, from, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}

func luaUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func luaChunkContains(chunks []model.Chunk, kind, content string) bool {
	for _, chunk := range chunks {
		if chunk.Kind == kind && strings.Contains(chunk.Content, content) {
			return true
		}
	}
	return false
}
