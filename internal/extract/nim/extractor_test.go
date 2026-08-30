package nim

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/testutil"
	"github.com/focalspan/focalspan/internal/model"
)

func TestLexNimHandlesIndentationCommentsStringsPragmasBackticksAndContinuation(t *testing.T) {
	source := []byte("#[ outer comment\n  #[ nested ]#\n]#\nlet raw = r\"C:\\\\tmp\"\nlet triple = \"\"\"multi\nline\"\"\"\nproc `odd name`(x: int) {.inline.} =\n  let value = (x +\n    1)\n")
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
	for _, kind := range []TokenKind{LongComment, RawString, TripleString, Pragma, BacktickIdentifier, Continuation, Identifier} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestNimExtractorBuildsDeclarationsAndRelations(t *testing.T) {
	source := []byte(`import std/strutils
from auth_types import Token
include helpers

type
  AuthState* = enum
    loggedOut, loggedIn
  Credentials* = object
    userName*: string
  TokenId* = distinct string

const defaultRole = "user"
let cachedRole = defaultRole
var activeUser: string

proc normalizeToken(token: string): string =
  result = token.strip()

func validateToken(token: Token): bool =
  result = normalizeToken($token) != ""

method authorize(request: Token): bool =
  validateToken(request)

iterator authStates(): AuthState =
  yield loggedOut

template withAuth(body: untyped) =
  body

macro makeAuth(body: untyped): untyped =
  body

suite "authentication":
  test "expired token":
    check validateToken(Token())
`)
	file := model.SourceFile{Path: "src/auth.nim", Language: "nim", Content: source}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"auth.nim", "module"}, {"AuthState", "enum"}, {"Credentials", "object"},
		{"TokenId", "distinct"}, {"defaultRole", "const"}, {"cachedRole", "let"},
		{"activeUser", "var"}, {"normalizeToken", "proc"}, {"validateToken", "func"},
		{"authorize", "method"}, {"authStates", "iterator"}, {"withAuth", "template"},
		{"makeAuth", "macro"}, {"expired token", "test"},
	} {
		if !hasNimSymbol(got.Symbols, want.name, want.kind) {
			t.Errorf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := findNimSymbol(got.Symbols, "auth.nim", "module")
	if owner.Handle == "" || !hasNimRelation(got.Relations, owner.Handle, "std/strutils", "imports") || !hasNimRelationKind(got.Relations, "contains") {
		t.Fatalf("module/import/contains relations missing: %+v", got.Relations)
	}
	validate := findNimSymbol(got.Symbols, "validateToken", "func")
	if validate.Handle == "" || !hasNimRelationKind(got.Relations, "calls") || !hasNimRelationKind(got.Relations, "references") {
		t.Fatalf("call/type relations missing: %+v", got.Relations)
	}
	test := findNimSymbol(got.Symbols, "expired token", "test")
	if test.Handle == "" || !hasNimRelationKind(got.Relations, "tests") {
		t.Fatalf("test relation missing: %+v", got.Relations)
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

func TestNimExtractorRecoversMalformedIndentationWithoutPanic(t *testing.T) {
	file := model.SourceFile{Path: "broken.nim", Language: "nim", Content: []byte("proc broken(x: int) =\n  if x > 0:\n    echo x\n  #[ unterminated\n")}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNimSymbol(got.Symbols, "broken", "proc") || len(got.Diagnostics) == 0 {
		t.Fatalf("malformed source was not recovered: %+v", got)
	}
	testutil.AssertExtraction(t, file, got)
}

func hasNimSymbol(symbols []model.Symbol, name, kind string) bool {
	return findNimSymbol(symbols, name, kind).Handle != ""
}

func findNimSymbol(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasNimRelation(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasNimRelationKind(relations []model.Relation, kind string) bool {
	for _, relation := range relations {
		if relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}
