package php

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type phpDecl struct {
	Kind       string
	Name       string
	Namespace  string
	Start      int
	End        int
	HeaderEnd  int
	BodyOpen   int
	BodyClose  int
	Parent     *phpDecl
	Modifiers  []string
	Attributes []string
	Doc        string
	Extends    []string
	Implements []string
	Recovered  bool
}

type phpUse struct {
	Start     int
	End       int
	Namespace string
}

type parseResult struct {
	Tokens       []Token
	Significant  []int
	Declarations []*phpDecl
	Uses         []phpUse
	Diagnostics  []model.Diagnostic
}

type phpParser struct {
	ctx    context.Context
	file   model.SourceFile
	tokens []Token
	sig    []int
	result parseResult
}

func parsePHP(ctx context.Context, file model.SourceFile, tokens []Token, diagnostics []model.Diagnostic) (parseResult, error) {
	if err := ctx.Err(); err != nil {
		return parseResult{}, err
	}
	p := &phpParser{ctx: ctx, file: file, tokens: tokens, result: parseResult{Tokens: tokens, Diagnostics: append([]model.Diagnostic(nil), diagnostics...)}}
	for index, token := range tokens {
		if token.Kind != KindWhitespace && token.Kind != KindLineComment && token.Kind != KindBlockComment && token.Kind != KindDocComment {
			p.sig = append(p.sig, index)
		}
	}
	p.result.Significant = p.sig
	if err := p.parseScope(0, len(p.sig), "", nil, false); err != nil {
		return parseResult{}, err
	}
	return p.result, nil
}

func (p *phpParser) parseScope(start, end int, namespace string, parent *phpDecl, classScope bool) error {
	currentNamespace := namespace
	for position := start; position < end; {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if position >= end {
			break
		}
		raw := p.sig[position]
		text := strings.ToLower(p.tokens[raw].Text)
		switch text {
		case "namespace":
			nameEnd, delimiter := p.findStatementDelimiter(position+1, end)
			name := canonicalName(p.joinNameTokens(position+1, nameEnd))
			if name == "" {
				p.addDiagnostic("php_malformed_declaration", "PHP namespace declaration has no name", raw)
				position++
				continue
			}
			if delimiter >= 0 && p.tokens[p.sig[delimiter]].Text == "{" {
				close := p.matchDelimiter(delimiter, end, "{", "}")
				decl := &phpDecl{Kind: "namespace", Name: name, Namespace: name, Start: p.declarationStart(position, start), End: p.endAfterDelimiter(close, delimiter, end), HeaderEnd: p.sig[delimiter] + 1, BodyOpen: p.sig[delimiter], BodyClose: closeRaw(p.sig, close)}
				p.result.Declarations = append(p.result.Declarations, decl)
				if close < 0 {
					decl.End = len(p.tokens)
					decl.Recovered = true
					p.addDiagnostic("php_unbalanced_brace", "PHP namespace brace is not closed", raw)
					return p.parseScope(delimiter+1, end, name, parent, classScope)
				}
				p.result.Declarations = p.result.Declarations[:len(p.result.Declarations)-1]
				if err := p.parseScope(delimiter+1, close, name, decl, classScope); err != nil {
					return err
				}
				p.result.Declarations = append(p.result.Declarations, decl)
				position = close + 1
				continue
			}
			if delimiter >= 0 {
				decl := &phpDecl{Kind: "namespace", Name: name, Namespace: name, Start: p.declarationStart(position, start), End: p.sig[delimiter] + 1, HeaderEnd: p.sig[delimiter] + 1}
				p.result.Declarations = append(p.result.Declarations, decl)
				currentNamespace = name
				position = delimiter + 1
				continue
			}
			p.addDiagnostic("php_malformed_declaration", "PHP namespace declaration has no terminator", raw)
			position++
			continue
		case "use":
			if !classScope {
				if delimiter := p.findSemicolon(position+1, end); delimiter >= 0 {
					p.result.Uses = append(p.result.Uses, phpUse{Start: p.declarationStart(position, start), End: p.sig[delimiter] + 1, Namespace: currentNamespace})
					position = delimiter + 1
					continue
				}
				p.addDiagnostic("php_malformed_declaration", "PHP use declaration has no terminator", raw)
			}
		case "class", "interface", "trait", "enum":
			if next := position + 1; next < end && p.isIdentifier(p.sig[next]) {
				decl, nextPosition := p.parseClassLike(position, end, currentNamespace, parent)
				p.result.Declarations = append(p.result.Declarations, decl)
				if decl.BodyOpen >= 0 && decl.BodyClose >= 0 {
					if err := p.parseScope(positionAfterRaw(p.sig, decl.BodyOpen)+1, positionAfterRaw(p.sig, decl.BodyClose), currentNamespace, decl, true); err != nil {
						return err
					}
				}
				position = nextPosition
				continue
			}
			p.addDiagnostic("php_malformed_declaration", "PHP class-like declaration has no name", raw)
		case "function":
			if next := position + 1; next < end && p.tokens[p.sig[next]].Text == "&" {
				next++
				if next >= end || !p.isIdentifier(p.sig[next]) {
					p.addDiagnostic("php_malformed_declaration", "PHP function declaration has no name", raw)
					position++
					continue
				}
			}
			if next := p.functionNamePosition(position, end); next >= 0 {
				decl, nextPosition := p.parseFunction(position, end, currentNamespace, parent)
				p.result.Declarations = append(p.result.Declarations, decl)
				if parent != nil && parent.Kind != "namespace" {
					p.appendPromotedProperties(decl, end)
				}
				position = nextPosition
				continue
			}
		}
		if classScope && (text == "const" || p.isPropertyCandidate(position, end)) {
			if added, nextPosition := p.parseClassMembers(position, end, currentNamespace, parent); added {
				position = nextPosition
				continue
			}
		}
		position++
	}
	return nil
}

