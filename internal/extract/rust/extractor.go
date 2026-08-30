package rust

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "rust-structural" }
func (Extractor) Supports(path, language string) bool {
	return language == "rust" || strings.EqualFold(filepath.Ext(path), ".rs")
}

type declaration struct {
	Kind      string
	Name      string
	Qualified string
	Start     int
	HeaderEnd int
	End       int
	BodyOpen  int
	BodyClose int
	Parent    *declaration
	Recovered bool
}

type parseResult struct {
	Tokens       []Token
	Significant  []int
	Matching     map[int]int
	Declarations []*declaration
	Diagnostics  []model.Diagnostic
}

type parser struct {
	ctx      context.Context
	tokens   []Token
	sig      []int
	matching map[int]int
	seen     map[int]bool
	result   parseResult
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	tokens, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed, err := parseRust(ctx, tokens, diagnostics)
	if err != nil {
		return model.Extraction{}, err
	}
	return build(ctx, file, parsed), nil
}

func parseRust(ctx context.Context, tokens []Token, diagnostics []model.Diagnostic) (parseResult, error) {
	p := &parser{ctx: ctx, tokens: tokens, matching: make(map[int]int), seen: make(map[int]bool), result: parseResult{Tokens: tokens, Diagnostics: append([]model.Diagnostic(nil), diagnostics...)}}
	for raw, token := range tokens {
		if token.significant() {
			p.sig = append(p.sig, raw)
		}
	}
	p.result.Significant = p.sig
	p.buildMatching()
	if err := p.parseRange(0, len(p.sig), nil, "crate"); err != nil {
		return parseResult{}, err
	}
	sort.SliceStable(p.result.Declarations, func(i, j int) bool {
		if p.result.Declarations[i].Start != p.result.Declarations[j].Start {
			return p.result.Declarations[i].Start < p.result.Declarations[j].Start
		}
		return p.result.Declarations[i].Kind < p.result.Declarations[j].Kind
	})
	return p.result, nil
}

func (p *parser) buildMatching() {
	opens := map[string]string{"(": ")", "{": "}", "[": "]"}
	stack := make([]int, 0)
	for position, raw := range p.sig {
		text := p.tokens[raw].Text
		if _, ok := opens[text]; ok {
			stack = append(stack, position)
			continue
		}
		if text != ")" && text != "}" && text != "]" {
			continue
		}
		for len(stack) > 0 {
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if opens[p.tokens[p.sig[open]].Text] == text {
				p.matching[open], p.matching[position] = position, open
				break
			}
		}
	}
	if len(stack) > 0 {
		p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{Level: "warning", Code: "rust_unbalanced_scope", Message: "one or more Rust delimiters are not balanced"})
	}
}

