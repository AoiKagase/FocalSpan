package jsts

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

type moduleRelation struct{ Target, Source, Alias string }

type parseResult struct {
	Tokens       []Token
	Significant  []int
	Declarations []*declaration
	Modules      []moduleRelation
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

func parseJSTS(ctx context.Context, tokens []Token) (parseResult, error) {
	p := &parser{ctx: ctx, tokens: tokens, matching: make(map[int]int), seen: make(map[int]bool)}
	p.result.Tokens = tokens
	for index, token := range tokens {
		if token.significant() {
			p.sig = append(p.sig, index)
		}
	}
	p.result.Significant = p.sig
	p.buildMatching()
	p.parseModules()
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
		p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{Level: "warning", Code: "jsts_unbalanced_scope", Message: "one or more delimiters are not balanced"})
	}
}

func (p *parser) parseModules() {
	for position := 0; position < len(p.sig); position++ {
		text := p.tokens[p.sig[position]].Text
		if text == "import" {
			if position+1 < len(p.sig) && p.tokens[p.sig[position+1]].Text == "(" {
				close := p.matching[p.sig[position+1]]
				if close > 0 {
					if target := firstString(p.tokens, p.sig[position+2:p.positionOf(close)]); target != "" {
						p.result.Modules = append(p.result.Modules, moduleRelation{Target: target, Source: "jsts:dynamic-import"})
					}
				}
				continue
			}
			end := statementEnd(p, position+1)
			if end < 0 {
				continue
			}
			for _, target := range stringLiterals(p.tokens, p.sig[position:end+1]) {
				p.result.Modules = append(p.result.Modules, moduleRelation{Target: target, Source: "jsts:import"})
			}
			for _, item := range importedNamesWithAliases(p.tokens, p.sig[position:end+1]) {
				p.result.Modules = append(p.result.Modules, moduleRelation{Target: item.Target, Alias: item.Alias, Source: "jsts:import-symbol"})
			}
			position = end
			continue
		}
		if text == "export" {
			if name := exportedDeclarationName(p.tokens, p.sig, position); name != "" {
				p.result.Modules = append(p.result.Modules, moduleRelation{Target: name, Source: "jsts:export-symbol"})
			}
			end := statementEnd(p, position+1)
			if end < 0 {
				continue
			}
			source := "jsts:export"
			if hasToken(p.tokens, p.sig[position:end+1], "from") {
				source = "jsts:reexport"
			}
			for _, target := range stringLiterals(p.tokens, p.sig[position:end+1]) {
				p.result.Modules = append(p.result.Modules, moduleRelation{Target: target, Source: source})
			}
			for _, name := range exportedNames(p.tokens, p.sig[position:end+1]) {
				p.result.Modules = append(p.result.Modules, moduleRelation{Target: name, Source: "jsts:export-symbol"})
			}
			position = end
		}
	}
	for position := 0; position+3 < len(p.sig); position++ {
		if p.tokens[p.sig[position]].Text != "require" || p.tokens[p.sig[position+1]].Text != "(" {
			continue
		}
		close := p.matching[p.sig[position+1]]
		if close == 0 {
			continue
		}
		if target := firstString(p.tokens, p.sig[position+2:p.positionOf(close)]); target != "" {
			p.result.Modules = append(p.result.Modules, moduleRelation{Target: target, Source: "jsts:require"})
		}
	}
	for position := 0; position+3 < len(p.sig); position++ {
		if p.tokens[p.sig[position]].Text != "module" || p.tokens[p.sig[position+1]].Text != "." || p.tokens[p.sig[position+2]].Text != "exports" {
			continue
		}
		end := statementEnd(p, position+3)
		if end < 0 {
			continue
		}
		for _, name := range exportedNames(p.tokens, p.sig[position+3:end+1]) {
			p.result.Modules = append(p.result.Modules, moduleRelation{Target: name, Source: "jsts:commonjs-export"})
		}
		position = end
	}
}

