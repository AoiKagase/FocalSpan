package cpp

// functionTail distinguishes declarator suffixes from the actual body. All
// positions refer to the original token stream, never a rewritten source.
func (p *parser) functionTail(close, hi int) (end, open, last, header int, ok bool) {
	open, last = -1, -1
	end = close
	header = p.tokens[p.sig[close]].EndByte
	text := func(i int) string {
		if i >= hi {
			return ""
		}
		return p.tokens[p.sig[i]].Text
	}
	skip := func(i int) int {
		if raw := p.matching[p.sig[i]]; raw > 0 {
			return p.positionOf(raw) + 1
		}
		return hi
	}
	for i := close + 1; i < hi; {
		switch text(i) {
		case ";":
			return i, -1, -1, p.tokens[p.sig[i]].EndByte, true
		case "=":
			if text(i+1) != "default" && text(i+1) != "delete" && text(i+1) != "0" {
				return end, open, last, header, false
			}
			for i < hi && text(i) != ";" {
				i++
			}
			if i < hi {
				return i, -1, -1, p.tokens[p.sig[i]].EndByte, true
			}
			return end, open, last, header, false
		case "requires":
			i++
			if text(i) == "requires" {
				i++
			}
			if text(i) == "(" {
				i = skip(i)
				if text(i) == "{" {
					i = skip(i)
				}
			} else if text(i) == "{" {
				i = skip(i)
			}
		case ":":
			i++
			for i < hi {
				for i < hi && text(i) != "(" && text(i) != "{" && text(i) != ";" {
					i++
				}
				if i >= hi || text(i) == ";" {
					return end, open, last, header, false
				}
				i = skip(i)
				if text(i) == "," {
					i++
					continue
				}
				break
			}
		case "(":
			i = skip(i)
		case "{":
			open = p.sig[i]
			last = p.matching[open]
			header = p.tokens[open].EndByte
			if last <= 0 {
				return hi - 1, open, last, header, true
			}
			end = p.positionOf(last)
			i = end + 1
			for text(i) == "catch" {
				i++
				if text(i) != "(" {
					break
				}
				i = skip(i)
				if text(i) != "{" {
					break
				}
				handler := p.matching[p.sig[i]]
				if handler <= 0 {
					return hi - 1, open, last, header, true
				}
				last = handler
				end = p.positionOf(last)
				i = end + 1
			}
			return end, open, last, header, true
		case "}":
			return end, open, last, header, false
		default:
			i++
		}
	}
	return end, open, last, header, false
}