func (p *parser) parseRange(lo, hi int, parent *declaration, namespace string) error {
	pendingTest := false
	for position := lo; position < hi; position++ {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		raw := p.sig[position]
		text := p.tokens[raw].Text
		if p.tokens[raw].Kind == Attribute {
			if strings.Contains(strings.ToLower(text), "test") {
				pendingTest = true
			}
			continue
		}
		switch text {
		case "mod":
			if decl, next, ok := p.parseModule(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseRange(decl.BodyOpen+1, decl.BodyClose, decl, decl.Qualified); err != nil {
						return err
					}
				}
				position = next
				pendingTest = false
				continue
			}
		case "use":
			if decl, next, ok := p.parseUse(position, hi, parent, namespace); ok {
				p.add(decl)
				position = next
				pendingTest = false
				continue
			}
		case "struct", "enum", "union", "trait":
			if decl, next, ok := p.parseNamed(position, hi, parent, namespace, text); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseRange(decl.BodyOpen+1, decl.BodyClose, decl, namespace); err != nil {
						return err
					}
				}
				position = next
				pendingTest = false
				continue
			}
		case "impl":
			if decl, next, ok := p.parseImpl(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseRange(decl.BodyOpen+1, decl.BodyClose, decl, namespace); err != nil {
						return err
					}
				}
				position = next
				pendingTest = false
				continue
			}
		case "fn":
			if decl, next, ok := p.parseFunction(position, hi, parent, namespace, pendingTest); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseRange(decl.BodyOpen+1, decl.BodyClose, decl, namespace); err != nil {
						return err
					}
				}
				position = next
				pendingTest = false
				continue
			}
		case "type":
			if decl, next, ok := p.parseSimpleDeclaration(position, hi, parent, namespace, "type_alias"); ok {
				p.add(decl)
				position = next
				pendingTest = false
				continue
			}
		case "const":
			if decl, next, ok := p.parseSimpleDeclaration(position, hi, parent, namespace, "const"); ok {
				p.add(decl)
				position = next
				pendingTest = false
				continue
			}
		case "static":
			if decl, next, ok := p.parseSimpleDeclaration(position, hi, parent, namespace, "static"); ok {
				p.add(decl)
				position = next
				pendingTest = false
				continue
			}
		case "macro_rules":
			if decl, next, ok := p.parseMacro(position, hi, parent, namespace); ok {
				p.add(decl)
				position = next
				pendingTest = false
				continue
			}
		case "extern":
			if decl, next, ok := p.parseExtern(position, hi, parent, namespace); ok {
				p.add(decl)
				if decl.BodyOpen >= 0 && decl.BodyClose > decl.BodyOpen {
					if err := p.parseRange(decl.BodyOpen+1, decl.BodyClose, decl, namespace); err != nil {
						return err
					}
				}
				position = next
				pendingTest = false
				continue
			}
		}
	}
	return nil
}

func (p *parser) parseModule(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	namePosition := nextIdentifier(p, position+1, hi)
	if namePosition < 0 {
		return nil, position, false
	}
	name := p.tokens[p.sig[namePosition]].Text
	return p.delimitedDeclaration(position, hi, parent, namespace, "module", name, namePosition)
}

func (p *parser) parseUse(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	end := p.findStatementEnd(position+1, hi)
	if end < 0 {
		return nil, position, false
	}
	target := compactTokens(p.tokens, p.sig[position+1:end])
	if target == "" {
		return nil, position, false
	}
	name := target
	if index := strings.LastIndex(name, "::"); index >= 0 {
		name = name[index+2:]
	}
	decl := &declaration{Kind: "use", Name: name, Qualified: rustQualified(namespace, "use::"+name), Start: position, HeaderEnd: end, End: end + 1, Parent: parent}
	return decl, end, true
}

func (p *parser) parseNamed(position, hi int, parent *declaration, namespace, kind string) (*declaration, int, bool) {
	namePosition := nextIdentifier(p, position+1, hi)
	if namePosition < 0 {
		return nil, position, false
	}
	return p.delimitedDeclaration(position, hi, parent, namespace, kind, p.tokens[p.sig[namePosition]].Text, namePosition)
}

func (p *parser) parseImpl(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	open := p.findTopLevel(position+1, hi, "{")
	semi := p.findTopLevel(position+1, hi, ";")
	if open < 0 || semi >= 0 && semi < open {
		return nil, position, false
	}
	name := "impl"
	cursor := position + 1
	if cursor < open && p.tokens[p.sig[cursor]].Text == "<" {
		depth := 0
		for cursor < open {
			switch p.tokens[p.sig[cursor]].Text {
			case "<":
				depth++
			case ">":
				depth--
			}
			cursor++
			if depth == 0 {
				break
			}
		}
	}
	for ; cursor < open; cursor++ {
		text := p.tokens[p.sig[cursor]].Text
		if text == "for" {
			if target := nextIdentifier(p, cursor+1, open); target >= 0 {
				name = p.tokens[p.sig[target]].Text
			}
			break
		}
		if name == "impl" && isIdentifierToken(p.tokens[p.sig[cursor]]) && text != "where" {
			name = text
		}
	}
	close, recovered := p.closeOrEnd(open, hi)
	decl := &declaration{Kind: "impl", Name: name, Qualified: rustQualified(namespace, name), Start: position, HeaderEnd: open, End: close + 1, BodyOpen: open, BodyClose: close, Parent: parent, Recovered: recovered}
	return decl, close, true
}

