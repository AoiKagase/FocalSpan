package jsts

import "strings"

// bindingIndex deliberately resolves declarations, not runtime values. A local
// callback/object must never fall back to an unrelated package-level name.
type lexicalBinding struct {
	name        string
	start, end  int
	declaration *declaration
}
type bindingIndex struct {
	entries      []lexicalBinding
	names        map[int]bool
	declarations []*declaration
	tokens       []Token
	sig          []int
	pairs        map[int]int
}

func buildBindingIndex(tokens []Token, declarations []*declaration) bindingIndex {
	b := bindingIndex{names: map[int]bool{}, declarations: declarations, tokens: tokens, pairs: map[int]int{}}
	stack := []int{}
	for n, t := range tokens {
		if !t.significant() {
			continue
		}
		b.sig = append(b.sig, n)
		switch t.Text {
		case "(", "[", "{":
			stack = append(stack, n)
		case ")", "]", "}":
			if len(stack) > 0 {
				open := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if tokens[open].Text+t.Text == "()" || tokens[open].Text+t.Text == "[]" || tokens[open].Text+t.Text == "{}" {
					b.pairs[open] = n
				}
			}
		}
	}
	chosen := preferredDeclarations(declarations)
	for _, d := range declarations {
		if chosen[declarationKey(d)] != d {
			continue
		}
		if !callableDeclaration(d) {
			continue
		}
		start, end := b.scopeAt(tokens[d.Start].StartByte, false)
		b.entries = append(b.entries, lexicalBinding{d.Name, start, end, d})
		if d.BodyOpen < 0 || d.BodyClose < 0 {
			continue
		}
		bodyStart, bodyEnd := tokens[d.BodyOpen].StartByte, tokens[d.BodyClose].EndByte
		lo, hi := b.position(d.Start), b.position(d.BodyOpen)
		// Variable functions start with their initializer. Do not treat the name
		// to the left of '=' as a parameter of an unparenthesized arrow.
		if d.Kind == "arrow_function" || d.Kind == "function-expression" {
			for p := lo; p < hi; p++ {
				if tokens[b.sig[p]].Text == "=" {
					lo = p + 1
					break
				}
			}
		}
		for p := lo; p < hi; p++ {
			raw := b.sig[p]
			if tokens[raw].Text == "(" {
				if close, ok := b.pairs[raw]; ok && close < d.BodyOpen {
					b.patternList(p+1, b.position(close), bodyStart, bodyEnd)
					break
				}
			}
			if p+1 < hi && tokens[b.sig[p+1]].Text == "=>" {
				b.pattern(p, p+1, bodyStart, bodyEnd)
				break
			}
		}
	}
	for p, raw := range b.sig {
		text := tokens[raw].Text
		if text == "let" || text == "const" || text == "var" {
			start, end := b.scopeAt(tokens[raw].StartByte, text == "var")
			cursor := p + 1
			for cursor < len(b.sig) {
				after := b.pattern(cursor, len(b.sig), start, end)
				if after == cursor {
					break
				}
				// Skip an optional type and initializer, keeping only top-level commas.
				for after < len(b.sig) {
					tok := tokens[b.sig[after]]
					if tok.Text == ";" || tok.Text == "}" {
						break
					}
					if tok.Text == "," {
						break
					}
					if after > cursor && tok.StartLine > tokens[b.sig[after-1]].EndLine {
						break
					}
					if close, ok := b.pairs[b.sig[after]]; ok {
						after = b.position(close) + 1
					} else {
						after++
					}
				}
				if after >= len(b.sig) || tokens[b.sig[after]].Text != "," {
					break
				}
				cursor = after + 1
			}
		}
		if text == "catch" && p+1 < len(b.sig) && tokens[b.sig[p+1]].Text == "(" {
			close, ok := b.pairs[b.sig[p+1]]
			if !ok {
				continue
			}
			after := b.position(close) + 1
			if after < len(b.sig) && tokens[b.sig[after]].Text == "{" {
				end, ok := b.pairs[b.sig[after]]
				if ok {
					b.patternList(p+2, b.position(close), tokens[b.sig[after]].StartByte, tokens[end].EndByte)
				}
			}
		}
	}
	return b
}

