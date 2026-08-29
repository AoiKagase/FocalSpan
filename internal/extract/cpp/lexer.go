package cpp

import (
	"context"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

func Lex(ctx context.Context, content []byte) ([]Token, []model.Diagnostic, error) {
	mapa := sourceutil.NewSourceMap(content)
	result := make([]Token, 0, len(content)/3)
	diagnostics := make([]model.Diagnostic, 0)
	active := true
	conditionals := make([]conditionalFrame, 0)
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
			text := string(content[index:end])
			result = append(result, makeToken(Preprocessor, content, mapa, index, end, active))
			active, conditionals = applyDirective(text, active, conditionals)
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
			for index < len(content) && content[index] != '\n' {
				index++
			}
			result = append(result, makeToken(LineComment, content, mapa, start, index, active))
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
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "cpp_unclosed_comment", Message: "block comment is not closed"})
			}
			result = append(result, makeToken(BlockComment, content, mapa, start, index, active))
			continue
		}
		if index+1 < len(content) && content[index] == '[' && content[index+1] == '[' {
			index = consumeBalancedAttribute(content, index, '[', ']')
			result = append(result, makeToken(Attribute, content, mapa, start, index, active))
			continue
		}
		if rawEnd, ok := rawStringEnd(content, index); ok {
			result = append(result, makeToken(RawString, content, mapa, index, rawEnd, active))
			index = rawEnd
			continue
		}
		if value == '"' || value == '\'' || hasStringPrefix(content, index) {
			kind := StringLiteral
			if value == '\'' || (index+1 < len(content) && (content[index] == 'L' || content[index] == 'u' || content[index] == 'U') && content[index+1] == '\'') {
				kind = CharLiteral
			}
			index = consumeQuoted(content, index)
			if index == start {
				index++
			}
			if index >= len(content) && content[index-1] != value {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "cpp_unclosed_string", Message: "quoted literal is not closed"})
			}
			result = append(result, makeToken(kind, content, mapa, start, index, active))
			continue
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
			if strings.ContainsRune("{}()[];,.:?", rune(op[0])) && len(op) == 1 {
				kind = Punctuation
			}
			result = append(result, makeToken(kind, content, mapa, start, index, active))
			continue
		}
		index++
		result = append(result, makeToken(Punctuation, content, mapa, start, index, active))
	}
	if len(conditionals) > 0 {
		diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "cpp_unbalanced_preprocessor", Message: "preprocessor conditional is not closed"})
	}
	return result, diagnostics, nil
}

func makeToken(kind TokenKind, content []byte, mapa sourceutil.SourceMap, start, end int, active bool) Token {
	span, _ := mapa.Span(start, end)
	return Token{Kind: kind, Text: string(content[start:end]), StartByte: start, EndByte: end, StartLine: span.StartLine, EndLine: span.EndLine, Active: active}
}

type conditionalFrame struct{ parent, disabled bool }

func applyDirective(text string, active bool, stack []conditionalFrame) (bool, []conditionalFrame) {
	trimmed := strings.TrimSpace(text)
	fields := strings.Fields(strings.TrimPrefix(trimmed, "#"))
	if len(fields) == 0 {
		return active, stack
	}
	switch fields[0] {
	case "if", "ifdef", "ifndef":
		frame := conditionalFrame{parent: active}
		if fields[0] == "if" && len(fields) > 1 && fields[1] == "0" {
			frame.disabled = true
			active = false
		}
		stack = append(stack, frame)
	case "else", "elif":
		if len(stack) > 0 {
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

func isIdentStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80
}
func isIdentPart(value byte) bool { return isIdentStart(value) || value >= '0' && value <= '9' }

func hasStringPrefix(content []byte, index int) bool {
	if index+1 >= len(content) || !strings.ContainsRune("LuU", rune(content[index])) {
		return false
	}
	return content[index+1] == '"' || content[index+1] == '\''
}

func consumeQuoted(content []byte, index int) int {
	quote := index
	if index < len(content) && strings.ContainsRune("LuU", rune(content[index])) && index+1 < len(content) && (content[index+1] == '"' || content[index+1] == '\'') {
		index++
	}
	if index < len(content) && content[index] == 'u' && index+2 < len(content) && content[index+1] == '8' && (content[index+2] == '"' || content[index+2] == '\'') {
		index += 2
	}
	if index >= len(content) || (content[index] != '"' && content[index] != '\'') {
		return quote
	}
	endQuote := content[index]
	index++
	for index < len(content) {
		if content[index] == '\\' {
			index += 2
			continue
		}
		if content[index] == endQuote {
			return index + 1
		}
		index++
	}
	return index
}

func rawStringEnd(content []byte, index int) (int, bool) {
	if index+1 < len(content) && (content[index] == 'u' || content[index] == 'U' || content[index] == 'L') && content[index+1] == 'R' {
		index++
	} else if index+2 < len(content) && content[index] == 'u' && content[index+1] == '8' && content[index+2] == 'R' {
		index += 2
	} else if index >= len(content) || content[index] != 'R' {
		return 0, false
	}
	if index+1 >= len(content) || content[index+1] != '"' {
		return 0, false
	}
	delimStart := index + 2
	paren := strings.IndexByte(string(content[delimStart:]), '(')
	if paren < 0 || paren > 16 {
		return 0, false
	}
	delim := string(content[delimStart : delimStart+paren])
	needle := ")" + delim + "\""
	close := strings.Index(string(content[delimStart+paren+1:]), needle)
	if close < 0 {
		return len(content), true
	}
	return delimStart + paren + 1 + close + len(needle), true
}

func consumeBalancedAttribute(content []byte, index int, open, close byte) int {
	depth := 0
	for position := index; position+1 < len(content); position++ {
		if content[position] == '"' || content[position] == '\'' {
			quote := content[position]
			position++
			for position < len(content) {
				if content[position] == '\\' {
					position++
				} else if content[position] == quote {
					break
				}
				position++
			}
			continue
		}
		if content[position] == open && content[position+1] == open {
			depth++
			position++
			continue
		}
		if content[position] == close && content[position+1] == close {
			depth--
			position++
			if depth <= 0 {
				return position + 1
			}
		}
	}
	return len(content)
}

func operatorAt(content []byte) string {
	for _, op := range []string{"<=>", "->*", "<<=", ">>=", "...", "::", "->", ".*", "++", "--", "&&", "||", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "<<", ">>", "&=", "|=", "^=", "##"} {
		if strings.HasPrefix(string(content), op) {
			return op
		}
	}
	if len(content) == 0 {
		return ""
	}
	if strings.ContainsRune("{}()[];,.:?~+-*/%<>=!&|^#", rune(content[0])) || unicode.IsPunct(rune(content[0])) {
		return string(content[0])
	}
	return ""
}