func (p *phpParser) parseClassLike(position, end int, namespace string, parent *phpDecl) (*phpDecl, int) {
	kind := strings.ToLower(p.tokens[p.sig[position]].Text)
	name := p.tokens[p.sig[position+1]].Text
	decl := &phpDecl{Kind: kind, Name: name, Namespace: namespace, Start: p.declarationStart(position, 0), BodyOpen: -1, BodyClose: -1, Parent: parent}
	decl.Start = p.declarationStart(position, 0)
	decl.Modifiers, decl.Attributes, decl.Doc = p.prefixMetadata(position)
	open := -1
	semi := -1
	for cursor := position + 2; cursor < end; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "{" {
			open = cursor
			break
		}
		if text == ";" {
			semi = cursor
			break
		}
		if strings.EqualFold(text, "extends") {
			if name := p.nameAfter(cursor+1, end, map[string]bool{"implements": true, "{": true, ";": true}); name != "" {
				decl.Extends = append(decl.Extends, name)
			}
		}
		if strings.EqualFold(text, "implements") {
			decl.Implements = append(decl.Implements, p.namesUntil(cursor+1, end, map[string]bool{"{": true, ";": true})...)
		}
	}
	if open >= 0 {
		close := p.matchDelimiter(open, end, "{", "}")
		decl.BodyOpen = p.sig[open]
		decl.BodyClose = closeRaw(p.sig, close)
		if close < 0 {
			decl.End = len(p.tokens)
			decl.HeaderEnd = p.sig[open] + 1
			decl.Recovered = true
			p.addDiagnostic("php_unbalanced_brace", "PHP class-like declaration brace is not closed", p.sig[open])
			return decl, end
		}
		decl.End = p.sig[close] + 1
		decl.HeaderEnd = p.sig[open] + 1
		return decl, close + 1
	}
	if semi >= 0 {
		decl.End = p.sig[semi] + 1
		decl.HeaderEnd = decl.End
		return decl, semi + 1
	}
	decl.End = len(p.tokens)
	decl.HeaderEnd = len(p.tokens)
	decl.Recovered = true
	p.addDiagnostic("php_malformed_declaration", "PHP class-like declaration has no body or terminator", p.sig[position])
	return decl, end
}