func (p *parser) parseFunction(position, hi int, parent *declaration, namespace string, isTest bool) (*declaration, int, bool) {
	namePosition := nextIdentifier(p, position+1, hi)
	if namePosition < 0 {
		return nil, position, false
	}
	name := p.tokens[p.sig[namePosition]].Text
	open := p.findTopLevel(namePosition+1, hi, "(")
	if open < 0 {
		return nil, position, false
	}
	closeParen, recovered := p.closeOrEnd(open, hi)
	if closeParen < open {
		return nil, position, false
	}
	bodyOpen := p.findTopLevel(closeParen+1, hi, "{")
	semi := p.findTopLevel(closeParen+1, hi, ";")
	if bodyOpen < 0 || semi >= 0 && semi < bodyOpen {
		end := semi
		if end < 0 {
			end = closeParen
		}
		kind := p.functionKind(parent, isTest, position)
		qualified := p.functionQualified(parent, namespace, name)
		return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: position, HeaderEnd: end, End: end + 1, BodyOpen: -1, BodyClose: -1, Parent: parent, Recovered: recovered}, end, true
	}
	bodyClose, bodyRecovered := p.closeOrEnd(bodyOpen, hi)
	if bodyRecovered {
		recovered = true
	}
	kind := p.functionKind(parent, isTest, position)
	qualified := p.functionQualified(parent, namespace, name)
	return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: position, HeaderEnd: bodyOpen, End: bodyClose + 1, BodyOpen: bodyOpen, BodyClose: bodyClose, Parent: parent, Recovered: recovered}, bodyClose, true
}

func (p *parser) functionKind(parent *declaration, isTest bool, position int) string {
	if isTest {
		return "test"
	}
	if parent != nil && parent.Kind == "extern" || position > 0 && p.tokens[p.sig[position-1]].Text == "extern" {
		return "extern_function"
	}
	if parent != nil && parent.Kind == "trait" {
		return "trait_method"
	}
	if parent != nil && parent.Kind == "impl" {
		return "method"
	}
	return "function"
}

func (p *parser) functionQualified(parent *declaration, namespace, name string) string {
	if parent != nil && parent.Kind == "impl" || parent != nil && parent.Kind == "trait" {
		return parent.Qualified + "::" + name
	}
	return rustQualified(namespace, name)
}

func (p *parser) parseSimpleDeclaration(position, hi int, parent *declaration, namespace, kind string) (*declaration, int, bool) {
	namePosition := nextIdentifier(p, position+1, hi)
	if namePosition < 0 {
		return nil, position, false
	}
	end := p.findStatementEnd(position+1, hi)
	if end < 0 {
		end = hi - 1
	}
	if end < namePosition {
		return nil, position, false
	}
	if parent != nil && (parent.Kind == "trait" || parent.Kind == "impl") {
		if kind == "const" {
			kind = "associated_const"
		} else if kind == "type_alias" {
			kind = "associated_type"
		}
	}
	name := p.tokens[p.sig[namePosition]].Text
	qualified := rustQualified(namespace, name)
	if parent != nil && (parent.Kind == "trait" || parent.Kind == "impl") {
		qualified = parent.Qualified + "::" + name
	}
	return &declaration{Kind: kind, Name: name, Qualified: qualified, Start: position, HeaderEnd: end, End: end + 1, Parent: parent}, end, true
}

func (p *parser) parseMacro(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	namePosition := nextIdentifier(p, position+1, hi)
	if namePosition < 0 {
		return nil, position, false
	}
	open := p.findTopLevel(namePosition+1, hi, "{")
	if open < 0 {
		return nil, position, false
	}
	close, recovered := p.closeOrEnd(open, hi)
	name := p.tokens[p.sig[namePosition]].Text
	return &declaration{Kind: "macro", Name: name, Qualified: rustQualified(namespace, name), Start: position, HeaderEnd: open, End: close + 1, BodyOpen: open, BodyClose: close, Parent: parent, Recovered: recovered}, close, true
}

