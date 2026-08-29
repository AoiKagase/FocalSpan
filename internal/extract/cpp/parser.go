package cpp

import (
	"context"
	"path"
	"regexp"
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
	Recovered    bool
	SignatureKey string
}

type include struct {
	Target string
	Source string
}

type parseResult struct {
	Tokens       []Token
	Significant  []int
	Declarations []*declaration
	Includes     []include
	Modules      []include
	Diagnostics  []model.Diagnostic
}

type parser struct {
	ctx       context.Context
	tokens    []Token
	sig       []int
	matching  map[int]int
	result    parseResult
	seenStart map[int]bool
}

func parseCPP(ctx context.Context, tokens []Token) (parseResult, error) {
	p := &parser{ctx: ctx, tokens: tokens, matching: make(map[int]int), seenStart: make(map[int]bool)}
	p.result.Tokens = tokens
	for index, token := range tokens {
		if token.significant() {
			p.sig = append(p.sig, index)
		}
	}
	p.result.Significant = p.sig
	p.buildMatching()
	p.parseDirectives()
	if err := p.parseScope(0, len(p.sig), nil, "", ""); err != nil {
		return parseResult{}, err
	}
	sort.SliceStable(p.result.Declarations, func(i, j int) bool { return p.result.Declarations[i].Start < p.result.Declarations[j].Start })
	return p.result, nil
}

func (p *parser) buildMatching() {
	types := map[string]string{"(": ")", "{": "}", "[": "]"}
	stack := make([]int, 0)
	for _, raw := range p.sig {
		text := p.tokens[raw].Text
		if _, ok := types[text]; ok {
			stack = append(stack, raw)
			continue
		}
		if text != ")" && text != "}" && text != "]" {
			continue
		}
		for len(stack) > 0 {
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if types[p.tokens[open].Text] == text {
				p.matching[open] = raw
				p.matching[raw] = open
				break
			}
		}
	}
	if len(stack) > 0 {
		p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{Level: "warning", Code: "cpp_unbalanced_scope", Message: "one or more delimiters are not balanced"})
	}
}

