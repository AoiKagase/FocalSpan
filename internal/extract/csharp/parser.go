package csharp

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type declaration struct {
	Kind         string
	Name         string
	Qualified    string
	Start        int
	HeaderEnd    int
	End          int
	BodyOpen     int
	BodyClose    int
	Parent       *declaration
	Namespace    string
	SignatureKey string
}

type importSpec struct{ Target, Source string }

type parseResult struct {
	Tokens       []Token
	Significant  []int
	Declarations []*declaration
	Imports      []importSpec
	Diagnostics  []model.Diagnostic
}

type parser struct {
	ctx      context.Context
	tokens   []Token
	sig      []int
	matching map[int]int
	result   parseResult
	seen     map[int]bool
}

func parseCSharp(ctx context.Context, tokens []Token) (parseResult, error) {
	p := &parser{ctx: ctx, tokens: tokens, matching: make(map[int]int), seen: make(map[int]bool)}
	p.result.Tokens = tokens
	for index, token := range tokens {
		if token.significant() {
			p.sig = append(p.sig, index)
		}
	}
	p.result.Significant = p.sig
	p.buildMatching()
	p.parseImports()
	if err := p.parseScope(0, len(p.sig), nil, "", ""); err != nil {
		return parseResult{}, err
	}
	sort.SliceStable(p.result.Declarations, func(i, j int) bool { return p.result.Declarations[i].Start < p.result.Declarations[j].Start })
	return p.result, nil
}

func (p *parser) buildMatching() {
	opens := map[string]string{"(": ")", "{": "}", "[": "]"}
	stack := make([]int, 0)
	for _, raw := range p.sig {
		text := p.tokens[raw].Text
		if _, ok := opens[text]; ok {
			stack = append(stack, raw)
			continue
		}
		if text != ")" && text != "}" && text != "]" {
			continue
		}
		for len(stack) > 0 {
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if opens[p.tokens[open].Text] == text {
				p.matching[open], p.matching[raw] = raw, open
				break
			}
		}
	}
	if len(stack) > 0 {
		p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{Level: "warning", Code: "csharp_unbalanced_scope", Message: "one or more delimiters are not balanced"})
	}
}

func (p *parser) parseImports() {
	for position := 0; position < len(p.sig); position++ {
		start := position
		if p.tokens[p.sig[position]].Text == "global" && position+1 < len(p.sig) && p.tokens[p.sig[position+1]].Text == "using" {
			position++
		} else if p.tokens[p.sig[position]].Text != "using" {
			continue
		}
		semi := p.findUntil(position+1, ";")
		if semi < 0 {
			continue
		}
		line := joinTokens(p.tokens, p.sig[start:semi+1])
		target := strings.TrimSpace(strings.TrimSuffix(line, ";"))
		target = strings.TrimSpace(strings.TrimPrefix(target, "global"))
		target = strings.TrimSpace(strings.TrimPrefix(target, "using"))
		if equal := strings.Index(target, "="); equal >= 0 {
			target = strings.TrimSpace(target[equal+1:])
		}
		if target != "" {
			p.result.Imports = append(p.result.Imports, importSpec{Target: target, Source: "csharp:using"})
		}
		position = semi
	}
}