func (p *parser) parseExtern(position, hi int, parent *declaration, namespace string) (*declaration, int, bool) {
	open := p.findTopLevel(position+1, hi, "{")
	if open < 0 {
		return nil, position, false
	}
	close, recovered := p.closeOrEnd(open, hi)
	return &declaration{Kind: "extern", Name: "extern", Qualified: rustQualified(namespace, "extern"), Start: position, HeaderEnd: open, End: close + 1, BodyOpen: open, BodyClose: close, Parent: parent, Recovered: recovered}, close, true
}

func (p *parser) delimitedDeclaration(position, hi int, parent *declaration, namespace, kind, name string, namePosition int) (*declaration, int, bool) {
	open := p.findTopLevel(namePosition+1, hi, "{")
	semi := p.findTopLevel(namePosition+1, hi, ";")
	if open < 0 || semi >= 0 && semi < open {
		end := semi
		if end < 0 {
			end = namePosition
		}
		return &declaration{Kind: kind, Name: name, Qualified: rustQualified(namespace, name), Start: position, HeaderEnd: end, End: end + 1, Parent: parent}, end, true
	}
	close, recovered := p.closeOrEnd(open, hi)
	return &declaration{Kind: kind, Name: name, Qualified: rustQualified(namespace, name), Start: position, HeaderEnd: open, End: close + 1, BodyOpen: open, BodyClose: close, Parent: parent, Recovered: recovered}, close, true
}

func (p *parser) closeOrEnd(open, hi int) (int, bool) {
	if close, ok := p.matching[open]; ok {
		return close, false
	}
	p.result.Diagnostics = append(p.result.Diagnostics, model.Diagnostic{Level: "warning", Code: "rust_recovered_scope", Message: "Rust declaration body is not closed"})
	return hi - 1, true
}

func (p *parser) findTopLevel(start, hi int, wanted string) int {
	depth := 0
	for position := start; position < hi; position++ {
		text := p.tokens[p.sig[position]].Text
		if text == wanted && depth == 0 {
			return position
		}
		switch text {
		case "(", "[", "{", "<":
			depth++
		case ")", "]", "}", ">":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				return -1
			}
		}
	}
	return -1
}

func (p *parser) findStatementEnd(start, hi int) int {
	depth := 0
	for position := start; position < hi; position++ {
		switch p.tokens[p.sig[position]].Text {
		case "(", "[", "{", "<":
			depth++
		case ")", "]", "}", ">":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				return position
			}
		}
	}
	return -1
}

func nextIdentifier(p *parser, start, hi int) int {
	for position := start; position < hi; position++ {
		if isIdentifierToken(p.tokens[p.sig[position]]) {
			text := p.tokens[p.sig[position]].Text
			if text != "pub" && text != "async" && text != "unsafe" && text != "const" && text != "extern" && text != "where" && text != "for" {
				return position
			}
		}
		if p.tokens[p.sig[position]].Text == ";" || p.tokens[p.sig[position]].Text == "{" {
			break
		}
	}
	return -1
}

func (p *parser) add(decl *declaration) {
	if decl == nil || p.seen[decl.Start] {
		return
	}
	p.seen[decl.Start] = true
	p.result.Declarations = append(p.result.Declarations, decl)
}

type rustIndex struct {
	byName      map[string][]model.Symbol
	byQualified map[string][]model.Symbol
}

type rustBuilder struct {
	ctx       context.Context
	file      model.SourceFile
	parsed    parseResult
	allocator *model.HandleAllocator
	result    model.Extraction
	byDecl    map[*declaration]model.Symbol
	owner     model.Symbol
}

