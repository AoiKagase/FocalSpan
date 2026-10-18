package cpp

import "strings"

func canonicalParameterKey(tokens []Token) (string, bool) {
	var parts []string
	start, paren, angle, bracket := 0, 0, 0, 0
	for i := 0; i <= len(tokens); i++ {
		if i == len(tokens) || (tokens[i].Text == "," && paren == 0 && angle == 0 && bracket == 0) {
			part := append([]Token(nil), tokens[start:i]...)
			cut := len(part)
			d := 0
			for j, t := range part {
				if t.Text == "(" || t.Text == "{" || t.Text == "[" {
					d++
				}
				if t.Text == ")" || t.Text == "}" || t.Text == "]" {
					d--
				}
				if t.Text == "=" && d == 0 {
					cut = j
					break
				}
			}
			part = part[:cut]
			candidate := -1
			idents, builtins := 0, false
			d = 0
			for j, t := range part {
				if t.Text == "(" || t.Text == "[" || t.Text == "<" {
					d++
				}
				if t.Text == ">>" {
					d -= 2
				}
				if t.Text == ")" || t.Text == "]" || t.Text == ">" {
					d--
				}
				if t.Kind == Identifier && d == 0 {
					idents++
					if (j == 0 || part[j-1].Text != "::") && (j+1 == len(part) || part[j+1].Text != "::") {
						candidate = j
					}
				}
				if typeWords[t.Text] && t.Text != "const" && t.Text != "volatile" {
					builtins = true
				}
				if t.Kind == Identifier && j > 0 && (part[j-1].Text == "*" || part[j-1].Text == "&") && d > 0 {
					part[j].Text = ""
				}
			}
			if candidate >= 0 && (idents > 1 || builtins) {
				part[candidate].Text = ""
			}

			// Normalize parameter names inside function-pointer/function declarators,
			// keeping the declarator parentheses and the source signature untouched.
			for j := 1; j < len(part); j++ {
				if part[j].Text == "(" && (part[j-1].Text == ")" || part[j-1].Text == "") {
					depth := 1
					k := j + 1
					for k < len(part) && depth > 0 {
						if part[k].Text == "(" {
							depth++
						}
						if part[k].Text == ")" {
							depth--
						}
						k++
					}
					if depth == 0 {
						key, ok := canonicalParameterKey(part[j+1 : k-1])
						if ok {
							if j+1 < k-1 {
								part[j+1].Text = key
								for l := j + 2; l < k-1; l++ {
									part[l].Text = ""
								}
							}
						}
						j = k - 1
					}
				}
			}
			var words []string
			for _, t := range part {
				if t.Text != "" {
					words = append(words, t.Text)
				}
			}
			parts = append(parts, strings.Join(words, " "))
			start = i + 1
			continue
		}
		switch tokens[i].Text {
		case "(":
			paren++
		case ")":
			paren--
		case "[":
			bracket++
		case "]":
			bracket--
		case "<":
			angle++
		case ">":
			angle--
		case ">>":
			angle -= 2
		}
	}
	return strings.Join(parts, ","), true
}
func (p *parser) callableTypeKey(open, close, body int) string {
	ts := make([]Token, 0, close-open)
	for _, raw := range p.sig[open+1 : close] {
		ts = append(ts, p.tokens[raw])
	}
	key, _ := canonicalParameterKey(ts)
	for i := close + 1; i < len(p.sig); i++ {
		raw := p.sig[i]
		v := p.tokens[raw].Text
		if raw == body || v == ";" || v == "{" || v == ":" || v == "=" {
			break
		}
		if v == "const" || v == "volatile" || v == "&" || v == "&&" {
			key += " " + v
		}
	}
	return key
}
