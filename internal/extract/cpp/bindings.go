package cpp

type localBinding struct {
	name       string
	start, end int
}
type bindingIndex []localBinding

func (b bindingIndex) shadows(name string, atByte int) bool {
	for _, v := range b {
		if v.name == name && atByte >= v.start && atByte < v.end {
			return true
		}
	}
	return false
}
func (b *builder) bindingFor(decl *declaration) bindingIndex {
	if b.bindings == nil {
		b.bindings = make(map[*declaration]bindingIndex)
	}
	if v, ok := b.bindings[decl]; ok {
		return v
	}
	v := buildBindingIndex(b.parsed.Tokens, decl)
	b.bindings[decl] = v
	return v
}
func bindingName(ts []Token) int {
	depth, angle, idents, candidate := 0, 0, 0, -1
	builtin := false
	for i, t := range ts {
		v := t.Text
		if v == "return" || v == "throw" || v == "." || v == "->" || v == "||" || controlWords[v] {
			return -1
		}
		if v == "=" {
			break
		}
		if v == "(" {
			depth++
		}
		if v == ")" {
			depth--
		}
		if v == "<" {
			angle++
		}
		if v == ">" {
			angle--
		}
		if v == ">>" {
			angle -= 2
		}
		if t.Kind == Identifier && depth > 0 && i > 0 && (ts[i-1].Text == "*" || ts[i-1].Text == "&") {
			return i
		}
		if depth == 0 && angle == 0 {
			if typeWords[v] && v != "const" && v != "volatile" {
				builtin = true
			}
			if t.Kind == Identifier {
				idents++
				if (i == 0 || ts[i-1].Text != "::") && (i+1 == len(ts) || ts[i+1].Text != "::") {
					candidate = i
				}
			}
		}
	}
	if candidate >= 0 && (builtin || idents > 1) {
		return candidate
	}
	return -1
}
func buildBindingIndex(tokens []Token, decl *declaration) bindingIndex {
	if decl.ParamOpen <= 0 || decl.ParamClose <= decl.ParamOpen {
		return nil
	}
	var sig []Token
	for _, t := range tokens {
		if t.significant() {
			sig = append(sig, t)
		}
	}
	pairs := map[int]int{}
	var stack []int
	for i, t := range sig {
		if t.Text == "(" || t.Text == "{" || t.Text == "[" {
			stack = append(stack, i)
		}
		if t.Text == ")" || t.Text == "}" || t.Text == "]" {
			if len(stack) > 0 {
				j := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pairs[j] = i
				pairs[i] = j
			}
		}
	}
	var result bindingIndex
	addParams := func(lo, hi, end int) {
		start, depth, angle := lo, 0, 0
		for i := lo; i <= hi; i++ {
			if i == hi || (sig[i].Text == "," && depth == 0 && angle == 0) {
				if n := bindingName(sig[start:i]); n >= 0 {
					t := sig[start+n]
					result = append(result, localBinding{t.Text, t.StartByte, end})
				}
				start = i + 1
				continue
			}
			switch sig[i].Text {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			case "<":
				angle++
			case ">":
				angle--
			case ">>":
				angle -= 2
			}
		}
	}
	open, close := -1, -1
	for i, t := range sig {
		if t.StartByte == tokens[decl.ParamOpen].StartByte {
			open = i
		}
		if t.StartByte == tokens[decl.ParamClose].StartByte {
			close = i
		}
	}
	if open < 0 || close < 0 {
		return result
	}
	addParams(open+1, close, decl.End)

	tailParser := parser{tokens: sig, matching: pairs}
	for i := range sig {
		tailParser.sig = append(tailParser.sig, i)
	}
	bodyAt := func(after int) (int, int, bool) {
		_, open, _, _, ok := tailParser.functionTail(after, len(sig))
		if !ok || open < 0 {
			return 0, 0, false
		}
		end, ok := pairs[open]
		return open, end, ok
	}
	var statementEnd func(int) int
	statementEnd = func(start int) int {
		if start >= len(sig) {
			return len(sig) - 1
		}

		if sig[start].Text == "if" || sig[start].Text == "for" || sig[start].Text == "while" || sig[start].Text == "switch" {
			op := start + 1
			if op < len(sig) && sig[op].Text == "constexpr" {
				op++
			}
			if op < len(sig) && sig[op].Text == "(" {
				if close, ok := pairs[op]; ok {
					end := statementEnd(close + 1)
					if sig[start].Text == "if" && end+1 < len(sig) && sig[end+1].Text == "else" {
						return statementEnd(end + 2)
					}
					return end
				}
			}
		}
		if sig[start].Text == "{" {
			if end, ok := pairs[start]; ok {
				return end
			}
		}
		for j := start; j < len(sig); j++ {
			if sig[j].Text == ";" {
				return j
			}
			if sig[j].Text == "(" || sig[j].Text == "[" || sig[j].Text == "{" {
				if end, ok := pairs[j]; ok {
					j = end
				}
			}
		}
		return len(sig) - 1
	}
	for i := close + 1; i < len(sig) && sig[i].StartByte < decl.End; i++ {
		if decl.BodyOpen < 0 || sig[i].StartByte < tokens[decl.BodyOpen].StartByte {
			continue
		}
		if sig[i].Text == "for" || sig[i].Text == "if" || sig[i].Text == "switch" || sig[i].Text == "while" {
			op := i + 1
			if op < len(sig) && sig[op].Text == "constexpr" {
				op++
			}
			if op < len(sig) && sig[op].Text == "(" {
				if c, ok := pairs[op]; ok {
					end := op + 1
					for end < c && sig[end].Text != ";" && sig[end].Text != ":" {
						end++
					}
					if n := bindingName(sig[op+1 : end]); n >= 0 {
						t := sig[op+1+n]
						last := statementEnd(c + 1)
						if sig[i].Text == "if" && last+1 < len(sig) && sig[last+1].Text == "else" {
							last = statementEnd(last + 2)
						}
						result = append(result, localBinding{t.Text, t.StartByte, sig[last].EndByte})
					}
				}
			}
		}
		if sig[i].Text == "[" {
			if c, ok := pairs[i]; ok {
				if open, end, ok := bodyAt(c); ok {
					for j := i + 1; j < c; j++ {
						if sig[j].Kind == Identifier && (j == i+1 || sig[j-1].Text == "," || sig[j-1].Text == "&") {
							result = append(result, localBinding{sig[j].Text, sig[open].StartByte, sig[end].EndByte})
						}
						if sig[j].Text == "=" {
							for j+1 < c && sig[j+1].Text != "," {
								j++
							}
						}
					}
				}
			}
		}
		if sig[i].Text == "(" && i > 0 && (sig[i-1].Text == "catch" || sig[i-1].Text == "]") {
			if c, ok := pairs[i]; ok {
				if _, end, ok := bodyAt(c); ok {
					addParams(i+1, c, sig[end].EndByte)
				}
			}
		}
		if sig[i].Text != "{" && sig[i].Text != ";" {
			continue
		}
		start := i + 1
		end := start
		for end < len(sig) && sig[end].StartByte < decl.End {
			v := sig[end].Text
			if v == ";" || v == "=" || v == "{" || v == "}" {
				break
			}
			end++
		}
		if n := bindingName(sig[start:end]); n >= 0 {
			t := sig[start+n]
			extent := decl.End
			for j := i; j >= 0; j-- {
				if sig[j].Text == "{" {
					if k, ok := pairs[j]; ok && k >= start {
						extent = sig[k].EndByte
						break
					}
				}
			}
			result = append(result, localBinding{t.Text, t.StartByte, extent})
		}
	}

	return result
}
