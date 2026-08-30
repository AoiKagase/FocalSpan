package zig

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/testutil"
	"github.com/focalspan/focalspan/internal/model"
)

func TestLexZigHandlesCommentsStringsMultilineCharsBuiltinsAndTypeOperators(t *testing.T) {
	source := []byte("// comment\nconst text = \"hello\\\"world\";\nconst letter = 'x';\nconst path =\\\n  \\\\assets\\n\nconst value: ?u32 = null;\nconst result: anyerror!u32 = error.Invalid;\nconst imported = @import(\"std\");\ncomptime { _ = imported; }\n")
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
	for _, kind := range []TokenKind{Comment, String, MultilineString, Character, Builtin, Comptime, Operator, Identifier} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestZigExtractorBuildsDeclarationsAndRelations(t *testing.T) {
	source := []byte(`const std = @import("std");
const AuthState = enum { logged_out, logged_in };
const Credentials = struct { user: []const u8 };
const AuthError = union(enum) { invalid_token, expired };
const Handle = opaque {};

fn normalizeToken(token: []const u8) []const u8 {
    return token;
}

pub fn validateToken(token: []const u8) !bool {
    return normalizeToken(token).len > 0;
}

pub export fn pluginEntry() void {
    _ = validateToken("guest");
}

comptime {
    _ = @TypeOf(Credentials);
}

test "expired token" {
    try std.testing.expect(!try validateToken("expired"));
}
`)
	file := model.SourceFile{Path: "src/auth.zig", Language: "zig", Content: source}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"auth.zig", "module"}, {"AuthState", "enum"}, {"Credentials", "struct"},
		{"AuthError", "union"}, {"Handle", "opaque"}, {"normalizeToken", "function"},
		{"validateToken", "function"}, {"pluginEntry", "function"}, {"expired token", "test"},
	} {
		if !hasZigSymbol(got.Symbols, want.name, want.kind) {
			t.Errorf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := findZigSymbol(got.Symbols, "auth.zig", "module")
	if owner.Handle == "" || !hasZigRelation(got.Relations, owner.Handle, "std", "imports") || !hasZigRelationKind(got.Relations, "contains") {
		t.Fatalf("import/contains relations missing: %+v", got.Relations)
	}
	test := findZigSymbol(got.Symbols, "expired token", "test")
	if test.Handle == "" || !hasZigRelationKind(got.Relations, "tests") || !hasZigRelationKind(got.Relations, "calls") {
		t.Fatalf("test/call relations missing: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if string(source[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("chunk source mismatch=%+v", chunk)
		}
	}
	testutil.AssertExtraction(t, file, got)
}

func TestZigExtractorRecoversMalformedBraces(t *testing.T) {
	file := model.SourceFile{Path: "broken.zig", Language: "zig", Content: []byte("pub fn broken() void {\n  const value = 1;\n")}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if !hasZigSymbol(got.Symbols, "broken", "function") || len(got.Diagnostics) == 0 {
		t.Fatalf("malformed source was not recovered: %+v", got)
	}
	testutil.AssertExtraction(t, file, got)
}

func hasZigSymbol(symbols []model.Symbol, name, kind string) bool {
	return findZigSymbol(symbols, name, kind).Handle != ""
}

func findZigSymbol(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasZigRelation(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasZigRelationKind(relations []model.Relation, kind string) bool {
	for _, relation := range relations {
		if relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}
