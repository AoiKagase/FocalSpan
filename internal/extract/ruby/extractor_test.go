package ruby

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexRubyRecognizesInterpolationRegexPercentHeredocAndBlocks(t *testing.T) {
	source := []byte("# comment\n=begin\nblock comment\n=end\nvalue = \"#{name}\"\nregex = /token+/i\nsymbol = :validate\nwords = %w[one two]\ntext = <<~TEXT\nhello\nTEXT\nitems.each do |item|\n  puts item if item\nend\n")
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
	for _, kind := range []TokenKind{Comment, String, Interpolation, Regex, Symbol, PercentLiteral, Heredoc, BlockKeyword} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
}

func TestExtractorBuildsRubyDeclarationsAndRelations(t *testing.T) {
	source := `require_relative "../auth/token_service"
require "json"

module Auth
  class TokenService < BaseService
    include TokenValidator
    attr_accessor :token
    TOKEN_KIND = :bearer

    class << self
    end

    def validate_token(value)
      normalize(value)
    end

    def self.build(value)
      new(value)
    end

    define_method(:normalize) { |value| value.to_s }
    alias validate validate_token
  end
end

class TokenServiceTest < Minitest::Test
  def test_expired_token_is_rejected
    TokenService.new.validate_token("expired")
  end
end

RSpec.describe TokenService do
  it "accepts a live token" do
    subject.validate_token("live")
  end
end
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "lib/auth/token_service.rb", Language: "ruby", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"Auth", "module"}, {"TokenService", "class"}, {"<< self", "singleton_class"},
		{"validate_token", "method"}, {"build", "singleton_method"}, {"token", "accessor"},
		{"TOKEN_KIND", "constant"}, {"normalize", "define_method"}, {"validate", "alias"},
		{"test_expired_token_is_rejected", "test"}, {"accepts a live token", "test"},
	} {
		if !rubySymbol(got.Symbols, want.name, want.kind) {
			t.Fatalf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := rubySymbolValue(got.Symbols, "token_service.rb", "module")
	if owner.Handle == "" || !rubyUnresolved(got.Relations, owner.Handle, "../auth/token_service", "imports") {
		t.Fatalf("imports missing: %+v", got.Relations)
	}
	if !rubyChunkContains(got.Chunks, "import", "require_relative \"../auth/token_service\"") {
		t.Fatalf("import source chunk missing: %+v", got.Chunks)
	}
	for _, chunk := range got.Chunks {
		if chunk.Kind == "import" && strings.Contains(chunk.Content, "require_relative \"../auth/token_service\"") {
			if chunk.StartLine != 1 || chunk.EndLine != 1 {
				t.Fatalf("import source chunk has byte offsets as line range: %+v", chunk)
			}
		}
	}
	service := rubySymbolValue(got.Symbols, "TokenService", "class")
	if service.Handle == "" || !rubyUnresolved(got.Relations, service.Handle, "TokenValidator", "references") {
		t.Fatalf("mixin relation missing: %+v", got.Relations)
	}
	if !rubyUnresolved(got.Relations, service.Handle, "BaseService", "references") {
		t.Fatalf("inheritance relation missing: %+v", got.Relations)
	}
	for _, symbol := range got.Symbols {
		if symbol.Name == "test_expired_token_is_rejected" && symbol.Kind == "test" && !rubyRelationKind(got.Relations, symbol.Handle, "tests") {
			t.Fatalf("test relation missing: %+v", got.Relations)
		}
	}
}

func rubySymbol(symbols []model.Symbol, name, kind string) bool {
	return rubySymbolValue(symbols, name, kind).Handle != ""
}
func rubySymbolValue(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}
func rubyRelationKind(relations []model.Relation, from, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}
func rubyUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func rubyChunkContains(chunks []model.Chunk, kind, content string) bool {
	for _, chunk := range chunks {
		if chunk.Kind == kind && strings.Contains(chunk.Content, content) {
			return true
		}
	}
	return false
}