func build(ctx context.Context, file model.SourceFile, parsed parseResult) model.Extraction {
	b := &rustBuilder{ctx: ctx, file: file, parsed: parsed, allocator: model.NewHandleAllocator(), byDecl: make(map[*declaration]model.Symbol)}
	b.result.Diagnostics = append(b.result.Diagnostics, parsed.Diagnostics...)
	b.owner = model.Symbol{Handle: b.allocator.Allocate("sym", file.Path, "rust", "crate_module", file.Path), FilePath: file.Path, Language: file.Language, Kind: "crate_module", Name: file.Path, QualifiedName: file.Path, Signature: "crate module " + file.Path, StartLine: 1, EndLine: lineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	b.result.Symbols = append(b.result.Symbols, b.owner)
	b.addOwnerChunk()
	for _, decl := range parsed.Declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		start, end := b.declSpan(decl)
		if end <= start {
			continue
		}
		kind := decl.Kind
		qualified := decl.Qualified
		signature := strings.TrimSpace(string(file.Content[start:minInt(b.headerEnd(decl), len(file.Content))]))
		handle := b.allocator.Allocate("sym", file.Path, "rust", kind, qualified, model.NormalizeSignature(signature))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: kind, Name: decl.Name, QualifiedName: qualified, Signature: signature, StartLine: lineAt(file.Content, start), EndLine: lineAt(file.Content, end-1), StartByte: start, EndByte: end, Confidence: .95}
		if decl.Parent != nil {
			symbol.ParentHandle = b.byDecl[decl.Parent].Handle
		}
		b.byDecl[decl] = symbol
		b.result.Symbols = append(b.result.Symbols, symbol)
		b.addDeclChunk(decl, symbol, start, end)
	}
	idx := b.index()
	for _, decl := range parsed.Declarations {
		symbol, ok := b.byDecl[decl]
		if !ok {
			continue
		}
		parent := b.owner
		if decl.Parent != nil {
			parent = b.byDecl[decl.Parent]
		}
		b.addRelation(model.Relation{FromHandle: parent.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "rust-structural"})
		b.addHeaderRelations(idx, decl, symbol)
		b.addBodyRelations(idx, decl, symbol)
	}
	b.sort()
	return b.result
}

func (b *rustBuilder) addOwnerChunk() {
	lines := []string{"crate module " + b.file.Path}
	for _, decl := range b.parsed.Declarations {
		if decl.Parent == nil {
			lines = append(lines, decl.Kind+": "+decl.Qualified)
		}
	}
	content := strings.Join(lines, "\n")
	digest := sha256.Sum256([]byte(content))
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", b.owner.Handle, "module-outline", content), FilePath: b.file.Path, Language: b.file.Language, Kind: "module-outline", SymbolHandle: b.owner.Handle, SymbolName: b.file.Path, Signature: "synthetic outline", StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}

func (b *rustBuilder) addDeclChunk(decl *declaration, symbol model.Symbol, start, end int) {
	content := string(b.file.Content[start:end])
	kind := decl.Kind
	if decl.Kind == "struct" || decl.Kind == "enum" || decl.Kind == "union" || decl.Kind == "trait" || decl.Kind == "impl" || decl.Kind == "module" {
		end = minInt(b.headerEnd(decl), len(b.file.Content))
		if end <= start {
			return
		}
		content = string(b.file.Content[start:end])
		kind += "-outline"
	}
	digest := sha256.Sum256([]byte(content))
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), fmt.Sprint(end), content), FilePath: b.file.Path, Language: b.file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: lineAt(b.file.Content, start), EndLine: lineAt(b.file.Content, end-1), StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}

func (b *rustBuilder) headerEnd(decl *declaration) int {
	if decl.HeaderEnd < 0 || decl.HeaderEnd >= len(b.parsed.Significant) {
		return b.declSpanEnd(decl)
	}
	return b.parsed.Tokens[b.parsed.Significant[decl.HeaderEnd]].EndByte
}

func (b *rustBuilder) declSpan(decl *declaration) (int, int) {
	if decl.Start < 0 || decl.Start >= len(b.parsed.Significant) {
		return 0, 0
	}
	start := b.parsed.Tokens[b.parsed.Significant[decl.Start]].StartByte
	return start, b.declSpanEnd(decl)
}

func (b *rustBuilder) declSpanEnd(decl *declaration) int {
	end := decl.End - 1
	if end < 0 || end >= len(b.parsed.Significant) {
		end = len(b.parsed.Significant) - 1
	}
	if end < 0 {
		return 0
	}
	return b.parsed.Tokens[b.parsed.Significant[end]].EndByte
}

