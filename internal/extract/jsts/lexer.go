package jsts

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, content []byte) ([]Token, []model.Diagnostic, error) {
	mapa := sourceutil.NewSourceMap(content)
	result := make([]Token, 0, len(content)/3)
	diagnostics := make([]model.Diagnostic, 0)
	for index := 0; index < len(content); {
		if err := ctx.Err(); err != nil {
			return nil, diagnostics, err
		}
		start := index
		value := content[index]
		if value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f' {
			index++
			for index < len(content) && strings.ContainsRune(" \t\r\n\f", rune(content[index])) {
				index++
			}
			result = append(result, makeToken(Whitespace, content, mapa, start, index))
			continue
		}
		if value == '/' && index+1 < len(content) && content[index+1] == '/' {
			index += 2
			for index < len(content) && content[index] != '\n' {
				index++
			}
			result = append(result, makeToken(LineComment, content, mapa, start, index))
			continue
		}
		if value == '/' && index+1 < len(content) && content[index+1] == '*' {
			index += 2
			closed := false
			for index+1 < len(content) {
				if content[index] == '*' && content[index+1] == '/' {
					index += 2
					closed = true
					break
				}
				index++
			}
			if !closed {
				index = len(content)
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "jsts_unclosed_comment", Message: "block comment is not closed"})
			}
			result = append(result, makeToken(BlockComment, content, mapa, start, index))
			continue
		}
		if value == '\'' || value == '"' {
			index = quotedEnd(content, index, value)
			result = append(result, makeToken(StringLiteral, content, mapa, start, index))
			if index >= len(content) && (len(content) == 0 || content[index-1] != value) {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "jsts_unclosed_string", Message: "string literal is not closed"})
			}
			continue
		}
		if value == '`' {
			index = templateEnd(content, index)
			result = append(result, makeToken(Template, content, mapa, start, index))
			if index >= len(content) || content[index-1] != '`' {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "jsts_unclosed_template", Message: "template literal is not closed"})
			}
			continue
		}
		if value == '/' && regexStart(result) {
			if end, ok := regexEnd(content, index); ok {
				result = append(result, makeToken(RegexLiteral, content, mapa, start, end))
				index = end
				continue
			}
		}
		if value == '<' && index+1 < len(content) && (isIdentStart(content[index+1]) || content[index+1] == '/' || content[index+1] == '>') && jsxStart(result) {
			if end, ok := jsxEnd(content, index); ok {
				result = append(result, makeToken(JSX, content, mapa, start, end))
				index = end
				continue
			}
		}
		if isIdentStart(value) || value == '#' {
			index++
			for index < len(content) && isIdentPart(content[index]) {
				index++
			}
			kind := Identifier
			if keywords[string(content[start:index])] {
				kind = Keyword
			}
			result = append(result, makeToken(kind, content, mapa, start, index))
			continue
		}
		if value >= '0' && value <= '9' {
			index++
			for index < len(content) && (isIdentPart(content[index]) || content[index] == '.') {
				index++
			}
			result = append(result, makeToken(Number, content, mapa, start, index))
			continue
		}
		if op := operatorAt(content[index:]); op != "" {
			index += len(op)
			kind := Operator
			if len(op) == 1 && strings.ContainsRune("{}()[],;.:?", rune(op[0])) {
				kind = Punctuation
			}
			result = append(result, makeToken(kind, content, mapa, start, index))
			continue
		}
		index++
		result = append(result, makeToken(Punctuation, content, mapa, start, index))
	}
	return result, diagnostics, nil
}

func makeToken(kind TokenKind, content []byte, mapa sourceutil.SourceMap, start, end int) Token {
	span, _ := mapa.Span(start, end)
	return Token{Kind: kind, Text: string(content[start:end]), StartByte: start, EndByte: end, StartLine: span.StartLine, EndLine: span.EndLine}
}

func quotedEnd(content []byte, index int, quote byte) int {
	index++
	for index < len(content) {
		if content[index] == '\\' {
			index += 2
			continue
		}
		if content[index] == quote {
			return index + 1
		}
		index++
	}
	return index
}

func templateEnd(content []byte, index int) int {
	index++
	depth := 0
	for index < len(content) {
		if content[index] == '\\' {
			index += 2
			continue
		}
		if content[index] == '`' && depth == 0 {
			return index + 1
		}
		if index+1 < len(content) && content[index] == '$' && content[index+1] == '{' {
			depth++
			index += 2
			continue
		}
		if content[index] == '}' && depth > 0 {
			depth--
		}
		index++
	}
	return index
}

func regexStart(tokens []Token) bool {
	for index := len(tokens) - 1; index >= 0; index-- {
		if !tokens[index].significant() {
			continue
		}
		text := tokens[index].Text
		return text == "=" || text == "(" || text == "[" || text == "{" || text == "," || text == ":" || text == "return" || text == "=>" || text == "!"
	}
	return true
}

func regexEnd(content []byte, index int) (int, bool) {
	index++
	class := false
	for index < len(content) {
		if content[index] == '\\' {
			index += 2
			continue
		}
		if content[index] == '[' {
			class = true
		} else if content[index] == ']' {
			class = false
		} else if content[index] == '/' && !class {
			index++
			for index < len(content) && isIdentPart(content[index]) {
				index++
			}
			return index, true
		} else if content[index] == '\n' {
			return 0, false
		}
		index++
	}
	return 0, false
}

func jsxStart(tokens []Token) bool {
	for index := len(tokens) - 1; index >= 0; index-- {
		if tokens[index].significant() {
			return tokens[index].Text == "return" || tokens[index].Text == "=" || tokens[index].Text == "(" || tokens[index].Text == "=>" || tokens[index].Text == ">"
		}
	}
	return false
}

func jsxEnd(content []byte, index int) (int, bool) {
	depth := 0
	for index < len(content) {
		if content[index] != '<' {
			index++
			continue
		}
		close := index+1 < len(content) && content[index+1] == '/'
		end := strings.IndexByte(string(content[index:]), '>')
		if end < 0 {
			return len(content), false
		}
		end += index
		selfClose := end > index && content[end-1] == '/'
		if close {
			depth--
		} else if !selfClose {
			depth++
		}
		index = end + 1
		if depth <= 0 {
			return index, true
		}
	}
	return len(content), false
}

func isIdentStart(value byte) bool {
	return value == '_' || value == '$' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80
}
func isIdentPart(value byte) bool {
	return isIdentStart(value) || value >= '0' && value <= '9' || value == '#'
}

func operatorAt(content []byte) string {
	for _, op := range []string{"===", "!==", ">>>=", "**=", "=>", "...", "?.", "??=", "??", "&&=", "||=", "++", "--", "**", "&&", "||", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "<<", ">>", ">>>"} {
		if strings.HasPrefix(string(content), op) {
			return op
		}
	}
	if len(content) == 0 {
		return ""
	}
	if strings.ContainsRune("{}()[],;.:?~+-*/%<>=!&|^@", rune(content[0])) {
		return string(content[0])
	}
	return ""
}
