package cpp

// parametersLikely rejects unambiguous initializer expressions. Unknown type
// identifiers remain syntactic candidates; this is not a C++ type checker.
func (p *parser) parametersLikely(lo, hi int) bool {
	first, depth, angle, defaultValue := true, 0, 0, false
	for i := lo; i < hi; i++ {
		t := p.tokens[p.sig[i]]
		v := t.Text
		if depth == 0 && angle == 0 {
			if v == "," {
				first = true
				defaultValue = false
				continue
			}
			if v == "=" {
				defaultValue = true
				continue
			}
			if !defaultValue {
				if first {
					switch t.Kind {
					case Number, StringLiteral, RawString, CharLiteral:
						return false
					}
					if v == "true" || v == "false" || v == "nullptr" || v == "&" || v == "+" || v == "-" || v == "!" || v == "{" {
						return false
					}
					first = false
				}
				if v == "+" || v == "-" || v == "/" || v == "%" || v == "." || v == "->" || v == "{" || v == "||" {
					return false
				}
			}
		}
		switch v {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case "<":
			angle++
		case ">":
			angle--
		case ">>":
			angle -= 2
		}
	}
	return true
}
