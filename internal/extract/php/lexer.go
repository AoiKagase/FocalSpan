package php

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/model"
)

type lexer struct {
	ctx         context.Context
	content     []byte
	offset      int
	line        int
	inPHP       bool
	tokens      []Token
	diagnostics []model.Diagnostic
}

func Lex(ctx context.Context, content []byte) ([]Token, []model.Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	l := &lexer{ctx: ctx, content: content, line: 1}
	for l.offset < len(l.content) {
		if err := l.ctx.Err(); err != nil {
			return nil, nil, err
		}
		if l.inPHP {
			l.scanPHP()
		} else {
			l.scanHTML()
		}
	}
	return l.tokens, l.diagnostics, nil
}

func (l *lexer) scanHTML() {
	if end, ok := l.openTagAt(l.offset); ok {
		l.emit(KindOpenTag, l.offset, end)
		l.offset = end
		l.inPHP = true
		return
	}
	start := l.offset
	for l.offset < len(l.content) {
		if l.ctx.Err() != nil {
			return
		}
		if _, ok := l.openTagAt(l.offset); ok {
			break
		}
		l.offset++
	}
	if l.offset == start {
		l.offset++
		return
	}
	l.emit(KindInlineHTML, start, l.offset)
}

func (l *lexer) scanPHP() {
	start := l.offset
	if l.hasPrefix(l.offset, "?>") {
		l.emit(KindCloseTag, l.offset, l.offset+2)
		l.offset += 2
		l.inPHP = false
		return
	}
	if isPHPWhitespace(l.content[l.offset]) {
		for l.offset < len(l.content) && isPHPWhitespace(l.content[l.offset]) {
			if l.ctx.Err() != nil {
				return
			}
			l.offset++
		}
		l.emit(KindWhitespace, start, l.offset)
		return
	}
	if l.hasPrefix(l.offset, "//") || (l.content[l.offset] == '#' && !l.hasPrefix(l.offset, "#[")) {
		if l.hasPrefix(l.offset, "//") {
			l.offset += 2
		} else {
			l.offset++
		}
		for l.offset < len(l.content) && l.content[l.offset] != '\n' {
			if l.ctx.Err() != nil {
				return
			}
			l.offset++
		}
		l.emit(KindLineComment, start, l.offset)
		return
	}
	if l.hasPrefix(l.offset, "/*") {
		kind := KindBlockComment
		if l.hasPrefix(l.offset, "/**") {
			kind = KindDocComment
		}
		l.offset += 2
		for l.offset < len(l.content) && !l.hasPrefix(l.offset, "*/") {
			if l.ctx.Err() != nil {
				return
			}
			l.offset++
		}
		if l.offset >= len(l.content) {
			l.addDiagnostic("php_unclosed_comment", "PHP block comment reaches end of file")
		} else {
			l.offset += 2
		}
		l.emit(kind, start, l.offset)
		return
	}
	if quote := l.content[l.offset]; quote == '\'' || quote == '"' || quote == '`' {
		l.scanQuoted(quote)
		return
	}
	if l.hasPrefix(l.offset, "<<<") {
		if end, kind, ok := l.scanHeredoc(l.offset); ok {
			l.offset = end
			l.emit(kind, start, l.offset)
			return
		}
	}
	if l.content[l.offset] == '$' {
		l.offset++
		if l.offset < len(l.content) {
			if end := identifierEnd(l.content, l.offset); end > l.offset {
				l.offset = end
				l.emit(KindVariable, start, l.offset)
				return
			}
		}
		l.emit(KindPunctuation, start, l.offset)
		return
	}
	if end := identifierEnd(l.content, l.offset); end > l.offset {
		l.offset = end
		kind := KindIdentifier
		if phpKeywords[strings.ToLower(string(l.content[start:l.offset]))] {
			kind = KindKeyword
		}
		l.emit(kind, start, l.offset)
		return
	}
	for _, operator := range phpOperators {
		if l.hasPrefix(l.offset, operator) {
			l.offset += len(operator)
			l.emit(KindOperator, start, l.offset)
			return
		}
	}
	l.offset++
	if isPHPunctuation(l.content[start]) {
		l.emit(KindPunctuation, start, l.offset)
	} else {
		l.emit(KindUnknown, start, l.offset)
	}
}

func (l *lexer) scanQuoted(quote byte) {
	start := l.offset
	l.offset++
	escaped := false
	for l.offset < len(l.content) {
		if err := l.ctx.Err(); err != nil {
			return
		}
		value := l.content[l.offset]
		l.offset++
		if escaped {
			escaped = false
			continue
		}
		if value == '\\' {
			escaped = true
			continue
		}
		if value == quote {
			kind := KindSingleQuotedString
			switch quote {
			case '"':
				kind = KindDoubleQuotedString
			case '`':
				kind = KindBacktickString
			}
			l.emit(kind, start, l.offset)
			return
		}
	}
	kind := KindSingleQuotedString
	switch quote {
	case '"':
		kind = KindDoubleQuotedString
	case '`':
		kind = KindBacktickString
	}
	l.addDiagnostic("php_unclosed_string", "PHP quoted string reaches end of file")
	l.emit(kind, start, l.offset)
}