func (p *parser) parseScope(lo, hi int, parent *declaration, namespace, currentType string) error {
	for position := lo; position < hi; position++ {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		text := p.tokens[p.sig[position]].Text
		if text == "namespace" {
			if decl, close, ok := p.parseNamespace(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, decl.Qualified, ""); err != nil {
						return err
					}
				} else {
					if err := p.parseScope(position+1, hi, decl, decl.Qualified, ""); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if isTypeKeyword(text) {
			if decl, close, ok := p.parseType(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, decl.Name); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if typePosition := p.typeKeywordAhead(position, hi); typePosition >= 0 {
			if decl, close, ok := p.parseType(typePosition, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, decl.Name); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if text == "event" {
			if decl, close, ok := p.parseEvent(position, hi, parent, namespace); ok {
				p.add(decl)
				position = close
				continue
			}
		}
		if decl, close, ok := p.parseProperty(position, hi, parent, namespace); ok {
			p.add(decl)
			position = close
			continue
		}
		if decl, close, ok := p.parseFunction(position, hi, parent, namespace, currentType); ok {
			p.add(decl)
			if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
				if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, currentType); err != nil {
					return err
				}
			}
			position = close
		}
	}
	return nil
}

func (p *parser) parseNamespace(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	end := position + 1
	for end < hi && (p.tokens[p.sig[end]].Kind == Identifier || p.tokens[p.sig[end]].Text == ".") {
		end++
	}
	name := strings.ReplaceAll(strings.ReplaceAll(joinTokens(p.tokens, p.sig[position+1:end]), " . ", "."), " ", "")
	if name == "" {
		return nil, position, false
	}
	qualified := joinQualified(namespace, name)
	if end < hi && p.tokens[p.sig[end]].Text == ";" {
		semi := p.sig[end]
		return &declaration{Kind: "namespace", Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: p.tokens[semi].EndByte, End: p.tokens[semi].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, end, true
	}
	if end >= hi || p.tokens[p.sig[end]].Text != "{" {
		return nil, position, false
	}
	close := p.matching[p.sig[end]]
	return &declaration{Kind: "namespace", Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: p.tokens[p.sig[end]].EndByte, End: p.endAfter(close, hi), BodyOpen: p.sig[end], BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, end), true
}

func (p *parser) parseType(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	kind := p.tokens[p.sig[position]].Text
	if kind == "record" {
		kind = "record"
		if position+1 < hi && (p.tokens[p.sig[position+1]].Text == "class" || p.tokens[p.sig[position+1]].Text == "struct") {
			position++
		}
	}
	namePos := position + 1
	for namePos < hi && (p.tokens[p.sig[namePos]].Text == "class" || p.tokens[p.sig[namePos]].Text == "struct") {
		namePos++
	}
	if namePos >= hi || p.tokens[p.sig[namePos]].Kind != Identifier {
		return nil, position, false
	}
	name := p.tokens[p.sig[namePos]].Text
	open := p.findUntil(namePos+1, "{")
	semi := p.findUntil(namePos+1, ";")
	if open < 0 || semi >= 0 && semi < open {
		if semi < 0 {
			return nil, position, false
		}
		return &declaration{Kind: kind, Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[semi]].EndByte, End: p.tokens[p.sig[semi]].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, semi, true
	}
	close := p.matching[p.sig[open]]
	return &declaration{Kind: kind, Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[open]].EndByte, End: p.endAfter(close, hi), BodyOpen: p.sig[open], BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, open), true
}

func (p *parser) parseFunction(position, hi int, parent *declaration, namespace, currentType string) (*declaration, int, bool) {
	openPosition := -1
	for cursor := position; cursor < hi && cursor < position+80; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == ";" || text == "{" || text == "}" {
			break
		}
		if text == "(" {
			openPosition = cursor
			break
		}
	}
	if openPosition < 1 {
		return nil, position, false
	}
	closeRaw := p.matching[p.sig[openPosition]]
	if closeRaw == 0 {
		return nil, position, false
	}
	closePosition := p.positionOf(closeRaw)
	namePosition := openPosition - 1
	name := p.tokens[p.sig[namePosition]].Text
	if name == "=" || p.tokens[p.sig[namePosition]].Kind == Operator {
		for cursor := namePosition - 1; cursor >= position; cursor-- {
			if p.tokens[p.sig[cursor]].Text == "operator" {
				name = "operator " + p.tokens[p.sig[namePosition]].Text
				break
			}
		}
	}
	if name == "this" {
		name = "this"
	}
	if controlWords[name] || name == "if" || name == "for" || name == "while" || name == "switch" || name == "catch" {
		return nil, position, false
	}
	if !p.functionLikely(position, namePosition, currentType, name) {
		return nil, position, false
	}
	qualified := name
	if currentType != "" {
		qualified = currentType + "." + name
	}
	if namespace != "" {
		qualified = namespace + "." + qualified
	}
	kind := "method"
	if currentType == "" && parent == nil {
		kind = "function"
	}
	if name == currentType {
		kind = "constructor"
	}
	if strings.HasPrefix(name, "~") {
		kind = "destructor"
	}
	if strings.HasPrefix(name, "operator ") {
		kind = "operator"
	}
	if p.isTestAt(position) {
		kind = "test"
	}
	headerEnd := p.tokens[closeRaw].EndByte
	endPosition := closePosition
	bodyOpen, bodyClose := -1, -1
	for cursor := closePosition + 1; cursor < hi && cursor < closePosition+80; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "{" {
			bodyOpen = p.sig[cursor]
			bodyClose = p.matching[bodyOpen]
			endPosition = p.positionOfOr(bodyClose, cursor)
			headerEnd = p.tokens[bodyOpen].EndByte
			break
		}
		if text == ";" {
			endPosition = cursor
			headerEnd = p.tokens[p.sig[cursor]].EndByte
			break
		}
		if text == "=>" {
			semi := p.findUntil(cursor+1, ";")
			if semi >= 0 {
				endPosition = semi
				headerEnd = p.tokens[p.sig[semi]].EndByte
			}
			break
		}
	}
	if endPosition <= closePosition && bodyOpen < 0 {
		return nil, position, false
	}
	return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: headerEnd, End: p.endAfter(p.sig[endPosition], hi), BodyOpen: bodyOpen, BodyClose: bodyClose, Parent: parent, Namespace: namespace, SignatureKey: joinTokens(p.tokens, p.sig[openPosition+1:closePosition])}, endPosition, true
}

