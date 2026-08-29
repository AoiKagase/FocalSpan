package csharp

import (
	"context"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

type conditionalFrame struct{ parent, disabled bool }

func Lex(ctx context.Context, content []byte) ([]Token, []model.Diagnostic, error) {
	mapa := sourceutil.NewSourceMap(content)
	result := make([]Token, 0, len(content)/3)
	diagnostics := make([]model.Diagnostic, 0)
	active := true
	stack := make([]conditionalFrame, 0)
	lineStart := true
	for index := 0; index < len(content); {
		if err := ctx.Err(); err != nil {
			return nil, diagnostics, err
		}
		start := index
		value := content[index]
		if lineStart && (value == ' ' || value == '\t' || value == '\r') {
			index++
			continue
		}
		if lineStart && value == '#' {
			end := index
			for end < len(content) {
				lineEnd := end
				for lineEnd < len(content) && content[lineEnd] != '\n' {
					lineEnd++
				}
				trimmed := strings.TrimSpace(string(content[index:lineEnd]))
				end = lineEnd
				if lineEnd >= len(content) || !strings.HasSuffix(trimmed, "\\") {
					break
				}
				end = lineEnd + 1
			}
			if end < len(content) && content[end] == '\n' {
				end++
			}
			result = append(result, makeToken(Preprocessor, content, mapa, index, end, active))
			active, stack = applyDirective(string(content[index:end]), active, stack)
			index = end
			lineStart = true
			continue
		}
		if value == '\n' {
			index++
			lineStart = true
			continue
		}
		lineStart = false
		if value == ' ' || value == '\t' || value == '\r' || value == '\f' {
			index++
			for index < len(content) && (content[index] == ' ' || content[index] == '\t' || content[index] == '\r' || content[index] == '\f') {
				index++
			}
			result = append(result, makeToken(Whitespace, content, mapa, start, index, active))
			continue
		}
		if value == '/' && index+1 < len(content) && content[index+1] == '/' {
			index += 2
			kind := LineComment
			if index < len(content) && content[index] == '/' {
				kind = XMLDocComment
			}
			for index < len(content) && content[index] != '\n' {
				index++
			}
			result = append(result, makeToken(kind, content, mapa, start, index, active))
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
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "csharp_unclosed_comment", Message: "block comment is not closed"})
			}
			result = append(result, makeToken(BlockComment, content, mapa, start, index, active))
			continue
		}
		if rawEnd, kind, ok := stringEnd(content, index); ok {
			result = append(result, makeToken(kind, content, mapa, start, rawEnd, active))
			index = rawEnd
			if rawEnd == len(content) && (len(content) == 0 || content[rawEnd-1] != '"') {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "csharp_unclosed_string", Message: "string literal is not closed"})
			}
			continue
		}
		if value == '\'' {
			index = quotedEnd(content, index, '\'')
			result = append(result, makeToken(CharLiteral, content, mapa, start, index, active))
			continue
		}
		if value == '[' && attributeStart(result) {
			if end := attributeEnd(content, index); end > index {
				result = append(result, makeToken(Attribute, content, mapa, index, end, active))
				index = end
				continue
			}
		}
		if isIdentStart(value) {
			index++
			for index < len(content) && isIdentPart(content[index]) {
				index++
			}
			kind := Identifier
			if keywords[string(content[start:index])] {
				kind = Keyword
			}
			result = append(result, makeToken(kind, content, mapa, start, index, active))
			continue
		}
		if value >= '0' && value <= '9' {
			index++
			for index < len(content) && (isIdentPart(content[index]) || content[index] == '.') {
				index++
			}
			result = append(result, makeToken(Number, content, mapa, start, index, active))
			continue
		}
		if op := operatorAt(content[index:]); op != "" {
			index += len(op)
			kind := Operator
			if len(op) == 1 && strings.ContainsRune("{}()[];,.:?", rune(op[0])) {
				kind = Punctuation
			}
			result = append(result, makeToken(kind, content, mapa, start, index, active))
			continue
		}
		index++
		result = append(result, makeToken(Punctuation, content, mapa, start, index, active))
	}
	if len(stack) > 0 {
		diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "csharp_unbalanced_preprocessor", Message: "preprocessor conditional is not closed"})
	}
	return result, diagnostics, nil
}