func (p *phpParser) parseFunction(position, end int, namespace string, parent *phpDecl) (*phpDecl, int) {
	namePosition := p.functionNamePosition(position, end)
	name := p.tokens[p.sig[namePosition]].Text
	decl := &phpDecl{Kind: "function", Name: name, Namespace: namespace, Start: p.declarationStart(position, 0), BodyOpen: -1, BodyClose: -1, Parent: parent}
	decl.Modifiers, decl.Attributes, decl.Doc = p.prefixMetadata(position)
	openParen := -1
	for cursor := namePosition + 1; cursor < end; cursor++ {
		if p.tokens[p.sig[cursor]].Text == "(" {
			openParen = cursor
			break
		}
	}
	if openParen < 0 {
		decl.End = p.sig[namePosition] + 1
		decl.HeaderEnd = decl.End
		decl.Recovered = true
		p.addDiagnostic("php_malformed_declaration", "PHP function declaration has no parameter list", p.sig[position])
		return decl, namePosition + 1
	}
	closeParen := p.matchDelimiter(openParen, end, "(", ")")
	if closeParen < 0 {
		decl.End = len(p.tokens)
		decl.HeaderEnd = len(p.tokens)
		decl.Recovered = true
		p.addDiagnostic("php_unbalanced_brace", "PHP function parameter list is not closed", p.sig[openParen])
		return decl, end
	}
	body := -1
	semi := -1
	for cursor := closeParen + 1; cursor < end; cursor++ {
		switch p.tokens[p.sig[cursor]].Text {
		case "{":
			body = cursor
			cursor = end
		case ";":
			semi = cursor
			cursor = end
		}
	}
	if body >= 0 {
		close := p.matchDelimiter(body, end, "{", "}")
		decl.BodyOpen = p.sig[body]
		decl.HeaderEnd = p.sig[body] + 1
		if close < 0 {
			decl.End = len(p.tokens)
			decl.Recovered = true
			p.addDiagnostic("php_unbalanced_brace", "PHP function body is not closed", p.sig[body])
			return decl, end
		}
		decl.BodyClose = p.sig[close]
		decl.End = p.sig[close] + 1
		return decl, close + 1
	}
	if semi >= 0 {
		decl.End = p.sig[semi] + 1
		decl.HeaderEnd = decl.End
		return decl, semi + 1
	}
	decl.End = len(p.tokens)
	decl.HeaderEnd = len(p.tokens)
	decl.Recovered = true
	p.addDiagnostic("php_malformed_declaration", "PHP function declaration has no body or terminator", p.sig[position])
	return decl, end
}

func (p *phpParser) parseClassMembers(position, end int, namespace string, parent *phpDecl) (bool, int) {
	semi := p.findSemicolon(position, end)
	if semi < 0 {
		return false, position + 1
	}
	for cursor := position; cursor < semi; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "(" || text == "{" {
			return false, position + 1
		}
	}
	kind := "property"
	if strings.EqualFold(p.tokens[p.sig[position]].Text, "const") || p.containsKeyword(position, semi, "const") {
		kind = "constant"
	}
	if kind == "property" {
		if !p.containsVariable(position, semi) {
			return false, position + 1
		}
	} else if !p.containsIdentifierAfter(position+1, semi) {
		return false, position + 1
	}
	for cursor := position; cursor < semi; cursor++ {
		raw := p.sig[cursor]
		name := p.tokens[raw].Text
		if kind == "property" && p.tokens[raw].Kind != KindVariable {
			continue
		}
		if kind == "constant" && !p.isIdentifier(raw) {
			continue
		}
		if kind == "constant" && cursor > position && strings.EqualFold(p.tokens[p.sig[cursor-1]].Text, "const") == false && p.tokens[p.sig[cursor-1]].Text != "," {
			continue
		}
		if kind == "property" {
			name = strings.TrimPrefix(name, "$")
		}
		decl := &phpDecl{Kind: kind, Name: name, Namespace: namespace, Start: p.declarationStart(position, 0), End: p.sig[semi] + 1, HeaderEnd: p.sig[semi] + 1, Parent: parent}
		decl.Modifiers, decl.Attributes, decl.Doc = p.prefixMetadata(position)
		p.result.Declarations = append(p.result.Declarations, decl)
	}
	return true, semi + 1
}

func (p *phpParser) appendPromotedProperties(method *phpDecl, end int) {
	if method.Parent == nil || method.Parent.Kind == "namespace" {
		return
	}
	start, finish := rawToSigRange(p.sig, method.Start, method.HeaderEnd)
	for cursor := start; cursor < finish; cursor++ {
		if p.tokens[p.sig[cursor]].Kind != KindVariable {
			continue
		}
		if cursor == start || !hasModifierBefore(p.tokens, p.sig, cursor, start) {
			continue
		}
		name := strings.TrimPrefix(p.tokens[p.sig[cursor]].Text, "$")
		methodProperty := &phpDecl{Kind: "property", Name: name, Namespace: method.Namespace, Start: method.Start, End: method.HeaderEnd, HeaderEnd: method.HeaderEnd, Parent: method.Parent}
		p.result.Declarations = append(p.result.Declarations, methodProperty)
	}
}

