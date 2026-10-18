package csharp

import "strings"

// genericNameBefore skips a balanced type argument list immediately before a
// parameter/argument list. On incomplete input it leaves the original position.
func genericNameBefore(tokens []Token, previous int) int {
	if previous < 0 || previous >= len(tokens) {
		return previous
	}
	text := tokens[previous].Text
	if text == "" || strings.Trim(text, ">") != "" {
		return previous
	}
	depth, round, square := 0, 0, 0
	for pos := previous; pos >= 0; pos = previousSignificant(tokens, pos) {
		text = tokens[pos].Text
		switch text {
		case ")":
			round++
			continue
		case "(":
			if round == 0 {
				return previous
			}
			round--
			continue
		case "]":
			square++
			continue
		case "[":
			if square == 0 {
				return previous
			}
			square--
			continue
		}
		if text != "" && strings.Trim(text, ">") == "" {
			depth += len(text)
			continue
		}
		if text == "<" {
			depth--
			if depth == 0 {
				if round != 0 || square != 0 {
					return previous
				}
				name := previousSignificant(tokens, pos)
				if name >= 0 && tokens[name].Kind == Identifier {
					return name
				}
				return previous
			}
		}
		if text == ";" || text == "{" || text == "}" {
			return previous
		}
	}
	return previous
}