func (b *rustBuilder) index() rustIndex {
	idx := rustIndex{byName: make(map[string][]model.Symbol), byQualified: make(map[string][]model.Symbol)}
	for _, symbol := range b.result.Symbols {
		if symbol.Handle == b.owner.Handle {
			continue
		}
		idx.byName[strings.ToLower(symbol.Name)] = append(idx.byName[strings.ToLower(symbol.Name)], symbol)
		idx.byQualified[strings.ToLower(symbol.QualifiedName)] = append(idx.byQualified[strings.ToLower(symbol.QualifiedName)], symbol)
	}
	return idx
}

func (b *rustBuilder) addHeaderRelations(idx rustIndex, decl *declaration, from model.Symbol) {
	start := decl.Start
	end := decl.HeaderEnd
	if end < start {
		return
	}
	for position := start + 1; position < end && position < len(b.parsed.Significant); position++ {
		text := b.parsed.Tokens[b.parsed.Significant[position]].Text
		if text == "for" || text == "impl" || text == "where" || text == "pub" || text == "fn" || text == "struct" || text == "trait" || text == "enum" || text == "union" || text == "type" || text == "const" || text == "static" {
			continue
		}
		if !isIdentifierToken(b.parsed.Tokens[b.parsed.Significant[position]]) || !looksLikeRustType(text) {
			continue
		}
		b.addResolvedOrUnresolved(idx, from, text, "references", "rust-type")
	}
	if decl.Kind == "impl" {
		traitName, typeName := b.implTraitAndType(decl)
		trait, traitOK := uniqueRust(idx.byName, traitName)
		implementedType, typeOK := uniqueRustType(idx.byName, typeName)
		if traitOK && typeOK {
			b.addRelation(model.Relation{FromHandle: implementedType.Handle, ToHandle: trait.Handle, Kind: "references", Confidence: .8, Source: "rust-implements"})
		}
	}
	if decl.Kind == "use" {
		target := compactTokens(b.parsed.Tokens, b.parsed.Significant[decl.Start+1:decl.HeaderEnd])
		b.addRelation(model.Relation{FromHandle: b.owner.Handle, UnresolvedTo: normalizeRustPath(target), Kind: "imports", Confidence: .9, Source: "rust-use"})
		return
	}
	if decl.Kind == "module" {
		b.addRelation(model.Relation{FromHandle: b.owner.Handle, UnresolvedTo: decl.Qualified, Kind: "imports", Confidence: .8, Source: "rust-mod"})
	}
}

func (b *rustBuilder) implTraitAndType(decl *declaration) (string, string) {
	start := decl.Start + 1
	end := decl.HeaderEnd
	for position := start; position < end && position < len(b.parsed.Significant); position++ {
		if b.parsed.Tokens[b.parsed.Significant[position]].Text != "for" {
			continue
		}
		traitName := ""
		for before := start; before < position; before++ {
			token := b.parsed.Tokens[b.parsed.Significant[before]]
			if isIdentifierToken(token) && token.Text != "impl" && token.Text != "where" {
				traitName = token.Text
			}
		}
		typePosition := nextIdentifier(b.parserForBuilder(), position+1, end)
		if typePosition < 0 {
			return traitName, ""
		}
		return traitName, b.parsed.Tokens[b.parsed.Significant[typePosition]].Text
	}
	return "", ""
}

func (b *rustBuilder) parserForBuilder() *parser {
	return &parser{tokens: b.parsed.Tokens, sig: b.parsed.Significant}
}

