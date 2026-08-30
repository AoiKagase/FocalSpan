package ruby

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
		if at == lineStart(source, at) && strings.HasPrefix(string(source[at:]), "=begin") {
			end := blockCommentEnd(source, at)
			tokens = append(tokens, rubyToken(BlockComment, source, at, end))
			if end == len(source) {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "ruby_unclosed_block_comment", Message: "=begin comment is not closed"})
			}
			at = end
			continue
		}
		if source[at] == '#' {
			for at < len(source) && source[at] != '\n' {
				at++
			}
			tokens = append(tokens, rubyToken(Comment, source, start, at))
			continue
		}
		if source[at] == '<' && at+2 < len(source) && source[at+1] == '<' {
			if end, ok := heredocEnd(source, at); ok {
				tokens = append(tokens, rubyToken(Heredoc, source, start, end))
				at = end
				continue
			}
		}
		if source[at] == '"' || source[at] == '\'' {
			quote := source[at]
			end := quotedEndRuby(source, at, quote)
			tokens = append(tokens, rubyToken(String, source, start, end))
			for offset := at; offset+2 < end; offset++ {
				if source[offset] == '#' && source[offset+1] == '{' {
					close := strings.IndexByte(string(source[offset+2:end]), '}')
					if close >= 0 {
						tokens = append(tokens, rubyToken(Interpolation, source, offset, offset+2+close+1))
						break
					}
				}
			}
			if end == len(source) && source[end-1] != quote {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "ruby_unclosed_string", Message: "string is not closed"})
			}
			at = end
			continue
		}
		if source[at] == ':' && at+1 < len(source) && isRubyIdent(source[at+1]) {
			at += 2
			for at < len(source) && isRubyIdent(source[at]) {
				at++
			}
			tokens = append(tokens, rubyToken(Symbol, source, start, at))
			continue
		}
		if source[at] == '%' && at+2 < len(source) && (source[at+1] == 'w' || source[at+1] == 'i' || source[at+1] == 'q') {
			end := percentEnd(source, at)
			tokens = append(tokens, rubyToken(PercentLiteral, source, start, end))
			at = end
			continue
		}
		if source[at] == '/' && at > 0 && strings.ContainsRune("=(:,[ ", rune(source[at-1])) {
			if end, ok := regexEndRuby(source, at); ok {
				tokens = append(tokens, rubyToken(Regex, source, start, end))
				at = end
				continue
			}
		}
		if isRubyIdent(source[at]) {
			at++
			for at < len(source) && (isRubyIdent(source[at]) || source[at] >= '0' && source[at] <= '9') {
				at++
			}
			kind := Identifier
			if rubyBlockWord[string(source[start:at])] {
				kind = BlockKeyword
			}
			tokens = append(tokens, rubyToken(kind, source, start, at))
			continue
		}
		at++
		tokens = append(tokens, rubyToken(Punctuation, source, start, at))
	}
	return tokens, diagnostics, nil
}

var rubyBlockWord = map[string]bool{"module": true, "class": true, "def": true, "do": true, "end": true, "if": true, "unless": true, "case": true, "begin": true}

func rubyToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: rubyLineNumber(source, start), EndLine: rubyLineNumber(source, end)}
}
func rubyLineNumber(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}
func lineStart(source []byte, at int) int {
	for at > 0 && source[at-1] != '\n' {
		at--
	}
	return at
}
func blockCommentEnd(source []byte, start int) int {
	if end := strings.Index(string(source[start:]), "\n=end"); end >= 0 {
		return start + end + len("\n=end")
	}
	return len(source)
}
func quotedEndRuby(source []byte, start int, quote byte) int {
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
func heredocEnd(source []byte, start int) (int, bool) {
	lineEnd := strings.IndexByte(string(source[start:]), '\n')
	if lineEnd < 0 {
		return len(source), false
	}
	header := strings.TrimSpace(string(source[start : start+lineEnd]))
	if !strings.HasPrefix(header, "<<") {
		return 0, false
	}
	marker := strings.Trim(strings.TrimSpace(strings.TrimPrefix(header, "<<")), "~-\"'")
	if marker == "" {
		return 0, false
	}
	rest := source[start+lineEnd+1:]
	for offset := 0; offset < len(rest); {
		next := bytesLineEnd(rest, offset)
		if strings.TrimSpace(string(rest[offset:next])) == marker {
			return start + lineEnd + 1 + next, true
		}
		offset = next
		if offset < len(rest) {
			offset++
		}
	}
	return len(source), false
}
func bytesLineEnd(source []byte, start int) int {
	for at := start; at < len(source); at++ {
		if source[at] == '\n' {
			return at
		}
	}
	return len(source)
}
func percentEnd(source []byte, start int) int {
	if start+2 >= len(source) {
		return len(source)
	}
	open := source[start+2]
	close := open
	if open == '[' {
		close = ']'
	} else if open == '(' {
		close = ')'
	} else if open == '{' {
		close = '}'
	}
	for at := start + 3; at < len(source); at++ {
		if source[at] == close {
			return at + 1
		}
	}
	return len(source)
}
func regexEndRuby(source []byte, start int) (int, bool) {
	for at := start + 1; at < len(source); at++ {
		if source[at] == '\\' {
			at++
			continue
		}
		if source[at] == '/' {
			at++
			for at < len(source) && source[at] >= 'a' && source[at] <= 'z' {
				at++
			}
			return at, true
		}
	}
	return len(source), false
}
func isRubyIdent(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
