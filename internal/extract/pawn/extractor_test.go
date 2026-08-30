package pawn

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexPawnRecognizesDirectivesCommentsStringsTagsAndDeclarations(t *testing.T) {
	source := []byte("#include <amxmodx>\n// comment\n/* block */\nnew bool:g_enabled[4];\nnew title[] = \"auth\";\nstock bool:validate_token(const value[], size = sizeof value) {\n  return value[0] != EOS;\n}\n")
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
	for _, kind := range []TokenKind{Directive, Comment, BlockComment, String, Identifier, Keyword, Number} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestLexPawnReportsMalformedDirective(t *testing.T) {
	_, diagnostics, err := Lex(context.Background(), []byte("#include <amxmodx\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "pawn_malformed_directive" {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
}

func TestExtractorBuildsPawnDeclarationsAndHandlerRelations(t *testing.T) {
	source := `#include <amxmodx>
#include "auth.inc"

enum AuthState {
  AUTH_DENIED,
  AUTH_ALLOWED
}

new g_token[64];
const MAX_ATTEMPTS = 3;
native get_user_authid(id, buffer[], length);
forward bool:validate_token(const value[]);

stock bool:validate_token(const value[]) {
  return value[0] != EOS;
}

public plugin_init() {
  register_plugin("Auth", "1.0", "FocalSpan");
  register_clcmd("say /login", "cmd_login");
}

public cmd_login(id) {
  if (validate_token(g_token)) {
    set_task(1.0, "finish_login", id);
  }
}

public finish_login(id) {
  client_print(id, print_chat, "ok");
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "addons/amxmodx/scripting/auth_plugin.sma", Language: "pawn", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"auth_plugin.sma", "pawn_unit"}, {"AuthState", "enum"}, {"g_token", "global"},
		{"MAX_ATTEMPTS", "constant"}, {"get_user_authid", "native"}, {"validate_token", "stock"},
		{"plugin_init", "callback"}, {"cmd_login", "callback"}, {"finish_login", "callback"},
	} {
		if !pawnSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := pawnSymbolValue(got.Symbols, "auth_plugin.sma", "pawn_unit")
	if owner.Handle == "" || !pawnUnresolved(got.Relations, owner.Handle, "auth.inc", "imports") {
		t.Fatalf("include relation missing: %+v", got.Relations)
	}
	login := pawnSymbolValue(got.Symbols, "plugin_init", "callback")
	if login.Handle == "" || !pawnResolvedName(got.Relations, got.Symbols, login.Handle, "cmd_login", "references") {
		t.Fatalf("handler reference missing: %+v", got.Relations)
	}
	cmd := pawnSymbolValue(got.Symbols, "cmd_login", "callback")
	if cmd.Handle == "" || !pawnResolvedName(got.Relations, got.Symbols, cmd.Handle, "validate_token", "calls") {
		t.Fatalf("call relation missing: %+v", got.Relations)
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

func pawnSymbol(symbols []model.Symbol, name, kind string) bool {
	return pawnSymbolValue(symbols, name, kind).Handle != ""
}

func pawnSymbolValue(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func pawnResolvedName(relations []model.Relation, symbols []model.Symbol, from, name, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle != from || relation.Kind != kind || relation.ToHandle == "" {
			continue
		}
		for _, symbol := range symbols {
			if symbol.Handle == relation.ToHandle && strings.EqualFold(symbol.Name, name) {
				return true
			}
		}
	}
	return false
}

func pawnUnresolved(relations []model.Relation, from, name, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.Kind == kind && strings.EqualFold(relation.UnresolvedTo, name) {
			return true
		}
	}
	return false
}
