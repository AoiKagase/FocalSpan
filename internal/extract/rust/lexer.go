package rust

import (
	"context"
	"strings"
	"unicode"

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
		switch {
		case isSpace(source[at]):
			at++
			for at < len(source) && isSpace(source[at]) {
				at++
			}
			tokens = append(tokens, makeToken(Whitespace, source, start, at))
		case source[at] == '/' && at+1 < len(source) && source[at+1] == '/':
			at += 2
			for at < len(source) && source[at] != '\n' {
				at++
			}
			tokens = append(tokens, makeToken(LineComment, source, start, at))
		case source[at] == '/' && at+1 < len(source) && source[at+1] == '*':
			at += 2
			depth := 1
			for at < len(source) && depth > 0 {
				if at+1 < len(source) && source[at] == '/' && source[at+1] == '*' {
					depth++
					at += 2
					continue
				}
				if at+1 < len(source) && source[at] == '*' && source[at+1] == '/' {
					depth--
					at += 2
					continue
				}
				at++
			}
			if depth != 0 {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "rust_unclosed_comment", Message: "block comment is not closed"})
			}
			tokens = append(tokens, makeToken(BlockComment, source, start, at))
		case source[at] == '#' && (at+1 < len(source) && source[at+1] == '[' || at+2 < len(source) && source[at+1] == '!' && source[at+2] == '['):
			at = attributeEnd(source, at)
			tokens = append(tokens, makeToken(Attribute, source, start, at))
		case rawPrefixLength(source, at) > 0:
			isByte := source[at] == 'b'
			prefix := rawPrefixLength(source, at)
			end, ok := rawStringEnd(source, at, prefix)
			at = end
			if !ok {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "rust_unclosed_raw_string", Message: "raw string is not closed"})
			}
			kind := RawString
			if isByte {
				kind = ByteString
			}
			tokens = append(tokens, makeToken(kind, source, start, at))
		case source[at] == 'b' && at+1 < len(source) && source[at+1] == '"':
			at = quotedEnd(source, at+1, '"')
			if at == len(source) && source[at-1] != '"' {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "rust_unclosed_string", Message: "byte string is not closed"})
			}
			tokens = append(tokens, makeToken(ByteString, source, start, at))
		case source[at] == 'b' && at+1 < len(source) && source[at+1] == '\'':
			at = quotedEnd(source, at+1, '\'')
			tokens = append(tokens, makeToken(Character, source, start, at))
		case source[at] == '"':
			at = quotedEnd(source, at, '"')
			if at == len(source) && source[at-1] != '"' {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "rust_unclosed_string", Message: "string is not closed"})
			}
			tokens = append(tokens, makeToken(String, source, start, at))
		case source[at] == '\'':
			if end, ok := lifetimeOrChar(source, at); ok {
				at = end
				kind := Lifetime
				if source[start] == '\'' && end > start+1 && source[end-1] == '\'' {
					kind = Character
				}
				tokens = append(tokens, makeToken(kind, source, start, at))
			} else {
				at++
				tokens = append(tokens, makeToken(Punctuation, source, start, at))
			}
		case isIdentStart(source[at]):
			at++
			for at < len(source) && isIdentPart(source[at]) {
				at++
			}
			kind := Identifier
			if rustKeywords[string(source[start:at])] {
				kind = Keyword
			}
			tokens = append(tokens, makeToken(kind, source, start, at))
		case source[at] >= '0' && source[at] <= '9':
			at++
			for at < len(source) && (isIdentPart(source[at]) || source[at] == '.') {
				at++
			}
			tokens = append(tokens, makeToken(Number, source, start, at))
		default:
			operator := operatorAt(source, at)
			if operator != "" {
				at += len(operator)
				kind := Operator
				if operator == "!" {
					kind = MacroPunctuation
				}
				tokens = append(tokens, makeToken(kind, source, start, at))
			} else {
				at++
				tokens = append(tokens, makeToken(Punctuation, source, start, at))
			}
		}
	}
	return tokens, diagnostics, nil
}

func makeToken(kind TokenKind, source []byte, start, end int) Token {
	startLine, endLine := 1, 1
	for _, value := range source[:start] {
		if value == '\n' {
			startLine++
		}
	}
	endLine = startLine
	for _, value := range source[start:end] {
		if value == '\n' {
			endLine++
		}
	}
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine}
}

func attributeEnd(source []byte, start int) int {
	open := start + 1
	if source[open] == '!' {
		open++
	}
	depth := 0
	quote := byte(0)
	for at := open; at < len(source); at++ {
		if quote != 0 {
			if source[at] == '\\' {
				at++
			} else if source[at] == quote {
				quote = 0
			}
			continue
		}
		if source[at] == '\'' || source[at] == '"' {
			quote = source[at]
			continue
		}
		if source[at] == '[' {
			depth++
		} else if source[at] == ']' {
			depth--
			if depth == 0 {
				return at + 1
			}
		}
	}
	return len(source)
}

func rawPrefixLength(source []byte, at int) int {
	if at >= len(source) || source[at] != 'r' && !(source[at] == 'b' && at+1 < len(source) && source[at+1] == 'r') {
		return 0
	}
	position := at + 1
	if source[at] == 'b' {
		position++
	}
	hashes := 0
	for position < len(source) && source[position] == '#' {
		hashes++
		position++
	}
	if position < len(source) && source[position] == '"' {
		return position - at + 1
	}
	return 0
}

func rawStringEnd(source []byte, start, prefix int) (int, bool) {
	hashes := 0
	for position := start; position < start+prefix && position < len(source); position++ {
		if source[position] == '#' {
			hashes++
		}
	}
	closing := "\"" + strings.Repeat("#", hashes)
	if at := strings.Index(string(source[start+prefix:]), closing); at >= 0 {
		return start + prefix + at + len(closing), true
	}
	return len(source), false
}

func quotedEnd(source []byte, start int, quote byte) int {
	for at := start + 1; at < len(source); at++ {
		if source[at] == '\\' {
			at++
			continue
		}
		if source[at] == quote {
			return at + 1
		}
	}
	return len(source)
}

func lifetimeOrChar(source []byte, start int) (int, bool) {
	if start+1 >= len(source) || !isIdentStart(source[start+1]) {
		return 0, false
	}
	position := start + 2
	for position < len(source) && isIdentPart(source[position]) {
		position++
	}
	if position < len(source) && source[position] == '\'' {
		return position + 1, true
	}
	return position, true
}

func operatorAt(source []byte, at int) string {
	for _, value := range []string{"::", "->", "=>", "..=", "..", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "!"} {
		if at+len(value) <= len(source) && string(source[at:at+len(value)]) == value {
			return value
		}
	}
	return ""
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f'
}
func isIdentStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80 && unicode.IsLetter(rune(value))
}
func isIdentPart(value byte) bool { return isIdentStart(value) || value >= '0' && value <= '9' }