func (p *parser) parseScope(lo, hi int, parent *declaration, namespace, currentType string) error {
	for position := lo; position < hi; position++ {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		text := p.tokens[p.sig[position]].Text
		if isTestCall(text) {
			if decl, close, ok := p.parseTest(position, hi, parent, namespace); ok {
				p.add(decl)
				// describe/suite callbacks can contain nested test callbacks. Keep
				// the suite as an outline anchor, but walk its body so the actual
				// test case becomes a first-class symbol and relation source.
				if text == "describe" && decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, ""); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if text == "namespace" || text == "module" && position > 0 && p.tokens[p.sig[position-1]].Text == "declare" {
			if decl, close, ok := p.parseNamespace(position, hi, parent, namespace); ok {
				p.add(decl)
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
		if text == "function" || text == "async" || text == "export" || text == "default" {
			if decl, close, ok := p.parseFunction(position, hi, parent, namespace, currentType); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, ""); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if text == "const" || text == "let" || text == "var" {
			if decl, close, ok := p.parseVariableFunction(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, ""); err != nil {
						return err
					}
				}
				position = close
				continue
			}
		}
		if decl, close, ok := p.parseFunction(position, hi, parent, namespace, currentType); ok {
			p.add(decl)
			if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
				if err := p.parseScope(p.positionOf(decl.BodyOpen)+1, p.positionOf(decl.BodyClose), decl, namespace, ""); err != nil {
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
	for end < hi && (p.tokens[p.sig[end]].Kind == Identifier || p.tokens[p.sig[end]].Kind == StringLiteral || p.tokens[p.sig[end]].Text == ".") {
		end++
	}
	name := strings.ReplaceAll(strings.ReplaceAll(joinTokens(p.tokens, p.sig[position+1:end]), " . ", "."), " ", "")
	name = strings.Trim(name, "\"'")
	if name == "" {
		return nil, position, false
	}
	open := p.findUntil(end, hi, "{")
	if open < 0 {
		return nil, position, false
	}
	close := p.matching[p.sig[open]]
	qualified := joinQualified(namespace, strings.Trim(name, "\""))
	return &declaration{Kind: "namespace", Name: strings.Trim(name, "\""), Qualified: qualified, Start: p.sig[position], HeaderEnd: p.tokens[p.sig[open]].EndByte, End: p.endAfter(close, hi), BodyOpen: p.sig[open], BodyClose: close, Parent: parent, Namespace: namespace}, p.positionOfOr(close, open), true
}

func (p *parser) parseType(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	kind := p.tokens[p.sig[position]].Text
	namePos := position + 1
	if kind == "type" {
		if namePos >= hi || p.tokens[p.sig[namePos]].Kind != Identifier {
			return nil, position, false
		}
		name := p.tokens[p.sig[namePos]].Text
		semi := p.findUntil(namePos+1, hi, ";")
		if semi < 0 {
			return nil, position, false
		}
		return &declaration{Kind: "type", Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: p.tokens[p.sig[semi]].EndByte, End: p.tokens[p.sig[semi]].EndByte, BodyOpen: -1, BodyClose: -1, Parent: parent, Namespace: namespace}, semi, true
	}
	if namePos >= hi || p.tokens[p.sig[namePos]].Kind != Identifier {
		return nil, position, false
	}
	name := p.tokens[p.sig[namePos]].Text
	open := p.findUntil(namePos+1, hi, "{")
	semi := p.findUntil(namePos+1, hi, ";")
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
	functionPosition := position
	for cursor := position; cursor < hi && cursor < position+80; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == ";" || text == "{" || text == "}" {
			break
		}
		if text == "function" {
			functionPosition = cursor
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
	if name == "function" {
		name = "default"
	}
	if name == "*" {
		namePosition--
		name = p.tokens[p.sig[namePosition]].Text
	}
	if name == "{" || name == "=" || controlWords[name] {
		return nil, position, false
	}
	if !p.functionLikely(position, namePosition, currentType, name) {
		return nil, position, false
	}
	if name == "" {
		name = "default"
	}
	qualified := name
	if currentType != "" {
		qualified = currentType + "." + name
	}
	if namespace != "" {
		qualified = namespace + "." + qualified
	}
	kind := "function"
	if currentType != "" || parent != nil && parent.Kind != "namespace" && functionPosition == position {
		kind = "method"
	}
	if name == "default" {
		kind = "function"
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
		if text == "=>" {
			semi := p.findUntil(cursor+1, hi, ";")
			if semi >= 0 {
				endPosition = semi
				headerEnd = p.tokens[p.sig[semi]].EndByte
			}
			break
		}
		if text == ";" {
			endPosition = cursor
			headerEnd = p.tokens[p.sig[cursor]].EndByte
			break
		}
	}
	if endPosition <= closePosition && bodyOpen < 0 {
		return nil, position, false
	}
	return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: headerEnd, End: p.endAfter(p.sig[endPosition], hi), BodyOpen: bodyOpen, BodyClose: bodyClose, Parent: parent, Namespace: namespace, SignatureKey: joinTokens(p.tokens, p.sig[openPosition+1:closePosition])}, endPosition, true
}

func (p *parser) parseVariableFunction(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	if position+3 >= hi || p.tokens[p.sig[position+1]].Kind != Identifier || p.tokens[p.sig[position+2]].Text != "=" {
		return nil, position, false
	}
	name := p.tokens[p.sig[position+1]].Text
	value := position + 3
	if value < hi && p.tokens[p.sig[value]].Text == "async" {
		value++
	}
	if value < hi && p.tokens[p.sig[value]].Text == "function" {
		decl, close, ok := p.parseFunction(value, hi, parent, namespace, "")
		if !ok {
			return nil, position, false
		}
		decl.Kind = "function-expression"
		decl.Name = name
		decl.Qualified = joinQualified(namespace, name)
		decl.Start = p.sig[position]
		return decl, close, true
	}
	open := value
	if open >= hi || p.tokens[p.sig[open]].Text != "(" {
		if open >= hi || p.tokens[p.sig[open]].Kind != Identifier {
			return nil, position, false
		}
		open = value + 1
	}
	if open >= hi || p.tokens[p.sig[open]].Text != "(" {
		return nil, position, false
	}
	closeRaw := p.matching[p.sig[open]]
	if closeRaw == 0 {
		return nil, position, false
	}
	close := p.positionOf(closeRaw)
	if close+1 >= hi || p.tokens[p.sig[close+1]].Text != "=>" {
		return nil, position, false
	}
	bodyOpen, bodyClose := -1, -1
	end := close + 1
	headerEnd := p.tokens[p.sig[end]].EndByte
	if end+1 < hi && p.tokens[p.sig[end+1]].Text == "{" {
		bodyOpen = p.sig[end+1]
		bodyClose = p.matching[bodyOpen]
		end = p.positionOfOr(bodyClose, end+1)
		headerEnd = p.tokens[bodyOpen].EndByte
	} else if semi := p.findUntil(end+1, hi, ";"); semi >= 0 {
		end = semi
		headerEnd = p.tokens[p.sig[semi]].EndByte
	}
	return &declaration{Kind: "arrow_function", Name: name, Qualified: joinQualified(namespace, name), Start: p.sig[position], HeaderEnd: headerEnd, End: p.endAfter(p.sig[end], hi), BodyOpen: bodyOpen, BodyClose: bodyClose, Parent: parent, Namespace: namespace, SignatureKey: joinTokens(p.tokens, p.sig[open+1:close])}, end, true
}

func (p *parser) parseTest(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	if position+1 >= hi || p.tokens[p.sig[position+1]].Text != "(" {
		return nil, position, false
	}
	closeRaw := p.matching[p.sig[position+1]]
	if closeRaw == 0 {
		return nil, position, false
	}
	close := p.positionOf(closeRaw)
	body := -1
	for cursor := position + 2; cursor < close; cursor++ {
		if p.tokens[p.sig[cursor]].Text == "{" {
			body = cursor
			break
		}
	}
	if body < 0 {
		return nil, position, false
	}
	bodyClose := p.matching[p.sig[body]]
	name := firstString(p.tokens, p.sig[position+2:close])
	if name == "" {
		name = p.tokens[p.sig[position]].Text
	}
	qualified := name
	if namespace != "" {
		qualified = namespace + "." + name
	}
	kind := "test"
	if p.tokens[p.sig[position]].Text == "describe" {
		kind = "test-suite"
	}
	return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: p.sig[position], HeaderEnd: p.tokens[p.sig[body]].EndByte, End: p.endAfter(bodyClose, hi), BodyOpen: p.sig[body], BodyClose: bodyClose, Parent: parent, Namespace: namespace}, p.positionOfOr(bodyClose, body), true
}

func (p *parser) functionLikely(start, namePosition int, currentType, name string) bool {
	if currentType != "" {
		return name != "if" && name != "for" && name != "while" && name != "switch" && name != "catch"
	}
	if namePosition <= start {
		return false
	}
	for cursor := start; cursor < namePosition; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "function" || text == "async" || text == "export" || text == "default" || text == "get" || text == "set" {
			return true
		}
	}
	if namePosition != start {
		return false
	}
	previous := start - 1
	return previous >= 0 && (p.tokens[p.sig[previous]].Text == "{" || p.tokens[p.sig[previous]].Text == "," || p.tokens[p.sig[previous]].Text == ";")
}

func (p *parser) add(decl *declaration) {
	if decl == nil || decl.Start < 0 || p.seen[decl.Start] {
		return
	}
	p.seen[decl.Start] = true
	p.result.Declarations = append(p.result.Declarations, decl)
}

func (p *parser) findUntil(start, hi int, wanted string) int {
	for position := start; position < hi && position < start+200; position++ {
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
		if value := p.positionOf(raw); value >= 0 {
			return value
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
	return text == "class" || text == "interface" || text == "enum" || text == "type"
}
func isTestCall(text string) bool {
	return text == "test" || text == "it" || text == "specify" || text == "describe" || text == "TEST"
}

var controlWords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "with": true, "typeof": true, "import": true, "require": true}

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
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}
func statementEnd(p *parser, start int) int {
	for position := start; position < len(p.sig) && position < start+240; position++ {
		if p.tokens[p.sig[position]].Text == ";" {
			return position
		}
	}
	return -1
}
func firstString(tokens []Token, indexes []int) string {
	for _, index := range indexes {
		if tokens[index].Kind == StringLiteral {
			return strings.Trim(tokens[index].Text, "\"'")
		}
	}
	return ""
}
func stringLiterals(tokens []Token, indexes []int) []string {
	result := make([]string, 0)
	for _, index := range indexes {
		if tokens[index].Kind == StringLiteral {
			result = append(result, strings.Trim(tokens[index].Text, "\"'"))
		}
	}
	return result
}
func importedNames(tokens []Token, indexes []int) []string {
	result := make([]string, 0)
	for position, index := range indexes {
		text := tokens[index].Text
		if text == "from" || text == "import" || text == "as" || text == "{" || text == "}" || text == "," || text == "*" {
			continue
		}
		if tokens[index].Kind == Identifier && (position == 0 || tokens[indexes[position-1]].Text != ".") {
			result = append(result, text)
		}
	}
	return uniqueStrings(result)
}

type importedName struct{ Target, Alias string }

func importedNamesWithAliases(tokens []Token, indexes []int) []importedName {
	result := make([]importedName, 0)
	inside := false
	for position, index := range indexes {
		text := tokens[index].Text
		if text == "{" {
			inside = true
			continue
		}
		if text == "}" {
			inside = false
			continue
		}
		if !inside || tokens[index].Kind != Identifier || text == "as" {
			continue
		}
		if position > 0 && tokens[indexes[position-1]].Text == "as" && len(result) > 0 {
			result[len(result)-1].Alias = text
			continue
		}
		if position+1 < len(indexes) && tokens[indexes[position+1]].Text == "as" || position+1 == len(indexes) || tokens[indexes[position+1]].Text == "," || tokens[indexes[position+1]].Text == "}" {
			result = append(result, importedName{Target: text, Alias: text})
		}
	}
	return result
}

func exportedDeclarationName(tokens []Token, sig []int, position int) string {
	for cursor := position + 1; cursor < len(sig) && cursor < position+8; cursor++ {
		text := tokens[sig[cursor]].Text
		if text == "default" {
			return "default"
		}
		if text == "function" || text == "class" || text == "interface" || text == "enum" || text == "type" || text == "const" || text == "let" || text == "var" {
			if cursor+1 < len(sig) && tokens[sig[cursor+1]].Kind == Identifier {
				return tokens[sig[cursor+1]].Text
			}
			return "default"
		}
	}
	return ""
}
func exportedNames(tokens []Token, indexes []int) []string {
	result := make([]string, 0)
	for position, index := range indexes {
		text := tokens[index].Text
		if text == "export" || text == "default" || text == "from" || text == "{" || text == "}" || text == "," || text == ";" {
			continue
		}
		if tokens[index].Kind == Identifier && (position == 0 || tokens[indexes[position-1]].Text != ".") {
			result = append(result, text)
		}
	}
	return uniqueStrings(result)
}
func hasToken(tokens []Token, indexes []int, wanted string) bool {
	for _, index := range indexes {
		if tokens[index].Text == wanted {
			return true
		}
	}
	return false
}
func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
func normalizeModule(filePath, target string) []string {
	target = strings.TrimSpace(strings.Trim(target, "\"'"))
	if target == "" || !strings.HasPrefix(target, ".") {
		return []string{target}
	}
	clean := path.Clean(path.Join(path.Dir(filePath), target))
	result := []string{clean}
	if path.Ext(clean) == "" {
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			result = append(result, clean+ext)
		}
	}
	return uniqueStrings(result)
}
