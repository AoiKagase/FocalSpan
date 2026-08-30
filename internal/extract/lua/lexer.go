package lua

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
		if source[at] == '-' && at+1 < len(source) && source[at+1] == '-' {
			if end, ok := luaLongBracketStart(source, at+2); ok {
				end, closed := luaLongBracketEnd(source, end)
				tokens = append(tokens, luaToken(LongComment, source, start, end))
				if !closed {
					diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "lua_unclosed_long_comment", Message: "long comment is not closed"})
				}
				at = end
				continue
			}
			for at < len(source) && source[at] != '\n' {
				at++
			}
			tokens = append(tokens, luaToken(Comment, source, start, at))
			continue
		}
		if source[at] == '[' {
			if contentStart, ok := luaLongBracketStart(source, at); ok {
				end, closed := luaLongBracketEnd(source, contentStart)
				tokens = append(tokens, luaToken(LongString, source, start, end))
				if !closed {
					diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "lua_unclosed_long_string", Message: "long string is not closed"})
				}
				at = end
				continue
			}
		}
		if source[at] == '\'' || source[at] == '"' {
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
			tokens = append(tokens, luaToken(String, source, start, at))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "lua_unclosed_string", Message: "string is not closed"})
			}
			continue
		}
		if luaIdentifierStart(source[at]) {
			at++
			for at < len(source) && luaIdentifierPart(source[at]) {
				at++
			}
			kind := Identifier
			if luaBlockWords[string(source[start:at])] {
				kind = BlockKeyword
			}
			tokens = append(tokens, luaToken(kind, source, start, at))
			continue
		}
		at++
		tokens = append(tokens, luaToken(Punctuation, source, start, at))
	}
	return tokens, diagnostics, nil
}

var luaBlockWords = map[string]bool{
	"and": true, "do": true, "else": true, "elseif": true, "end": true,
	"for": true, "function": true, "if": true, "in": true, "local": true,
	"not": true, "or": true, "repeat": true, "then": true, "until": true,
	"while": true,
}

func luaToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: luaLineNumber(source, start), EndLine: luaLineNumber(source, end)}
}

func luaLineNumber(source []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}

func luaLongBracketStart(source []byte, start int) (int, bool) {
	if start >= len(source) || source[start] != '[' {
		return 0, false
	}
	at := start + 1
	for at < len(source) && source[at] == '=' {
		at++
	}
	if at >= len(source) || source[at] != '[' {
		return 0, false
	}
	return at + 1, true
}

func luaLongBracketEnd(source []byte, contentStart int) (int, bool) {
	open := contentStart - 1
	level := 0
	for open-level-1 >= 0 && source[open-level-1] == '=' {
		level++
	}
	for at := contentStart; at < len(source); at++ {
		if source[at] != ']' {
			continue
		}
		end := at + 1 + level
		if end < len(source) && source[end] == ']' && strings.Count(string(source[at+1:end]), "=") == level {
			return end + 1, true
		}
	}
	return len(source), false
}

func luaIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func luaIdentifierPart(value byte) bool {
	return luaIdentifierStart(value) || value >= '0' && value <= '9'
}