func attributeStart(tokens []Token) bool {
	for index := len(tokens) - 1; index >= 0; index-- {
		if !tokens[index].significant() {
			continue
		}
		return tokens[index].Text == "{" || tokens[index].Text == "}" || tokens[index].Text == ";" || tokens[index].Text == ")" || tokens[index].Text == ":"
	}
	return true
}

func makeToken(kind TokenKind, content []byte, mapa sourceutil.SourceMap, start, end int, active bool) Token {
	span, _ := mapa.Span(start, end)
	return Token{Kind: kind, Text: string(content[start:end]), StartByte: start, EndByte: end, StartLine: span.StartLine, EndLine: span.EndLine, Active: active}
}

func applyDirective(text string, active bool, stack []conditionalFrame) (bool, []conditionalFrame) {
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(text), "#"))
	if len(fields) == 0 {
		return active, stack
	}
	switch fields[0] {
	case "if", "elif", "else":
		if fields[0] == "if" {
			frame := conditionalFrame{parent: active}
			if len(fields) > 1 && fields[1] == "false" || len(fields) > 1 && fields[1] == "0" {
				frame.disabled = true
				active = false
			}
			stack = append(stack, frame)
		} else if len(stack) > 0 {
			frame := &stack[len(stack)-1]
			if frame.disabled {
				active = frame.parent
				frame.disabled = false
			} else {
				active = frame.parent
			}
		}
	case "endif":
		if len(stack) > 0 {
			frame := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			active = frame.parent
		}
	}
	return active, stack
}

func stringEnd(content []byte, index int) (int, TokenKind, bool) {
	start := index
	interpolated := false
	verbatim := false
	if index < len(content) && content[index] == '$' {
		interpolated = true
		index++
	}
	if index < len(content) && content[index] == '@' {
		verbatim = true
		index++
	}
	if index < len(content) && content[index] == '$' {
		interpolated = true
		index++
	}
	if index+2 < len(content) && content[index] == '"' && content[index+1] == '"' && content[index+2] == '"' {
		quotes := 0
		for index+quotes < len(content) && content[index+quotes] == '"' {
			quotes++
		}
		if quotes < 3 {
			return 0, "", false
		}
		close := strings.Index(string(content[index+quotes:]), strings.Repeat("\"", quotes))
		if close < 0 {
			return len(content), RawString, true
		}
		return index + quotes + close + quotes, RawString, true
	}
	if index >= len(content) || content[index] != '"' || index == start && content[index] != '"' {
		return 0, "", false
	}
	end := quotedEnd(content, index, '"')
	kind := NormalString
	if verbatim {
		kind = VerbatimString
	}
	if interpolated {
		kind = InterpolatedString
	}
	return end, kind, true
}

func quotedEnd(content []byte, index int, quote byte) int {
	index++
	for index < len(content) {
		if content[index] == '\\' && quote == '"' {
			index += 2
			continue
		}
		if content[index] == quote {
			if index+1 < len(content) && content[index+1] == quote && quote == '"' {
				index += 2
				continue
			}
			return index + 1
		}
		index++
	}
	return index
}

func attributeEnd(content []byte, index int) int {
	if index+1 >= len(content) || content[index] != '[' {
		return 0
	}
	depth := 0
	for position := index; position+1 < len(content); position++ {
		if content[position] == '[' {
			depth++
		} else if content[position] == ']' {
			depth--
			if depth == 0 {
				return position + 1
			}
		}
	}
	return 0
}

func isIdentStart(value byte) bool {
	return value == '_' || value == '@' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80
}
func isIdentPart(value byte) bool { return isIdentStart(value) || value >= '0' && value <= '9' }

func operatorAt(content []byte) string {
	for _, op := range []string{">>=", ">>>", "=>", "??=", "?.", "??", "++", "--", "&&", "||", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "<<", ">>", "::", "..", "&=", "|=", "^="} {
		if strings.HasPrefix(string(content), op) {
			return op
		}
	}
	if len(content) == 0 {
		return ""
	}
	if strings.ContainsRune("{}()[];,.:?~+-*/%<>=!&|^@", rune(content[0])) || unicode.IsPunct(rune(content[0])) {
		return string(content[0])
	}
	return ""
}