func (b *rustBuilder) addBodyRelations(idx rustIndex, decl *declaration, from model.Symbol) {
	if decl.BodyOpen < 0 || decl.BodyClose <= decl.BodyOpen {
		return
	}
	for position := decl.BodyOpen + 1; position < decl.BodyClose && position < len(b.parsed.Significant); position++ {
		if err := b.ctx.Err(); err != nil {
			return
		}
		text := b.parsed.Tokens[b.parsed.Significant[position]].Text
		if text != "(" || position == 0 {
			continue
		}
		previous := position - 1
		if !isIdentifierToken(b.parsed.Tokens[b.parsed.Significant[previous]]) {
			continue
		}
		name := b.parsed.Tokens[b.parsed.Significant[previous]].Text
		if rustControl[name] || name == "fn" || name == "macro_rules" {
			continue
		}
		kind := "calls"
		if from.Kind == "test" {
			kind = "tests"
		}
		if target, ok := uniqueRust(idx.byName, name); ok {
			b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .9, Source: "rust-call"})
		} else {
			b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: normalizeRustPath(name), Kind: kind, Confidence: .25, Source: "rust-call"})
		}
	}
}

func (b *rustBuilder) addResolvedOrUnresolved(idx rustIndex, from model.Symbol, name, kind, source string) {
	if target, ok := uniqueRust(idx.byName, name); ok {
		b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .8, Source: source})
		return
	}
	b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: normalizeRustPath(name), Kind: kind, Confidence: .3, Source: source})
}

func (b *rustBuilder) addRelation(relation model.Relation) {
	if relation.FromHandle == "" || relation.FromHandle == relation.ToHandle {
		return
	}
	for _, old := range b.result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	b.result.Relations = append(b.result.Relations, relation)
}

func (b *rustBuilder) sort() {
	sort.SliceStable(b.result.Symbols, func(i, j int) bool {
		if b.result.Symbols[i].StartByte != b.result.Symbols[j].StartByte {
			return b.result.Symbols[i].StartByte < b.result.Symbols[j].StartByte
		}
		return b.result.Symbols[i].Kind < b.result.Symbols[j].Kind
	})
	sort.SliceStable(b.result.Chunks, func(i, j int) bool { return b.result.Chunks[i].StartByte < b.result.Chunks[j].StartByte })
	sort.SliceStable(b.result.Relations, func(i, j int) bool {
		if b.result.Relations[i].FromHandle != b.result.Relations[j].FromHandle {
			return b.result.Relations[i].FromHandle < b.result.Relations[j].FromHandle
		}
		if b.result.Relations[i].Kind != b.result.Relations[j].Kind {
			return b.result.Relations[i].Kind < b.result.Relations[j].Kind
		}
		return b.result.Relations[i].UnresolvedTo < b.result.Relations[j].UnresolvedTo
	})
}

var rustControl = map[string]bool{"if": true, "for": true, "while": true, "match": true, "loop": true, "println": true, "panic": true, "Some": false, "None": false}

func uniqueRust(values map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := values[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func uniqueRustType(values map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := values[strings.ToLower(name)]
	types := make([]model.Symbol, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case "struct", "enum", "union":
			types = append(types, item)
		}
	}
	if len(types) != 1 {
		return model.Symbol{}, false
	}
	return types[0], true
}

func rustQualified(namespace, name string) string {
	if namespace == "" || namespace == "crate" {
		return "crate::" + name
	}
	return namespace + "::" + name
}

func normalizeRustPath(value string) string {
	value = compactTokens(nil, strings.Fields(value))
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "self::")
	value = strings.TrimPrefix(value, "super::")
	return value
}

func compactTokens(tokens []Token, indexes interface{}) string {
	var values []string
	switch input := indexes.(type) {
	case []int:
		for _, index := range input {
			if index >= 0 && index < len(tokens) {
				values = append(values, tokens[index].Text)
			}
		}
	case []string:
		values = input
	}
	return strings.Join(values, "")
}

func isIdentifierToken(token Token) bool {
	return token.Kind == Identifier || token.Kind == Keyword && token.Text == "Self"
}
func looksLikeRustType(value string) bool {
	return value != "" && (value[0] >= 'A' && value[0] <= 'Z' || strings.Contains(value, "::"))
}
func nameAndGenerics(value string) string { return value }
func lineAt(source []byte, offset int) int {
	if offset < 0 {
		return 1
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}
func lineCount(source []byte) int { return 1 + strings.Count(string(source), "\n") }
func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