func (p *parser) parseScope(lo, hi int, parent *declaration, namespace, currentType string) error {
	for position := lo; position < hi; position++ {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		raw := p.sig[position]
		text := p.tokens[raw].Text
		if text == "namespace" {
			decl, close, ok := p.parseNamespace(position, hi, parent, namespace)
			if ok {
				p.addDeclaration(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, decl.Qualified, ""); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if isTypeKeyword(text) {
			decl, close, ok := p.parseType(position, hi, parent, namespace)
			if ok {
				p.addDeclaration(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespaceFor(decl.Qualified), decl.Name); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if isTestMacro(text) {
			if decl, close, ok := p.parseTest(position, hi, parent, namespace); ok {
				p.addDeclaration(decl)
				position = close
				continue
			}
		}
		if text == "using" || text == "typedef" || text == "concept" {
			if decl, close, ok := p.parseAlias(position, hi, parent, namespace); ok {
				p.addDeclaration(decl)
				position = close
				continue
			}
		}
		if decl, close, ok := p.parseFunction(position, hi, parent, namespace, currentType); ok {
			p.addDeclaration(decl)
			position = close
		}
	}
	return nil
}

func (p *parser) parseNamespace(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	nameEnd := position + 1
	for nameEnd < hi && (p.tokens[p.sig[nameEnd]].Kind == Identifier || p.tokens[p.sig[nameEnd]].Text == "::" || p.tokens[p.sig[nameEnd]].Text == "inline") {
		nameEnd++
	}
	name := joinTokens(p.tokens, p.sig[position+1:nameEnd])
	if name == "" || name == "inline" {
		return nil, position, false
	}
	open := p.findToken(nameEnd, nameEnd, hi, "{")
	if open < 0 {
		return nil, position, false
	}
	close := p.matching[p.sig[open]]
	end := hi - 1
	if close > 0 {
		end = p.positionOf(close)
	}
	qualified := joinQualified(namespace, name)
	return &declaration{Kind: "namespace", Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: p.tokens[p.sig[open]].EndByte, End: p.endAfter(close, hi), BodyOpen: p.sig[open], BodyClose: close, Parent: parent, Namespace: namespace}, end, true
}

func (p *parser) parseType(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	kind := p.tokens[p.sig[position]].Text
	namePos := position + 1
	if kind == "enum" && namePos < hi && (p.tokens[p.sig[namePos]].Text == "class" || p.tokens[p.sig[namePos]].Text == "struct") {
		kind = "enum"
		namePos++
	}
	for namePos < hi && (p.tokens[p.sig[namePos]].Text == "final" || p.tokens[p.sig[namePos]].Text == "class") {
		namePos++
	}
	if namePos >= hi || p.tokens[p.sig[namePos]].Kind != Identifier {
		return nil, position, false
	}
	name := p.tokens[p.sig[namePos]].Text
	open := p.findToken(namePos+1, namePos+1, hi, "{")
	if open < 0 {
		semi := p.findToken(namePos+1, namePos+2, hi, ";")
		if semi < 0 {
			return nil, position, false
		}
		return &declaration{Kind: kind, Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[semi]].EndByte, End: p.tokens[p.sig[semi]].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, semi, true
	}
	close := p.matching[p.sig[open]]
	end := p.endAfter(close, hi)
	return &declaration{Kind: kind, Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[open]].EndByte, End: end, BodyOpen: p.sig[open], BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, end), true
}

func (p *parser) parseAlias(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	keyword := p.tokens[p.sig[position]].Text
	semi := p.findToken(position+1, position+2, hi, ";")
	if semi < 0 {
		return nil, position, false
	}
	name := ""
	kind := "alias"
	for cursor := position + 1; cursor < semi; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if keyword == "using" && text == "=" && cursor > position+1 {
			name = p.tokens[p.sig[cursor-1]].Text
			break
		}
		if keyword == "typedef" && p.tokens[p.sig[cursor]].Kind == Identifier {
			name = text
		}
		if keyword == "concept" && p.tokens[p.sig[cursor]].Kind == Identifier {
			name = text
			kind = "concept"
			break
		}
	}
	if name == "" {
		return nil, position, false
	}
	return &declaration{Kind: kind, Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[semi]].EndByte, End: p.tokens[p.sig[semi]].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, semi, true
}

func (p *parser) parseFunction(position, hi int, parent *declaration, namespace, currentType string) (*declaration, int, bool) {
	if p.seenStart[p.sig[position]] {
		return nil, position, false
	}
	openPosition := -1
	namePosition := -1
	for cursor := position; cursor < hi && cursor < position+80; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == ";" || text == "{" || text == "}" {
			break
		}
		if text == "(" {
			openPosition = cursor
			namePosition = cursor - 1
			break
		}
	}
	if openPosition < 1 || namePosition < 1 {
		return nil, position, false
	}
	openRaw := p.sig[openPosition]
	closeRaw := p.matching[openRaw]
	if closeRaw == 0 {
		return nil, position, false
	}
	closePosition := p.positionOf(closeRaw)
	if !p.functionHeaderLikely(position, namePosition, openPosition, currentType) {
		return nil, position, false
	}
	name, qualified := p.functionName(namePosition, position, namespace, currentType)
	if name == "" {
		return nil, position, false
	}
	endPosition := closePosition
	bodyOpen := -1
	bodyClose := -1
	headerEnd := p.tokens[closeRaw].EndByte
	for cursor := closePosition + 1; cursor < hi && cursor < closePosition+80; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "{" {
			bodyOpen = p.sig[cursor]
			bodyClose = p.matching[bodyOpen]
			if bodyClose > 0 {
				endPosition = p.positionOf(bodyClose)
			} else {
				endPosition = hi - 1
			}
			headerEnd = p.tokens[bodyOpen].EndByte
			break
		}
		if text == ";" {
			endPosition = cursor
			headerEnd = p.tokens[p.sig[cursor]].EndByte
			break
		}
		if text == "=" {
			if cursor+1 < hi && p.tokens[p.sig[cursor+1]].Text == "default" {
				semi := p.findToken(cursor+1, cursor+2, hi, ";")
				if semi >= 0 {
					endPosition = semi
					headerEnd = p.tokens[p.sig[semi]].EndByte
				}
			}
			break
		}
	}
	if endPosition <= closePosition && bodyOpen < 0 {
		return nil, position, false
	}
	kind := "function"
	if currentType != "" || strings.Contains(qualified, "::") {
		kind = "method"
	}
	if name == currentType || strings.HasPrefix(name, "~") {
		kind = "constructor"
		if strings.HasPrefix(name, "~") {
			kind = "destructor"
		}
	}
	if strings.HasPrefix(name, "operator") {
		kind = "operator"
	}
	if parent != nil && parent.Kind == "class" && (kind == "function" || kind == "method") {
		kind = "method"
	}
	decl := &declaration{Kind: kind, Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: headerEnd, End: p.endAfter(p.sig[endPosition], hi), BodyOpen: bodyOpen, BodyClose: bodyClose, Parent: parent, Namespace: namespace, SignatureKey: joinTokens(p.tokens, p.sig[openPosition+1:closePosition])}
	return decl, endPosition, true
}