func (p *phpParser) functionNamePosition(position, end int) int {
	if position+1 < end && p.tokens[p.sig[position+1]].Text == "&" {
		position++
	}
	if position+1 < end && p.isIdentifier(p.sig[position+1]) {
		return position + 1
	}
	return -1
}

func (p *phpParser) findStatementDelimiter(start, end int) (int, int) {
	for cursor := start; cursor < end; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == ";" || text == "{" {
			return cursor, cursor
		}
	}
	return -1, -1
}

func (p *phpParser) findSemicolon(start, end int) int {
	depth := 0
	for cursor := start; cursor < end; cursor++ {
		switch p.tokens[p.sig[cursor]].Text {
		case "(", "[":
			depth++
		case ")", "]":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				return cursor
			}
		case "{", "}":
			if depth == 0 {
				return -1
			}
		}
	}
	return -1
}

func (p *phpParser) matchDelimiter(open, end int, opening, closing string) int {
	depth := 0
	for cursor := open; cursor < end; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == opening {
			depth++
		} else if text == closing {
			depth--
			if depth == 0 {
				return cursor
			}
		}
	}
	return -1
}

func (p *phpParser) isPropertyCandidate(position, end int) bool {
	if position >= end || p.tokens[p.sig[position]].Kind == KindVariable {
		return true
	}
	return phpModifiers[strings.ToLower(p.tokens[p.sig[position]].Text)]
}

func (p *phpParser) containsVariable(start, end int) bool {
	for cursor := start; cursor < end; cursor++ {
		if p.tokens[p.sig[cursor]].Kind == KindVariable {
			return true
		}
	}
	return false
}

func (p *phpParser) containsIdentifierAfter(start, end int) bool {
	for cursor := start; cursor < end; cursor++ {
		if p.isIdentifier(p.sig[cursor]) {
			return true
		}
	}
	return false
}

func (p *phpParser) containsKeyword(start, end int, keyword string) bool {
	for cursor := start; cursor < end; cursor++ {
		if strings.EqualFold(p.tokens[p.sig[cursor]].Text, keyword) {
			return true
		}
	}
	return false
}

func (p *phpParser) isIdentifier(raw int) bool {
	return p.tokens[raw].Kind == KindIdentifier || p.tokens[raw].Kind == KindKeyword && !phpKeywords[strings.ToLower(p.tokens[raw].Text)]
}

func (p *phpParser) declarationStart(position, scopeStart int) int {
	if position <= scopeStart {
		return p.sig[position]
	}
	start := position
	for start > scopeStart {
		previous := p.tokens[p.sig[start-1]].Text
		if phpModifiers[strings.ToLower(previous)] {
			start--
			continue
		}
		if previous == "]" {
			open := start - 1
			depth := 0
			for open >= scopeStart {
				token := p.tokens[p.sig[open]].Text
				if token == "]" {
					depth++
				} else if token == "[" {
					depth--
					if depth == 0 {
						if open > scopeStart && p.tokens[p.sig[open-1]].Text == "#" {
							open--
						}
						start = open
						break
					}
				}
				open--
			}
			if start != position {
				continue
			}
		}
		break
	}
	if start > scopeStart {
		raw := p.sig[start]
		for raw > 0 && p.tokens[raw-1].Kind == KindWhitespace {
			raw--
		}
		if raw > 0 && p.tokens[raw-1].Kind == KindDocComment {
			return raw - 1
		}
	}
	return p.sig[start]
}