func (p *parser) parseProperty(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	if position+1 >= hi || p.tokens[p.sig[position]].Kind != Identifier {
		return nil, position, false
	}
	if p.tokens[p.sig[position+1]].Text == "[" && p.matching[p.sig[position+1]] > 0 {
		closeBracket := p.matching[p.sig[position+1]]
		closePosition := p.positionOf(closeBracket)
		if closePosition+1 < hi && p.tokens[p.sig[closePosition+1]].Text == "{" {
			open := p.sig[closePosition+1]
			close := p.matching[open]
			if close > 0 && p.rangeContainsProperty(close) {
				return &declaration{Kind: "property", Name: "this[]", Qualified: memberQualified(parent, namespace, "this[]"), Start: p.sig[position], HeaderEnd: p.tokens[open].EndByte, End: p.endAfter(close, hi), BodyOpen: open, BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, closePosition+1), true
			}
		}
	}
	if p.tokens[p.sig[position+1]].Text == "=>" {
		semi := p.findUntil(position+2, ";")
		if semi < 0 {
			return nil, position, false
		}
		return &declaration{Kind: "property", Name: p.tokens[p.sig[position]].Text, Qualified: memberQualified(parent, namespace, p.tokens[p.sig[position]].Text), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[semi]].EndByte, End: p.tokens[p.sig[semi]].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, semi, true
	}
	if p.tokens[p.sig[position+1]].Text != "{" {
		return nil, position, false
	}
	open := p.sig[position+1]
	close := p.matching[open]
	if close == 0 || !p.rangeContainsProperty(close) {
		return nil, position, false
	}
	return &declaration{Kind: "property", Name: p.tokens[p.sig[position]].Text, Qualified: memberQualified(parent, namespace, p.tokens[p.sig[position]].Text), Start: p.sig[position], HeaderEnd: p.tokens[open].EndByte, End: p.endAfter(close, hi), BodyOpen: open, BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, position+1), true
}

func (p *parser) parseEvent(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	name := ""
	namePos := position + 1
	for namePos < hi && p.tokens[p.sig[namePos]].Text != ";" && p.tokens[p.sig[namePos]].Text != "{" {
		if p.tokens[p.sig[namePos]].Kind == Identifier {
			name = p.tokens[p.sig[namePos]].Text
		}
		namePos++
	}
	if name == "" || namePos >= hi {
		return nil, position, false
	}
	end := p.sig[namePos]
	if p.tokens[end].Text == "{" {
		end = p.matching[end]
	}
	return &declaration{Kind: "event", Name: name, Qualified: memberQualified(parent, namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[namePos]].EndByte, End: p.endAfter(end, hi), BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, p.positionOfOr(end, namePos), true
}

func (p *parser) functionLikely(start, namePosition int, currentType, name string) bool {
	if currentType != "" && (name == currentType || name == "~"+currentType || strings.HasPrefix(name, "operator ")) {
		return true
	}
	if namePosition <= start {
		return false
	}
	for cursor := start; cursor < namePosition; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if typeWords[text] || text == "async" || text == "static" || text == "public" || text == "private" || text == "protected" || text == "internal" || text == "virtual" || text == "override" || text == "extern" || text == "partial" || text == "new" {
			return true
		}
	}
	return false
}

