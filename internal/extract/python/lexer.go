package python

import (
	"context"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, source []byte) ([]Token, []model.Diagnostic, error) {
	starts := lineStarts(source)
	tokens := make([]Token, 0, len(source)/3)
	diagnostics := make([]model.Diagnostic, 0)
	indentStack := []int{0}
	skipUntil := -1
	for line := 0; line < len(starts); line++ {
		if err := ctx.Err(); err != nil {
			return tokens, diagnostics, err
		}
		lineStart := starts[line]
		lineEnd := len(source)
		if line+1 < len(starts) {
			lineEnd = starts[line+1]
		}
		contentEnd := lineEnd
		for contentEnd > lineStart && (source[contentEnd-1] == '\n' || source[contentEnd-1] == '\r') {
			contentEnd--
		}
		if lineStart < skipUntil {
			continue
		}
		indentEnd := lineStart
		indent := 0
		for indentEnd < contentEnd && (source[indentEnd] == ' ' || source[indentEnd] == '\t') {
			if source[indentEnd] == '\t' {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "python_tab_indentation", Message: "tab indentation is retained and measured as four spaces"})
				indent = (indent/4 + 1) * 4
			} else {
				indent++
			}
			indentEnd++
		}
		trimmed := strings.TrimSpace(string(source[indentEnd:contentEnd]))
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if indent > indentStack[len(indentStack)-1] {
				indentStack = append(indentStack, indent)
				tokens = append(tokens, pythonToken(Indent, source, lineStart, indentEnd))
			} else {
				for indent < indentStack[len(indentStack)-1] && len(indentStack) > 1 {
					indentStack = indentStack[:len(indentStack)-1]
					tokens = append(tokens, pythonToken(Dedent, source, lineStart, lineStart))
				}
			}
		}
		for at := indentEnd; at < contentEnd; {
			if err := ctx.Err(); err != nil {
				return tokens, diagnostics, err
			}
			if source[at] == ' ' || source[at] == '\t' {
				at++
				continue
			}
			start := at
			switch {
			case source[at] == '#':
				at = contentEnd
				kind := Comment
				if strings.HasPrefix(strings.TrimSpace(string(source[start:at])), "# type") {
					kind = TypeComment
				}
				tokens = append(tokens, pythonToken(kind, source, start, at))
			case source[at] == '@':
				at++
				for at < contentEnd && (isIdentifier(source[at]) || source[at] == '.' || source[at] == '[' || source[at] == ']' || source[at] == '(' || source[at] == ')' || source[at] == ',') {
					at++
				}
				tokens = append(tokens, pythonToken(Decorator, source, start, at))
			case stringPrefixLength(source, at) > 0:
				prefix, quote, triple := stringPrefixLength(source, at), source[at+stringPrefixLength(source, at)-1], false
				if at+prefix+1 < len(source) && source[at+prefix] == quote && source[at+prefix+1] == quote {
					triple = true
				}
				end := stringEnd(source, at, prefix, quote, triple)
				if end <= at {
					end = contentEnd
				}
				at = end
				if triple && end > contentEnd {
					skipUntil = end
				}
				kind := String
				prefixText := strings.ToLower(string(source[start : start+prefix-1]))
				if strings.Contains(prefixText, "f") {
					kind = FString
				}
				if triple {
					kind = TripleString
				}
				if end == len(source) && !closedString(source, start, end, quote, triple) {
					diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "python_unclosed_string", Message: "string is not closed"})
				}
				tokens = append(tokens, pythonToken(kind, source, start, end))
			case source[at] == '\'' || source[at] == '"':
				quote := source[at]
				triple := at+2 < len(source) && source[at+1] == quote && source[at+2] == quote
				end := stringEnd(source, at, 1, quote, triple)
				at = end
				if triple && end > contentEnd {
					skipUntil = end
				}
				kind := String
				if triple {
					kind = TripleString
				}
				if end == len(source) && !closedString(source, start, end, quote, triple) {
					diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "python_unclosed_string", Message: "string is not closed"})
				}
				tokens = append(tokens, pythonToken(kind, source, start, end))
			case isIdentifier(source[at]):
				at++
				for at < contentEnd && (isIdentifier(source[at]) || source[at] >= '0' && source[at] <= '9') {
					at++
				}
				tokens = append(tokens, pythonToken(Identifier, source, start, at))
			case source[at] >= '0' && source[at] <= '9':
				at++
				for at < contentEnd && (source[at] >= '0' && source[at] <= '9' || source[at] == '.') {
					at++
				}
				tokens = append(tokens, pythonToken(Number, source, start, at))
			default:
				at++
				tokens = append(tokens, pythonToken(Punctuation, source, start, at))
			}
		}
	}
	for len(indentStack) > 1 {
		indentStack = indentStack[:len(indentStack)-1]
		tokens = append(tokens, pythonToken(Dedent, source, len(source), len(source)))
	}
	return tokens, diagnostics, nil
}

func pythonToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: lineAt(source, start), EndLine: lineAt(source, end)}
}

func lineStarts(source []byte) []int {
	starts := []int{0}
	for at, value := range source {
		if value == '\n' && at+1 < len(source) {
			starts = append(starts, at+1)
		}
	}
	return starts
}

func lineAt(source []byte, offset int) int {
	return 1 + strings.Count(string(source[:minPython(offset, len(source))]), "\n")
}
func minPython(left, right int) int {
	if left < right {
		return left
	}
	return right
}
func isIdentifier(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80 && unicode.IsLetter(rune(value))
}

func stringPrefixLength(source []byte, at int) int {
	for _, prefix := range []string{"rf", "fr", "rb", "br", "r", "f", "b", "u"} {
		if at+len(prefix) < len(source) && strings.EqualFold(string(source[at:at+len(prefix)]), prefix) && (source[at+len(prefix)] == '\'' || source[at+len(prefix)] == '"') {
			return len(prefix) + 1
		}
	}
	return 0
}

func stringEnd(source []byte, at, prefix int, quote byte, triple bool) int {
	open := at + prefix - 1
	if triple {
		needle := string([]byte{quote, quote, quote})
		if close := strings.Index(string(source[open+3:]), needle); close >= 0 {
			return open + 3 + close + 3
		}
		return len(source)
	}
	for position := open + 1; position < len(source); position++ {
		if source[position] == '\\' {
			position++
			continue
		}
		if source[position] == quote {
			return position + 1
		}
	}
	return len(source)
}

func closedString(source []byte, start, end int, quote byte, triple bool) bool {
	if end <= start {
		return false
	}
	if triple {
		return end >= 3 && source[end-3] == quote && source[end-2] == quote && source[end-1] == quote
	}
	return source[end-1] == quote
}