func (p *phpParser) prefixMetadata(position int) ([]string, []string, string) {
	keyword := p.sig[position]
	start := p.declarationStart(position, 0)
	modifiers := make([]string, 0)
	attributes := make([]string, 0)
	for cursor := start; cursor <= keyword && cursor < len(p.tokens); cursor++ {
		if phpModifiers[strings.ToLower(p.tokens[cursor].Text)] {
			modifiers = append(modifiers, p.tokens[cursor].Text)
		}
	}
	for cursor := start; cursor <= keyword && cursor < len(p.tokens); cursor++ {
		if p.tokens[cursor].Text == "#" {
			attributes = append(attributes, sourceText(p.tokens, cursor, keyword+1))
			break
		}
	}
	for cursor := start; cursor >= 0; cursor-- {
		if p.tokens[cursor].Kind == KindDocComment {
			return modifiers, attributes, p.tokens[cursor].Text
		}
		if p.tokens[cursor].Kind != KindWhitespace {
			break
		}
	}
	return modifiers, attributes, ""
}

func (p *phpParser) nameAfter(start, end int, stop map[string]bool) string {
	parts := make([]string, 0)
	for cursor := start; cursor < end; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if stop[strings.ToLower(text)] {
			break
		}
		if p.isIdentifier(p.sig[cursor]) || text == "\\" {
			parts = append(parts, text)
			continue
		}
		if len(parts) > 0 {
			break
		}
	}
	return canonicalName(strings.Join(parts, ""))
}

func (p *phpParser) namesUntil(start, end int, stop map[string]bool) []string {
	result := make([]string, 0)
	for cursor := start; cursor < end; {
		name := p.nameAfter(cursor, end, stop)
		if name == "" {
			break
		}
		result = append(result, name)
		for cursor < end && p.tokens[p.sig[cursor]].Text != "," && !stop[strings.ToLower(p.tokens[p.sig[cursor]].Text)] {
			cursor++
		}
		if cursor < end && p.tokens[p.sig[cursor]].Text == "," {
			cursor++
		}
	}
	return result
}

func (p *phpParser) joinNameTokens(start, end int) string {
	parts := make([]string, 0)
	for cursor := start; cursor < end; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if p.isIdentifier(p.sig[cursor]) || text == "\\" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

func (p *phpParser) prefixMetadataAtRaw(raw int) ([]string, []string, string) {
	_ = raw
	return nil, nil, ""
}

func (p *phpParser) addDiagnostic(code, message string, raw int) {
	line := 0
	if raw >= 0 && raw < len(p.tokens) {
		line = p.tokens[raw].StartLine
	}
	p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{FilePath: p.file.Path, Level: "warning", Code: code, Message: messageWithLine(message, line)})
}

var phpModifiers = map[string]bool{"public": true, "protected": true, "private": true, "static": true, "abstract": true, "final": true, "readonly": true, "var": true}

func canonicalName(value string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "/", "\\"), "\\")
}

func positionAfterRaw(sig []int, raw int) int {
	for position, value := range sig {
		if value == raw {
			return position
		}
	}
	return len(sig)
}

func closeRaw(sig []int, position int) int {
	if position < 0 || position >= len(sig) {
		return -1
	}
	return sig[position]
}

func (p *phpParser) endAfterDelimiter(position, delimiter, end int) int {
	if position >= 0 && position < end {
		return p.sig[position] + 1
	}
	if delimiter >= 0 && delimiter < end {
		return p.sig[delimiter] + 1
	}
	return len(p.tokens)
}

func rawToSigRange(sig []int, start, end int) (int, int) {
	first, last := 0, len(sig)
	for position, raw := range sig {
		if raw >= start {
			first = position
			break
		}
	}
	for position, raw := range sig {
		if raw >= end {
			last = position
			break
		}
	}
	return first, last
}

func hasModifierBefore(tokens []Token, sig []int, position, start int) bool {
	for cursor := position - 1; cursor >= 0 && sig[cursor] >= start; cursor-- {
		if phpModifiers[strings.ToLower(tokens[sig[cursor]].Text)] {
			return true
		}
		if tokens[sig[cursor]].Text == "," || tokens[sig[cursor]].Text == "(" {
			return false
		}
	}
	return false
}

func sourceText(tokens []Token, start, end int) string {
	if start < 0 || end <= start || start >= len(tokens) {
		return ""
	}
	if end > len(tokens) {
		end = len(tokens)
	}
	return tokens[start].Text + func() string {
		var b strings.Builder
		for _, token := range tokens[start+1 : end] {
			b.WriteString(token.Text)
		}
		return b.String()
	}()
}

func messageWithLine(message string, line int) string {
	if line <= 0 {
		return message
	}
	return message + " at line " + strconvItoa(line)
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}