func callableDeclaration(d *declaration) bool {
	switch d.Kind {
	case "function", "function-expression", "arrow_function", "method", "test":
		return true
	}
	return false
}
func (b bindingIndex) position(raw int) int {
	for p, r := range b.sig {
		if r == raw {
			return p
		}
	}
	return len(b.sig)
}
func (b bindingIndex) scopeAt(offset int, functionOnly bool) (int, int) {
	start, end := -1, int(^uint(0)>>1)
	if functionOnly {
		for _, d := range b.declarations {
			if callableDeclaration(d) && d.BodyOpen >= 0 && d.BodyClose >= 0 {
				a, z := b.tokens[d.BodyOpen].StartByte, b.tokens[d.BodyClose].EndByte
				if a < offset && offset < z && a > start {
					start, end = a, z
				}
			}
		}
		return start, end
	}
	for open, close := range b.pairs {
		if b.tokens[open].Text != "{" {
			continue
		}
		a, z := b.tokens[open].StartByte, b.tokens[close].EndByte
		if a < offset && offset < z && a > start {
			start, end = a, z
		}
	}
	return start, end
}
func (b *bindingIndex) patternList(lo, hi, start, end int) {
	for lo < hi {
		next := b.pattern(lo, hi, start, end)
		if next == lo {
			next++
		}
		for next < hi && b.tokens[b.sig[next]].Text != "," {
			if close, ok := b.pairs[b.sig[next]]; ok {
				next = b.position(close) + 1
			} else {
				next++
			}
		}
		lo = next + 1
	}
}
func (b *bindingIndex) pattern(lo, hi, start, end int) int {
	for lo < hi && (b.tokens[b.sig[lo]].Text == "..." || b.tokens[b.sig[lo]].Text == "." || b.tokens[b.sig[lo]].Text == "public" || b.tokens[b.sig[lo]].Text == "private" || b.tokens[b.sig[lo]].Text == "readonly") {
		lo++
	}
	if lo >= hi {
		return lo
	}
	raw := b.sig[lo]
	token := b.tokens[raw]
	if token.Text == "{" || token.Text == "[" {
		close, ok := b.pairs[raw]
		if !ok {
			return lo
		}
		stop := b.position(close)
		for p := lo + 1; p < stop; {
			if token.Text == "{" && p+1 < stop && b.tokens[b.sig[p+1]].Text == ":" {
				p += 2
			}
			next := b.pattern(p, stop, start, end)
			if next == p {
				next++
			}
			for next < stop && b.tokens[b.sig[next]].Text != "," {
				if z, ok := b.pairs[b.sig[next]]; ok {
					next = b.position(z) + 1
				} else {
					next++
				}
			}
			p = next + 1
		}
		return stop + 1
	}
	if token.Kind != Identifier {
		return lo
	}
	var known *declaration
	for _, d := range b.declarations {
		if callableDeclaration(d) && d.Name == token.Text && b.position(d.Start)+1 == lo && (d.Kind == "arrow_function" || d.Kind == "function-expression") {
			known = d
			break
		}
	}
	b.entries = append(b.entries, lexicalBinding{token.Text, start, end, known})
	b.names[token.StartByte] = true
	return lo + 1
}
func (b bindingIndex) lookup(offset int, name string) (*declaration, bool) {
	var found *lexicalBinding
	for i := range b.entries {
		v := &b.entries[i]
		if v.name != name || offset < v.start || offset >= v.end {
			continue
		}
		if found == nil || v.start > found.start || v.start == found.start && v.end < found.end {
			found = v
		} else if v.start == found.start && v.end == found.end && v.declaration == nil {
			found = v
		}
	}
	if found == nil {
		return nil, false
	}
	return found.declaration, true
}
func (b bindingIndex) ownsCall(d *declaration, offset int) bool {
	for _, child := range b.declarations {
		if child == d || !callableDeclaration(child) || child.Start < 0 {
			continue
		}
		a := b.tokens[child.Start].StartByte
		if child.Parent == d && a <= offset && offset < child.End {
			return false
		}
	}
	return true
}
func (b bindingIndex) callBinding(offset int, name, qualified string) (*declaration, bool) {
	if dot := strings.IndexByte(qualified, '.'); dot >= 0 {
		root := qualified[:dot]
		if root != "this" && root != "super" {
			_, bound := b.lookup(offset, root)
			if bound {
				return nil, true
			}
		}
		return nil, false
	}
	return b.lookup(offset, name)
}
