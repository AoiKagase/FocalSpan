package nim

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, source []byte) ([]Token, []model.Diagnostic, error) {
	tokens := make([]Token, 0, len(source)/4)
	diagnostics := make([]model.Diagnostic, 0)
	depth := 0
	for at := 0; at < len(source); {
		if err := ctx.Err(); err != nil {
			return tokens, diagnostics, err
		}
		start := at
		switch {
		case source[at] == '#' && at+1 < len(source) && source[at+1] == '[':
			end, closed := nimLongCommentEnd(source, at)
			tokens = append(tokens, nimToken(LongComment, source, at, end))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "nim_unclosed_comment", Message: "Nim long comment is not closed"})
			}
			at = end
		case source[at] == '#':
			at = nimLineEnd(source, at)
			tokens = append(tokens, nimToken(Comment, source, start, at))
		case source[at] == '"' && at+2 < len(source) && source[at+1] == '"' && source[at+2] == '"':
			end, closed := nimQuotedEnd(source, at+3, "\"\"\"")
			tokens = append(tokens, nimToken(TripleString, source, at, end))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "nim_unclosed_triple_string", Message: "Nim triple string is not closed"})
			}
			at = end
		case (source[at] == 'r' || source[at] == 'R') && at+1 < len(source) && source[at+1] == '"':
			end, closed := nimQuotedEnd(source, at+2, "\"")
			tokens = append(tokens, nimToken(RawString, source, at, end))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "nim_unclosed_raw_string", Message: "Nim raw string is not closed"})
			}
			at = end
		case source[at] == '"':
			end, closed := nimQuotedEnd(source, at+1, "\"")
			tokens = append(tokens, nimToken(String, source, at, end))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "nim_unclosed_string", Message: "Nim string is not closed"})
			}
			at = end
		case source[at] == '`':
			end := at + 1
			for end < len(source) && source[end] != '`' {
				end++
			}
			if end < len(source) {
				end++
			}
			tokens = append(tokens, nimToken(BacktickIdentifier, source, at, end))
			if end == len(source) && (len(source) == 0 || source[end-1] != '`') {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "nim_unclosed_backtick", Message: "Nim backtick identifier is not closed"})
			}
			at = end
		case source[at] == '{' && at+1 < len(source) && source[at+1] == '.':
			end := strings.Index(string(source[at+2:]), ".}")
			if end < 0 {
				end = len(source)
			} else {
				end = at + 2 + end + 2
			}
			tokens = append(tokens, nimToken(Pragma, source, at, end))
			at = end
		case source[at] == '\n' && depth > 0:
			tokens = append(tokens, nimToken(Continuation, source, at, at+1))
			at++
		case isNimIdentifierStart(source[at]):
			at++
			for at < len(source) && isNimIdentifierPart(source[at]) {
				at++
			}
			tokens = append(tokens, nimToken(Identifier, source, start, at))
		default:
			switch source[at] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth > 0 {
					depth--
				}
			}
			at++
			if !isNimSpace(source[start]) {
				tokens = append(tokens, nimToken(Punctuation, source, start, at))
			}
		}
	}
	return tokens, diagnostics, nil
}

func nimQuotedEnd(source []byte, at int, delimiter string) (int, bool) {
	for at < len(source) {
		if source[at] == '\\' && delimiter == "\"" {
			at += 2
			continue
		}
		if strings.HasPrefix(string(source[at:]), delimiter) {
			return at + len(delimiter), true
		}
		at++
	}
	return len(source), false
}

func nimLongCommentEnd(source []byte, start int) (int, bool) {
	depth := 1
	for at := start + 2; at+1 < len(source); at++ {
		if source[at] == '#' && source[at+1] == '[' {
			depth++
			at++
			continue
		}
		if source[at] == ']' && source[at+1] == '#' {
			depth--
			at++
			if depth == 0 {
				return at + 1, true
			}
		}
	}
	return len(source), false
}

func nimLineEnd(source []byte, start int) int {
	for at := start; at < len(source); at++ {
		if source[at] == '\n' {
			return at
		}
	}
	return len(source)
}

func isNimSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isNimIdentifierStart(value byte) bool {
	return value == '_' || value >= 0x80 || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isNimIdentifierPart(value byte) bool {
	return isNimIdentifierStart(value) || value >= '0' && value <= '9' || value == '\''
}

func nimToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: nimLineNumber(source, start), EndLine: nimLineNumber(source, end)}
}

func nimLineNumber(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}