func (p *parser) parseTest(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	open := position + 1
	if open >= hi || p.tokens[p.sig[open]].Text != "(" {
		return nil, position, false
	}
	closeRaw := p.matching[p.sig[open]]
	if closeRaw == 0 {
		return nil, position, false
	}
	close := p.positionOf(closeRaw)
	body := close + 1
	for body < hi && (p.tokens[p.sig[body]].Text == "->" || p.tokens[p.sig[body]].Text == "void") {
		body++
	}
	if body >= hi || p.tokens[p.sig[body]].Text != "{" {
		return nil, position, false
	}
	bodyClose := p.matching[p.sig[body]]
	args := strings.TrimSpace(joinTokens(p.tokens, p.sig[position+2:close]))
	name := testName(args)
	if name == "" {
		name = p.tokens[p.sig[position]].Text
	}
	return &declaration{Kind: "test", Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[body]].EndByte, End: p.endAfter(bodyClose, hi), BodyOpen: p.sig[body], BodyClose: bodyClose, Parent: parent, Namespace: namespace}, p.positionOfOr(bodyClose, body), true
}

func (p *parser) functionHeaderLikely(start, name, open int, currentType string) bool {
	nameText := p.tokens[p.sig[name]].Text
	if controlWords[nameText] || nameText == "sizeof" || nameText == "decltype" || nameText == "static_assert" {
		return false
	}
	if nameText == "." || nameText == "->" || nameText == "*" || nameText == "&" {
		return false
	}
	if name == start {
		return false
	}
	if currentType != "" && (nameText == currentType || nameText == "~"+currentType) {
		return true
	}
	for cursor := start; cursor < name; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "::" || text == "operator" || text == "template" || typeWords[text] || text == "auto" || text == "inline" || text == "static" || text == "constexpr" || text == "virtual" {
			return true
		}
	}
	return false
}

func (p *parser) functionName(namePosition, start int, namespace, currentType string) (string, string) {
	name := p.tokens[p.sig[namePosition]].Text
	if name == "=" || name == "(" {
		for cursor := namePosition - 1; cursor >= start; cursor-- {
			if p.tokens[p.sig[cursor]].Text == "operator" {
				name = "operator="
				break
			}
		}
	}
	if name == "bool" || name == "int" || name == "void" || name == "auto" {
		return "", ""
	}
	if namePosition > start && p.tokens[p.sig[namePosition-1]].Text == "~" {
		name = "~" + name
	}
	parts := []string{name}
	for cursor := namePosition - 1; cursor >= start+1; cursor-- {
		if p.tokens[p.sig[cursor]].Text != "::" || cursor == start {
			break
		}
		parts = append([]string{p.tokens[p.sig[cursor-1]].Text, "::"}, parts...)
		cursor--
	}
	qualifiedPrefix := strings.Join(parts, "")
	if strings.Contains(qualifiedPrefix, "::") {
		return name, joinQualified(namespace, qualifiedPrefix)
	}
	if currentType != "" {
		return name, joinQualified(namespace, currentType+"::"+name)
	}
	return name, joinQualified(namespace, name)
}

