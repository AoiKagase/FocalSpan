package rust

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexRustHandlesCommentsStringsLifetimesAttributesAndMacros(t *testing.T) {
	source := []byte(`// line
/* outer /* nested */ comment */
#![allow(dead_code)]
#[derive(Debug)]
macro_rules! trace { ($value:expr) => {{ println!("{}", $value); }} }
fn parse<'a>(value: &'a str) -> &'a str {
    let raw = r#"{ not a brace }"#;
    let bytes = br##"raw bytes"##;
    let c = b'c';
    value
}
`)
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
	}
	for _, kind := range []TokenKind{LineComment, BlockComment, Attribute, RawString, ByteString, Character, Lifetime, MacroPunctuation} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
	for _, token := range tokens {
		if token.StartByte < 0 || token.EndByte < token.StartByte || token.EndByte > len(source) || string(source[token.StartByte:token.EndByte]) != token.Text {
			t.Fatalf("invalid token span=%+v", token)
		}
	}
}

func TestExtractorBuildsRustHierarchyAndRelations(t *testing.T) {
	source := `mod auth;
use crate::auth::TokenService;

pub struct TokenService<T> { value: T }
pub enum Status { Ready, Expired }
pub union RawToken { value: u64 }
pub trait TokenValidator { type Error; const LIMIT: usize; fn validate(&self, value: &str) -> bool; }
impl<T> TokenService<T> {
    pub const DEFAULT: usize = 1;
    pub async fn validate_token(&self, value: &str) -> bool { self.validate(value) }
}
impl<T> TokenValidator for TokenService<T> {
    type Error = (); 
    fn validate(&self, value: &str) -> bool { value.len() > 0 }
}
type Token = String;
const MAX: usize = 10;
static NAME: &str = "token";
macro_rules! make_token { ($value:expr) => { $value }; }
extern "C" { fn external_validate(value: *const u8) -> bool; }
#[test]
fn expired_token_is_rejected() { let service = TokenService { value: "" }; service.validate_token(""); }
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth/token_service.rs", Language: "rust", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"auth", "module"}, {"TokenService", "struct"}, {"Status", "enum"}, {"RawToken", "union"},
		{"TokenValidator", "trait"}, {"TokenService", "impl"}, {"validate_token", "method"},
		{"validate", "method"}, {"DEFAULT", "associated_const"}, {"Error", "associated_type"}, {"Token", "type_alias"},
		{"MAX", "const"}, {"NAME", "static"}, {"make_token", "macro"}, {"external_validate", "extern_function"},
		{"expired_token_is_rejected", "test"},
	} {
		if !hasRustSymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := rustSymbol(got.Symbols, "src/auth/token_service.rs", "crate_module")
	service := rustSymbol(got.Symbols, "crate::TokenService", "struct")
	test := rustSymbol(got.Symbols, "crate::expired_token_is_rejected", "test")
	if owner.Handle == "" || service.Handle == "" || test.Handle == "" {
		t.Fatalf("qualified symbols missing: %+v", got.Symbols)
	}
	if !hasRustUnresolved(got.Relations, owner.Handle, "crate::auth::TokenService", "imports") {
		t.Fatalf("use relation missing: %+v", got.Relations)
	}
	if !hasRustRelation(got.Relations, test.Handle, "tests") {
		t.Fatalf("test relation missing: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte > 0 && (chunk.EndByte > len(source) || string(source[chunk.StartByte:chunk.EndByte]) != chunk.Content) {
			t.Fatalf("chunk source mismatch=%+v", chunk)
		}
	}
}

func TestExtractorRecoversMalformedRustAndHonorsCancellation(t *testing.T) {
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "broken.rs", Language: "rust", Content: []byte("pub fn validate_token(value: &str) { let raw = r#\"missing\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Symbols) == 0 || len(got.Diagnostics) == 0 {
		t.Fatalf("partial extraction was not retained: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewExtractor().Extract(ctx, model.SourceFile{Path: "cancel.rs", Language: "rust", Content: []byte("fn main() {}")}); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestExtractorRelatesImplementingTypeToTraitForReverseLookup(t *testing.T) {
	source := `trait TokenValidator {}
struct TokenService {}
impl TokenValidator for TokenService {}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/auth/token_service.rs", Language: "rust", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	service := rustSymbol(got.Symbols, "crate::TokenService", "struct")
	trait := rustSymbol(got.Symbols, "crate::TokenValidator", "trait")
	if service.Handle == "" || trait.Handle == "" {
		t.Fatalf("symbols missing: %+v", got.Symbols)
	}
	t.Logf("symbols=%+v relations=%+v", got.Symbols, got.Relations)
	for _, relation := range got.Relations {
		if relation.FromHandle == service.Handle && relation.ToHandle == trait.Handle && relation.Kind == "references" {
			return
		}
	}
	t.Fatalf("implementing type to trait relation missing: %+v", got.Relations)
}

func hasRustSymbol(symbols []model.Symbol, name, kind string) bool {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return true
		}
	}
	return false
}

func rustSymbol(symbols []model.Symbol, qualified, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.QualifiedName == qualified && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasRustRelation(relations []model.Relation, from, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}

func hasRustUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}
