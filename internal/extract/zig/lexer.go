package zig

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, source []byte) ([]Token, []model.Diagnostic, error) {
	tokens := make([]Token, 0, len(source)/4)
	diagnostics := make([]model.Diagnostic, 0)
	for at := 0; at < len(source); {
		if err := ctx.Err(); err != nil {
			return tokens, diagnostics, err
		}
		start := at
		if source[at] == '/' && at+1 < len(source) && source[at+1] == '/' {
			at = zigLineEnd(source, at)
			tokens = append(tokens, zigToken(Comment, source, start, at))
			continue
		}
		if source[at] == '\\' && at+1 < len(source) && source[at+1] == '\\' {
			at = zigLineEnd(source, at)
			tokens = append(tokens, zigToken(MultilineString, source, start, at))
			continue
		}
		if source[at] == '"' {
			at++
			closed := false
			for at < len(source) {
				if source[at] == '\\' {
					at += 2
					continue
				}
				if source[at] == '"' {
					at++
					closed = true
					break
				}
				at++
			}
			tokens = append(tokens, zigToken(String, source, start, at))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "zig_unclosed_string", Message: "Zig string is not closed"})
			}
			continue
		}
		if source[at] == '\'' {
			at++
			closed := false
			for at < len(source) {
				if source[at] == '\\' {
					at += 2
					continue
				}
				if source[at] == '\'' {
					at++
					closed = true
					break
				}
				at++
			}
			tokens = append(tokens, zigToken(Character, source, start, at))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "zig_unclosed_character", Message: "Zig character literal is not closed"})
			}
			continue
		}
		if source[at] == '@' && at+1 < len(source) && isZigIdentifierStart(source[at+1]) {
			at += 2
			for at < len(source) && isZigIdentifierPart(source[at]) {
				at++
			}
			tokens = append(tokens, zigToken(Builtin, source, start, at))
			continue
		}
		if isZigIdentifierStart(source[at]) {
			at++
			for at < len(source) && isZigIdentifierPart(source[at]) {
				at++
			}
			kind := Identifier
			if strings.EqualFold(string(source[start:at]), "comptime") {
				kind = Comptime
			}
			tokens = append(tokens, zigToken(kind, source, start, at))
			continue
		}
		if strings.ContainsRune("!?|", rune(source[at])) {
			tokens = append(tokens, zigToken(Operator, source, at, at+1))
			at++
			continue
		}
		if isZigSpace(source[at]) {
			at++
			continue
		}
		at++
		tokens = append(tokens, zigToken(Punctuation, source, start, at))
	}
	return tokens, diagnostics, nil
}

func zigLineEnd(source []byte, start int) int {
	for at := start; at < len(source); at++ {
		if source[at] == '\n' {
			return at
		}
	}
	return len(source)
}

func isZigSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isZigIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80
}

func isZigIdentifierPart(value byte) bool {
	return isZigIdentifierStart(value) || value >= '0' && value <= '9'
}

func zigToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: zigLineNumber(source, start), EndLine: zigLineNumber(source, end)}
}

func zigLineNumber(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}
