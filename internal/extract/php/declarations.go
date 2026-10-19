package php

// findUseEnd balances the braces in a group import, without consuming a
// following declaration when malformed source omits its closing delimiter.
func (p *phpParser) findUseEnd(start, end int) (int, int) {
	depth := 0
	for cursor := start; cursor < end; cursor++ {
		if p.ctx.Err() != nil {
			return -1, end
		}
		switch p.tokens[p.sig[cursor]].Text {
		case "{":
			depth++
		case "}":
			if depth == 0 {
				return -1, cursor
			}
			depth--
		case ";":
			if depth == 0 {
				return cursor, cursor + 1
			}
			return -1, cursor + 1
		case "class", "interface", "trait", "enum":
			return -1, cursor
		case "function":
			// A group element may say `function Name`, but a parameter list
			// begins a following declaration, not an import element.
			if cursor+2 < end && p.isIdentifier(p.sig[cursor+1]) && p.tokens[p.sig[cursor+2]].Text == "(" {
				return -1, cursor
			}
		}
	}
	return -1, end
}

func (p *phpParser) appendConstants(start, semi int, namespace string, parent *phpDecl) {
	segment := start
	depth := 0
	inValue := false
	for cursor := start; cursor < semi; cursor++ {
		if p.ctx.Err() != nil {
			return
		}
		text := p.tokens[p.sig[cursor]].Text
		switch text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ",":
			if depth == 0 {
				segment = cursor + 1
				inValue = false
			}
		case "=":
			if depth != 0 || inValue {
				continue
			}
			inValue = true
			if cursor <= segment || !p.isIdentifier(p.sig[cursor-1]) {
				continue
			}
			decl := &phpDecl{Kind: "constant", Name: p.tokens[p.sig[cursor-1]].Text, Namespace: namespace, Start: p.declarationStart(start, 0), End: p.sig[semi] + 1, HeaderEnd: p.sig[semi] + 1, Parent: parent}
			decl.Modifiers, decl.Attributes, decl.Doc = p.prefixMetadata(start)
			p.result.Declarations = append(p.result.Declarations, decl)
		}
	}
}
