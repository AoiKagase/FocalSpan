package pawn

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, source []byte) ([]Token, []model.Diagnostic, error) {
	tokens := make([]Token, 0, len(source)/3)
	diagnostics := make([]model.Diagnostic, 0)
	for at := 0; at < len(source); {
		if err := ctx.Err(); err != nil {
			return tokens, diagnostics, err
		}
		start := at
		if source[at] == '#' && (at == 0 || source[at-1] == '\n') {
			for at < len(source) && source[at] != '\n' {
				at++
			}
			tokens = append(tokens, pawnToken(Directive, source, start, at))
			text := strings.TrimSpace(string(source[start:at]))
			if strings.HasPrefix(strings.ToLower(text), "#include") && !strings.Contains(text, ">") && !strings.Contains(text, "\"") {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "pawn_malformed_directive", Message: "include directive is not closed"})
			}
			continue
		}
		if source[at] == '/' && at+1 < len(source) && source[at+1] == '/' {
			at += 2
			for at < len(source) && source[at] != '\n' {
				at++
			}
			tokens = append(tokens, pawnToken(Comment, source, start, at))
			continue
		}
		if source[at] == '/' && at+1 < len(source) && source[at+1] == '*' {
			at += 2
			closed := false
			for at+1 < len(source) {
				if source[at] == '*' && source[at+1] == '/' {
					at += 2
					closed = true
					break
				}
				at++
			}
			if !closed {
				at = len(source)
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "pawn_unclosed_block_comment", Message: "block comment is not closed"})
			}
			tokens = append(tokens, pawnToken(BlockComment, source, start, at))
			continue
		}
		if source[at] == '"' || source[at] == '\'' {
			quote := source[at]
			at++
			closed := false
			for at < len(source) {
				if source[at] == '\\' {
					at += 2
					continue
				}
				if source[at] == quote {
					at++
					closed = true
					break
				}
				at++
			}
			kind := String
			if quote == '\'' {
				kind = Char
			}
			tokens = append(tokens, pawnToken(kind, source, start, at))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "pawn_unclosed_literal", Message: "literal is not closed"})
			}
			continue
		}
		if source[at] >= '0' && source[at] <= '9' {
			at++
			for at < len(source) && (source[at] >= '0' && source[at] <= '9' || source[at] == '.') {
				at++
			}
			tokens = append(tokens, pawnToken(Number, source, start, at))
			continue
		}
		if pawnIdentifierStart(source[at]) {
			at++
			for at < len(source) && pawnIdentifierPart(source[at]) {
				at++
			}
			kind := Identifier
			if pawnKeywords[strings.ToLower(string(source[start:at]))] {
				kind = Keyword
			}
			tokens = append(tokens, pawnToken(kind, source, start, at))
			continue
		}
		at++
		tokens = append(tokens, pawnToken(Punctuation, source, start, at))
	}
	return tokens, diagnostics, nil
}

var pawnKeywords = map[string]bool{
	"assert": true, "bool": true, "case": true, "const": true, "default": true,
	"do": true, "else": true, "enum": true, "false": true, "for": true,
	"forward": true, "if": true, "new": true, "native": true, "public": true,
	"return": true, "sizeof": true, "static": true, "stock": true, "switch": true,
	"true": true, "while": true,
}

func pawnToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: pawnLineNumber(source, start), EndLine: pawnLineNumber(source, end)}
}

func pawnLineNumber(source []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}

func pawnIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func pawnIdentifierPart(value byte) bool {
	return pawnIdentifierStart(value) || value >= '0' && value <= '9'
}
