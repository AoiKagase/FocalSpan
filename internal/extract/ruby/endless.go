package ruby

import (
	"regexp"
	"strings"
)

var rubyExpressionWord = regexp.MustCompile(`[A-Za-z_]\w*`)

// Find the assignment after the complete method header. An argument default
// and a setter name are not an endless method separator.
func rubyEndlessSeparator(text string) int {
	if !rubyCodeStarts(text, "def") {
		return -1
	}
	at := 3
	for at < len(text) && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	for at < len(text) && text[at] != ' ' && text[at] != '\t' && text[at] != '(' {
		at++
	}
	for at < len(text) && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	if at < len(text) && text[at] == '(' {
		depth := 0
		for at < len(text) {
			if text[at] == '(' {
				depth++
			}
			if text[at] == ')' {
				depth--
				if depth == 0 {
					at++
					break
				}
			}
			at++
		}
		if depth != 0 {
			return -1
		}
	}
	for at < len(text) && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	if at >= len(text) || text[at] != '=' || at+1 < len(text) && (text[at+1] == '=' || text[at+1] == '>') {
		return -1
	}
	return at + 1
}

func rubyEndlessEnd(lines []rubyLine, start int) (int, bool) {
	text := strings.TrimSpace(lines[start].Text)
	separator := rubyEndlessSeparator(text)
	if separator < 0 {
		return 0, false
	}
	delimiters, blocks, literalEnd := 0, 0, 0
	for at := start; at < len(lines); at++ {
		part := strings.TrimSpace(lines[at].Text)
		if at == start {
			part = text[separator:]
		}
		if at > start && blocks == 0 && lines[at].Indent <= lines[start].Indent && rubyCodeStarts(part, "def", "class", "module") {
			return at - 1, true
		}
		for _, c := range part {
			switch c {
			case '(', '[', '{':
				delimiters++
			case ')', ']', '}':
				delimiters--
			}
		}
		loop := false
		for _, span := range rubyExpressionWord.FindAllStringIndex(part, -1) {
			if span[0] > 0 && strings.ContainsRune(".:@$", rune(part[span[0]-1])) {
				continue
			}
			word := part[span[0]:span[1]]
			prefix := strings.TrimSpace(part[:span[0]])
			switch word {
			case "begin", "case":
				blocks++
			case "if", "unless", "while", "until", "for":
				if prefix == "" || strings.HasSuffix(prefix, ";") || strings.HasSuffix(prefix, "(") {
					blocks++
					if word == "while" || word == "until" || word == "for" {
						loop = true
					}
				}
			case "do":
				if !loop {
					blocks++
				}
			case "end":
				if blocks > 0 {
					blocks--
				}
			}
		}
		if lines[at].LiteralEndLine > literalEnd {
			literalEnd = lines[at].LiteralEndLine
		}
		continuation := false
		if at == start && strings.TrimSpace(part) == "" {
			raw := strings.TrimSpace(lines[start].Original)
			if raw == "" {
				raw = text
			}
			if separator <= len(raw) {
				rest := strings.TrimSpace(raw[separator:])
				continuation = rest == "" || strings.HasPrefix(rest, "#")
			}
		}
		trimmed := strings.TrimSpace(part)
		for _, suffix := range []string{"+", "-", "*", "/", "&&", "||", ",", ".", "\\", "="} {
			if strings.HasSuffix(trimmed, suffix) {
				continuation = true
			}
		}
		if delimiters <= 0 && blocks == 0 && at+1 >= literalEnd && !continuation {
			return at, true
		}
	}
	return len(lines) - 1, true
}