func (p *parser) isTestAt(position int) bool {
	if position >= 0 && position < len(p.sig) && p.tokens[p.sig[position]].Kind == Attribute {
		lower := strings.ToLower(p.tokens[p.sig[position]].Text)
		if strings.Contains(lower, "fact") || strings.Contains(lower, "theory") || strings.Contains(lower, "test") || strings.Contains(lower, "testcase") {
			return true
		}
	}
	for cursor := position - 1; cursor >= 0 && cursor >= position-8; cursor-- {
		text := p.tokens[p.sig[cursor]].Text
		if p.tokens[p.sig[cursor]].Kind == Attribute {
			lower := strings.ToLower(text)
			if strings.Contains(lower, "fact") || strings.Contains(lower, "theory") || strings.Contains(lower, "test") || strings.Contains(lower, "testcase") {
				return true
			}
		}
		if text == "}" || text == ";" {
			break
		}
	}
	return strings.Contains(strings.ToLower(currentTypeName(p, position)), "test")
}

func currentTypeName(p *parser, position int) string {
	for cursor := position - 1; cursor >= 0 && cursor >= position-20; cursor-- {
		if p.tokens[p.sig[cursor]].Text == "class" && cursor+1 < len(p.sig) {
			return p.tokens[p.sig[cursor+1]].Text
		}
	}
	return ""
}

func (p *parser) rangeContainsProperty(closeRaw int) bool {
	open := p.matching[closeRaw]
	if open == 0 {
		return false
	}
	for _, raw := range p.sig {
		if raw > open && raw < closeRaw && (p.tokens[raw].Text == "get" || p.tokens[raw].Text == "set" || p.tokens[raw].Text == "init" || p.tokens[raw].Text == "=>") {
			return true
		}
	}
	return false
}

func (p *parser) add(decl *declaration) {
	if decl == nil || decl.Start < 0 || p.seen[decl.Start] {
		return
	}
	p.seen[decl.Start] = true
	p.result.Declarations = append(p.result.Declarations, decl)
}

func (p *parser) findUntil(start int, wanted string) int {
	for position := start; position < len(p.sig) && position < start+160; position++ {
		text := p.tokens[p.sig[position]].Text
		if text == wanted {
			return position
		}
		if text == "{" || text == "}" {
			return -1
		}
	}
	return -1
}

func (p *parser) typeKeywordAhead(position, hi int) int {
	for cursor := position; cursor < hi && cursor < position+8; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if isTypeKeyword(text) {
			return cursor
		}
		if text == ";" || text == "{" || text == "(" {
			return -1
		}
	}
	return -1
}

func (p *parser) positionOf(raw int) int {
	for position, value := range p.sig {
		if value == raw {
			return position
		}
	}
	return -1
}

func (p *parser) positionOfOr(raw, fallback int) int {
	if raw > 0 {
		if position := p.positionOf(raw); position >= 0 {
			return position
		}
	}
	return fallback
}

func (p *parser) endAfter(raw, hi int) int {
	if raw <= 0 {
		if hi <= 0 {
			return 0
		}
		raw = p.sig[hi-1]
	}
	return p.tokens[raw].EndByte
}

func isTypeKeyword(text string) bool {
	return text == "class" || text == "record" || text == "struct" || text == "interface" || text == "enum" || text == "delegate"
}

var controlWords = map[string]bool{"if": true, "for": true, "foreach": true, "while": true, "switch": true, "catch": true, "lock": true, "using": true, "nameof": true, "sizeof": true, "typeof": true, "return": true, "throw": true, "when": true}
var typeWords = map[string]bool{"bool": true, "byte": true, "char": true, "decimal": true, "double": true, "float": true, "int": true, "long": true, "object": true, "sbyte": true, "short": true, "string": true, "uint": true, "ulong": true, "ushort": true, "void": true, "var": true, "Task": true, "ValueTask": true}

func joinTokens(tokens []Token, indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		parts = append(parts, tokens[index].Text)
	}
	return strings.Join(parts, " ")
}

func joinQualified(namespace, name string) string {
	name = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(name), " . ", "."), " ", "")
	namespace = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(namespace), " . ", "."), " ", "")
	name = strings.Trim(name, ".")
	if namespace == "" {
		return name
	}
	return strings.Trim(namespace, ".") + "." + name
}

func memberQualified(parent *declaration, namespace, name string) string {
	if parent != nil && parent.Kind != "namespace" {
		return joinQualified(parent.Qualified, name)
	}
	return joinQualified(namespace, name)
}

func normalizeImport(target string) string {
	target = strings.Trim(target, " \t\"")
	target = strings.ReplaceAll(target, " . ", ".")
	target = strings.ReplaceAll(target, " ", "")
	if strings.HasPrefix(target, ".") {
		return path.Clean(target)
	}
	return target
}