func (l *lexer) scanHeredoc(start int) (int, Kind, bool) {
	position := start + 3
	for position < len(l.content) && (l.content[position] == ' ' || l.content[position] == '\t') {
		if l.ctx.Err() != nil {
			return 0, KindUnknown, false
		}
		position++
	}
	kind := KindHeredoc
	if position < len(l.content) && (l.content[position] == '\'' || l.content[position] == '"') {
		if l.content[position] == '\'' {
			kind = KindNowdoc
		}
		position++
	}
	labelStart := position
	if position >= len(l.content) || !isASCIIIdentifierStart(l.content[position]) {
		return 0, KindUnknown, false
	}
	for position < len(l.content) && isASCIIIdentifierPart(l.content[position]) {
		if l.ctx.Err() != nil {
			return 0, KindUnknown, false
		}
		position++
	}
	label := string(l.content[labelStart:position])
	if position < len(l.content) && (l.content[position] == '\'' || l.content[position] == '"') {
		position++
	}
	if position >= len(l.content) || (l.content[position] != '\n' && !(l.content[position] == '\r' && position+1 < len(l.content) && l.content[position+1] == '\n')) {
		return 0, KindUnknown, false
	}
	lineStart := position + 1
	if l.content[position] == '\r' {
		lineStart++
	}
	for lineStart <= len(l.content) {
		if l.ctx.Err() != nil {
			return 0, KindUnknown, false
		}
		lineEnd := lineStart
		for lineEnd < len(l.content) && l.content[lineEnd] != '\n' {
			if l.ctx.Err() != nil {
				return 0, KindUnknown, false
			}
			lineEnd++
		}
		line := strings.TrimSuffix(string(l.content[lineStart:lineEnd]), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == label || trimmed == label+";" {
			terminator := strings.Index(line, label)
			if terminator < 0 {
				terminator = 0
			}
			return lineStart + terminator + len(label), kind, true
		}
		if lineEnd >= len(l.content) {
			break
		}
		lineStart = lineEnd + 1
	}
	l.addDiagnostic("php_unclosed_heredoc", "PHP heredoc reaches end of file")
	return len(l.content), kind, true
}

func (l *lexer) emit(kind Kind, start, end int) {
	if end <= start || start < 0 || end > len(l.content) {
		return
	}
	startLine := 1 + strings.Count(string(l.content[:start]), "\n")
	endLine := startLine + strings.Count(string(l.content[start:end]), "\n")
	l.tokens = append(l.tokens, Token{Kind: kind, Text: string(l.content[start:end]), StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine})
}

func (l *lexer) addDiagnostic(code, message string) {
	l.diagnostics = append(l.diagnostics, model.Diagnostic{Level: "warning", Code: code, Message: message})
}

func (l *lexer) hasPrefix(offset int, prefix string) bool {
	return offset >= 0 && offset+len(prefix) <= len(l.content) && strings.EqualFold(string(l.content[offset:offset+len(prefix)]), prefix)
}

func (l *lexer) openTagAt(offset int) (int, bool) {
	if offset+2 > len(l.content) || l.content[offset] != '<' || l.content[offset+1] != '?' {
		return 0, false
	}
	rest := l.content[offset+2:]
	if len(rest) >= 3 && strings.EqualFold(string(rest[:3]), "php") && (len(rest) == 3 || !isASCIIIdentifierPart(rest[3])) {
		return offset + 5, true
	}
	if len(rest) >= 1 && rest[0] == '=' {
		return offset + 3, true
	}
	if len(rest) >= 3 && strings.EqualFold(string(rest[:3]), "xml") {
		return 0, false
	}
	return offset + 2, true
}

func identifierEnd(content []byte, start int) int {
	if start >= len(content) {
		return start
	}
	if content[start] < utf8.RuneSelf {
		if !isASCIIIdentifierStart(content[start]) {
			return start
		}
		end := start + 1
		for end < len(content) && isASCIIIdentifierPart(content[end]) {
			end++
		}
		return end
	}
	if !utf8.Valid(content[start:]) {
		return start
	}
	r, size := utf8.DecodeRune(content[start:])
	if !unicode.IsLetter(r) && r != '_' {
		return start
	}
	end := start + size
	for end < len(content) {
		r, size = utf8.DecodeRune(content[end:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		end += size
	}
	return end
}

func isASCIIIdentifierStart(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value == '_'
}

func isASCIIIdentifierPart(value byte) bool {
	return isASCIIIdentifierStart(value) || value >= '0' && value <= '9'
}

func isPHPWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f' || value == '\v'
}

func isPHPunctuation(value byte) bool {
	return strings.ContainsRune("{}[]();,.:?@#", rune(value))
}

var phpOperators = []string{
	"?->", "??=", "===", "!==", "**=", "<<=", ">>=", "...", "=>", "==", "!=", "<=", ">=", "&&", "||", "??", "::", "->", "++", "--", "+=", "-=", "*=", "/=", "%=", ".=", "&=", "|=", "^=", "<<", ">>", "**", "=", "+", "-", "*", "/", "%", ".", "<", ">", "!", "&", "|", "^", "~",
}

var phpKeywords = map[string]bool{
	"abstract": true, "and": true, "array": true, "as": true, "break": true, "callable": true, "case": true, "catch": true, "class": true, "clone": true, "const": true, "continue": true, "declare": true, "default": true, "die": true, "do": true, "echo": true, "else": true, "elseif": true, "empty": true, "enddeclare": true, "endfor": true, "endforeach": true, "endif": true, "endswitch": true, "endwhile": true, "enum": true, "eval": true, "exit": true, "extends": true, "final": true, "finally": true, "fn": true, "for": true, "foreach": true, "function": true, "global": true, "goto": true, "if": true, "implements": true, "include": true, "include_once": true, "instanceof": true, "insteadof": true, "interface": true, "isset": true, "list": true, "match": true, "namespace": true, "new": true, "or": true, "print": true, "private": true, "protected": true, "public": true, "readonly": true, "require": true, "require_once": true, "return": true, "static": true, "switch": true, "throw": true, "trait": true, "try": true, "unset": true, "use": true, "var": true, "while": true, "xor": true, "yield": true, "null": true, "true": true, "false": true,
}