func (p *parser) parseDirectives() {
	includePattern := regexp.MustCompile(`^#\s*(include_next|include)\s*([<\"])([^>\"]+)[>\"]`)
	definePattern := regexp.MustCompile(`^#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	for index, token := range p.tokens {
		if token.Kind != Preprocessor || !token.Active {
			continue
		}
		line := strings.TrimSpace(token.Text)
		if match := includePattern.FindStringSubmatch(line); len(match) == 4 {
			source := "cpp:include:angle"
			if match[2] == `"` {
				source = "cpp:include:quote"
			}
			if match[1] == "include_next" {
				source = "cpp:include-next"
			}
			p.result.Includes = append(p.result.Includes, include{Target: match[3], Source: source})
		}
		if match := definePattern.FindStringSubmatch(line); len(match) == 2 {
			p.addDeclaration(&declaration{Kind: "macro", Name: match[1], Qualified: match[1], Start: index, HeaderEnd: token.EndByte, End: token.EndByte, BodyOpen: -1, BodyClose: -1})
		}
	}
	for position := 0; position < len(p.sig); position++ {
		text := p.tokens[p.sig[position]].Text
		if text != "import" && text != "module" && text != "export" {
			continue
		}
		end := position
		for end < len(p.sig) && p.tokens[p.sig[end]].Text != ";" && end < position+32 {
			end++
		}
		if end >= len(p.sig) {
			continue
		}
		line := joinTokens(p.tokens, p.sig[position:end+1])
		if text == "import" || strings.Contains(line, " import ") || strings.Contains(line, "export module") || text == "module" {
			kind := "cpp:module-import"
			if strings.Contains(line, "module") && !strings.HasPrefix(strings.TrimSpace(line), "import") {
				kind = "cpp:module-declaration"
			}
			p.result.Modules = append(p.result.Modules, include{Target: strings.TrimSpace(line), Source: kind})
		}
		position = end
	}
}

func (p *parser) addDeclaration(decl *declaration) {
	if decl == nil || decl.Start < 0 || p.seenStart[decl.Start] && decl.Kind != "macro" {
		return
	}
	if decl.Kind != "macro" {
		p.seenStart[decl.Start] = true
	}
	p.result.Declarations = append(p.result.Declarations, decl)
}

func (p *parser) findToken(start, fallback, hi int, wanted string) int {
	if start < fallback {
		start = fallback
	}
	for position := start; position < hi && position < start+128; position++ {
		if p.tokens[p.sig[position]].Text == wanted {
			return position
		}
		if p.tokens[p.sig[position]].Text == "{" || p.tokens[p.sig[position]].Text == "}" {
			return -1
		}
	}
	return -1
}

func (p *parser) positionOf(raw int) int {
	for position, index := range p.sig {
		if index == raw {
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
		last := p.sig[hi-1]
		return p.tokens[last].EndByte
	}
	return p.tokens[raw].EndByte
}

func isTypeKeyword(text string) bool {
	return text == "class" || text == "struct" || text == "union" || text == "enum"
}

func isTestMacro(text string) bool {
	return strings.HasPrefix(text, "TEST") || text == "SCENARIO" || text == "TEST_CASE" || text == "TYPED_TEST"
}

var controlWords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "sizeof": true, "decltype": true, "static_assert": true}
var typeWords = map[string]bool{"bool": true, "char": true, "char8_t": true, "char16_t": true, "char32_t": true, "double": true, "float": true, "int": true, "long": true, "short": true, "signed": true, "unsigned": true, "void": true, "auto": true, "const": true, "constexpr": true, "typename": true, "class": true, "struct": true}

func joinTokens(tokens []Token, indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		parts = append(parts, tokens[index].Text)
	}
	return strings.Join(parts, " ")
}

func joinQualified(namespace, name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(strings.Trim(name, ":")), " ", "")
	namespace = strings.ReplaceAll(strings.TrimSpace(strings.Trim(namespace, ":")), " ", "")
	if namespace == "" {
		return name
	}
	return namespace + "::" + name
}

func namespaceFor(value string) string {
	if position := strings.LastIndex(value, "::"); position >= 0 {
		return value[:position]
	}
	return value
}

func testName(args string) string {
	parts := strings.Split(args, ",")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	if strings.HasPrefix(args, `"`) && strings.HasSuffix(args, `"`) {
		return strings.Trim(args, `"`)
	}
	return strings.TrimSpace(args)
}

func normalizeInclude(filePath, target string) (string, bool) {
	if target == "" || strings.HasPrefix(target, "/") || strings.Contains(target, ":") {
		return "", false
	}
	clean := path.Clean(target)
	if strings.HasPrefix(target, "../") || strings.HasPrefix(target, "./") {
		clean = path.Clean(path.Join(path.Dir(filePath), target))
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}
