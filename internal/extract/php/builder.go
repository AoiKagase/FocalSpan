package php

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type phpBuilder struct {
	ctx       context.Context
	file      model.SourceFile
	parsed    parseResult
	allocator *model.HandleAllocator
	result    model.Extraction
	byDecl    map[*phpDecl]model.Symbol
	fileOwner model.Symbol
}

func buildExtraction(ctx context.Context, file model.SourceFile, parsed parseResult) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	b := &phpBuilder{ctx: ctx, file: file, parsed: parsed, allocator: model.NewHandleAllocator(), byDecl: make(map[*phpDecl]model.Symbol)}
	b.result.Diagnostics = append(b.result.Diagnostics, parsed.Diagnostics...)
	b.fileOwner = b.makeFileOwner()
	b.result.Symbols = append(b.result.Symbols, b.fileOwner)
	for _, decl := range parsed.Declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		if decl.Kind == "" || decl.Name == "" {
			continue
		}
		symbol := b.symbolForDecl(decl)
		b.byDecl[decl] = symbol
		b.result.Symbols = append(b.result.Symbols, symbol)
	}
	for _, decl := range parsed.Declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		symbol, ok := b.byDecl[decl]
		if !ok {
			continue
		}
		b.appendDeclarationChunk(decl, symbol)
		if parent := classOrNamespaceParent(decl); parent != nil {
			if parentSymbol, ok := b.byDecl[parent]; ok {
				b.addRelation(model.Relation{FromHandle: parentSymbol.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "php-structural"})
			}
		}
	}
	b.appendFallbackChunks()
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	b.buildRelations()
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	b.sortResult()
	return b.result, nil
}

func (b *phpBuilder) makeFileOwner() model.Symbol {
	qualified := b.file.Path
	handle := b.allocator.Allocate("sym", b.file.Path, "php", "file", qualified, qualified)
	return model.Symbol{Handle: handle, FilePath: b.file.Path, Language: "php", Kind: "file", Name: b.file.Path, QualifiedName: qualified, StartLine: 1, EndLine: lineCount(b.file.Content), StartByte: 0, EndByte: len(b.file.Content), Confidence: .7}
}

func (b *phpBuilder) symbolForDecl(decl *phpDecl) model.Symbol {
	kind := decl.Kind
	parentClass := classParent(decl)
	if kind == "function" && parentClass != nil {
		kind = "method"
		if b.isTestDecl(decl) {
			kind = "test"
		}
	}
	qualified := qualifiedName(decl)
	signature := b.rangeText(decl.Start, decl.HeaderEnd)
	if strings.TrimSpace(signature) == "" {
		signature = decl.Name
	}
	handle := b.allocator.Allocate("sym", b.file.Path, "php", kind, qualified, model.NormalizeSignature(signature))
	startByte, endByte, startLine, endLine := b.rawSpan(decl.Start, decl.End)
	confidence := 1.0
	if decl.Recovered {
		confidence = .55
	}
	name := decl.Name
	if decl.Kind == "namespace" {
		name = decl.Name
	}
	return model.Symbol{Handle: handle, FilePath: b.file.Path, Language: "php", Kind: kind, Name: name, QualifiedName: qualified, Signature: strings.TrimSpace(signature), StartLine: startLine, EndLine: endLine, StartByte: startByte, EndByte: endByte, ParentHandle: parentHandle(parentClass, b.byDecl), Confidence: confidence}
}

func (b *phpBuilder) appendDeclarationChunk(decl *phpDecl, symbol model.Symbol) {
	start, end := decl.Start, decl.End
	kind := symbol.Kind
	if isClassLike(decl.Kind) {
		kind = decl.Kind + "-outline"
		end = decl.HeaderEnd
	}
	if decl.Kind == "namespace" && decl.BodyOpen >= 0 {
		end = decl.HeaderEnd
	}
	content := b.rangeText(start, end)
	if strings.TrimSpace(content) == "" {
		return
	}
	b.result.Chunks = append(b.result.Chunks, b.newChunk(symbol, kind, start, end, content))
}

func (b *phpBuilder) appendFallbackChunks() {
	covered := make([]bool, len(b.parsed.Tokens))
	for _, decl := range b.parsed.Declarations {
		start, end := decl.Start, decl.End
		if isClassLike(decl.Kind) || decl.Kind == "namespace" && decl.BodyOpen >= 0 {
			end = decl.HeaderEnd
		}
		if start < 0 {
			start = 0
		}
		if end > len(covered) {
			end = len(covered)
		}
		for index := start; index < end; index++ {
			covered[index] = true
		}
	}
	start := -1
	end := -1
	kind := "procedural"
	for index, token := range b.parsed.Tokens {
		if err := b.ctx.Err(); err != nil {
			return
		}
		useful := isUsefulToken(token)
		if useful && !covered[index] {
			if start < 0 {
				start = index
				kind = "procedural"
			}
			if token.Kind == KindInlineHTML {
				kind = "template"
			}
			end = index
			continue
		}
		if start >= 0 && (covered[index] || useful) {
			b.appendFallbackRange(start, end, kind)
			start, end = -1, -1
		}
	}
	if start >= 0 {
		b.appendFallbackRange(start, end, kind)
	}
}

func (b *phpBuilder) appendFallbackRange(start, end int, kind string) {
	if start < 0 || end < start || start >= len(b.parsed.Tokens) {
		return
	}
	if end >= len(b.parsed.Tokens) {
		end = len(b.parsed.Tokens) - 1
	}
	const windowSize = 80
	const overlap = 10
	windowStart := start
	for windowStart <= end {
		if b.ctx.Err() != nil {
			return
		}
		windowLine := b.parsed.Tokens[windowStart].StartLine
		windowEnd := windowStart
		for windowEnd <= end && b.parsed.Tokens[windowEnd].EndLine-windowLine+1 <= windowSize {
			if b.ctx.Err() != nil {
				return
			}
			windowEnd++
		}
		if windowEnd == windowStart {
			windowEnd++
		}
		if windowEnd > end {
			windowEnd = end + 1
		}
		b.appendFallbackChunk(windowStart, windowEnd-1, kind)
		if windowEnd > end {
			return
		}

		nextLine := b.parsed.Tokens[windowEnd].StartLine - overlap
		nextStart := windowEnd
		for nextStart > windowStart && b.parsed.Tokens[nextStart-1].StartLine >= nextLine {
			if b.ctx.Err() != nil {
				return
			}
			nextStart--
		}
		if nextStart <= windowStart {
			nextStart = windowEnd
		}
		windowStart = nextStart
	}
}

func (b *phpBuilder) appendFallbackChunk(start, end int, kind string) {
	if start < 0 || end < start {
		return
	}
	content := b.rangeText(start, end+1)
	if strings.TrimSpace(content) == "" {
		return
	}
	b.result.Chunks = append(b.result.Chunks, b.newChunk(b.fileOwner, kind, start, end+1, content))
}

func (b *phpBuilder) newChunk(symbol model.Symbol, kind string, start, end int, content string) model.Chunk {
	startByte, endByte, startLine, endLine := b.rawSpan(start, end)
	digest := sha256.Sum256([]byte(content))
	handle := model.StableHandle("chunk", symbol.Handle, kind, strconv.Itoa(startByte), strconv.Itoa(endByte), content)
	symbolName := symbol.Name
	if symbol.Kind == "file" {
		symbolName = ""
	}
	return model.Chunk{Handle: handle, FilePath: b.file.Path, Language: "php", Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbolName, Signature: symbol.Signature, StartLine: startLine, EndLine: endLine, StartByte: startByte, EndByte: endByte, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func (b *phpBuilder) rangeText(start, end int) string {
	startByte, endByte, _, _ := b.rawSpan(start, end)
	if startByte < 0 || endByte < startByte || startByte > len(b.file.Content) {
		return ""
	}
	if endByte > len(b.file.Content) {
		endByte = len(b.file.Content)
	}
	return string(b.file.Content[startByte:endByte])
}

func (b *phpBuilder) rawSpan(start, end int) (int, int, int, int) {
	if len(b.parsed.Tokens) == 0 {
		return 0, 0, 1, 1
	}
	if start < 0 {
		start = 0
	}
	if start >= len(b.parsed.Tokens) {
		start = len(b.parsed.Tokens) - 1
	}
	if end <= start {
		end = start + 1
	}
	if end > len(b.parsed.Tokens) {
		end = len(b.parsed.Tokens)
	}
	first := b.parsed.Tokens[start]
	last := b.parsed.Tokens[end-1]
	return first.StartByte, last.EndByte, first.StartLine, last.EndLine
}

func (b *phpBuilder) addRelation(relation model.Relation) {
	for _, old := range b.result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	b.result.Relations = append(b.result.Relations, relation)
}

func (b *phpBuilder) sortResult() {
	sort.SliceStable(b.result.Symbols, func(i, j int) bool {
		if b.result.Symbols[i].StartByte != b.result.Symbols[j].StartByte {
			return b.result.Symbols[i].StartByte < b.result.Symbols[j].StartByte
		}
		return b.result.Symbols[i].Handle < b.result.Symbols[j].Handle
	})
	sort.SliceStable(b.result.Chunks, func(i, j int) bool {
		if b.result.Chunks[i].StartByte != b.result.Chunks[j].StartByte {
			return b.result.Chunks[i].StartByte < b.result.Chunks[j].StartByte
		}
		return b.result.Chunks[i].Handle < b.result.Chunks[j].Handle
	})
	sort.SliceStable(b.result.Relations, func(i, j int) bool {
		if b.result.Relations[i].FromHandle != b.result.Relations[j].FromHandle {
			return b.result.Relations[i].FromHandle < b.result.Relations[j].FromHandle
		}
		if b.result.Relations[i].Kind != b.result.Relations[j].Kind {
			return b.result.Relations[i].Kind < b.result.Relations[j].Kind
		}
		if b.result.Relations[i].ToHandle != b.result.Relations[j].ToHandle {
			return b.result.Relations[i].ToHandle < b.result.Relations[j].ToHandle
		}
		return b.result.Relations[i].UnresolvedTo < b.result.Relations[j].UnresolvedTo
	})
}

func qualifiedName(decl *phpDecl) string {
	if decl.Kind == "namespace" {
		return canonicalName(decl.Name)
	}
	if parent := classParent(decl); parent != nil {
		return qualifiedName(parent) + "::" + decl.Name
	}
	if decl.Namespace == "" {
		return decl.Name
	}
	return canonicalName(decl.Namespace + "\\" + decl.Name)
}

func classParent(decl *phpDecl) *phpDecl {
	for parent := decl.Parent; parent != nil; parent = parent.Parent {
		if isClassLike(parent.Kind) {
			return parent
		}
	}
	return nil
}

func classOrNamespaceParent(decl *phpDecl) *phpDecl {
	if decl.Parent == nil {
		return nil
	}
	if isClassLike(decl.Parent.Kind) || decl.Parent.Kind == "namespace" {
		return decl.Parent
	}
	return classOrNamespaceParent(decl.Parent)
}

func parentHandle(parent *phpDecl, symbols map[*phpDecl]model.Symbol) string {
	if parent == nil {
		return ""
	}
	return symbols[parent].Handle
}

func isClassLike(kind string) bool {
	return kind == "class" || kind == "interface" || kind == "trait" || kind == "enum"
}

func (b *phpBuilder) isTestDecl(decl *phpDecl) bool {
	class := classParent(decl)
	if class == nil || !b.isPHPUnitClass(class) {
		return false
	}
	if strings.HasPrefix(strings.ToLower(decl.Name), "test") {
		return true
	}
	return hasExactTestAttribute(decl.Attributes)
}

func (b *phpBuilder) isPHPUnitClass(decl *phpDecl) bool {
	for _, target := range decl.Extends {
		if b.isPHPUnitTestCaseReference(decl.Namespace, target) {
			return true
		}
	}
	return false
}

func (b *phpBuilder) isPHPUnitTestCaseReference(namespace, target string) bool {
	const testCase = "PHPUnit\\Framework\\TestCase"
	target = canonicalName(target)
	if strings.EqualFold(target, testCase) {
		return true
	}
	for _, use := range b.parsed.Uses {
		if !strings.EqualFold(use.Namespace, namespace) {
			continue
		}
		for _, imported := range b.importsInRange(use.Start, use.End) {
			if strings.EqualFold(imported.Alias, target) && strings.EqualFold(imported.Target, testCase) {
				return true
			}
		}
	}
	return false
}

func hasExactTestAttribute(attributes []string) bool {
	for _, attribute := range attributes {
		text := strings.TrimSpace(attribute)
		for strings.HasPrefix(text, "#[") {
			end := attributeGroupEnd(text)
			if end < 0 {
				break
			}
			group := text[2:end]
			for _, item := range strings.Split(group, ",") {
				name := strings.TrimSpace(item)
				if open := strings.IndexByte(name, '('); open >= 0 {
					name = name[:open]
				}
				name = canonicalName(name)
				if strings.EqualFold(name, "Test") || strings.EqualFold(name, "PHPUnit\\Framework\\Attributes\\Test") {
					return true
				}
			}
			text = strings.TrimSpace(text[end+1:])
		}
	}
	return false
}

func attributeGroupEnd(text string) int {
	parenDepth := 0
	bracketDepth := 0
	var quote byte
	for position := 2; position < len(text); position++ {
		value := text[position]
		if quote != 0 {
			if value == '\\' {
				position++
				continue
			}
			if value == quote {
				quote = 0
			}
			continue
		}
		if value == '\'' || value == '"' {
			quote = value
			continue
		}
		switch value {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			} else if parenDepth == 0 {
				return position
			}
		}
	}
	return -1
}

func isUsefulToken(token Token) bool {
	switch token.Kind {
	case KindWhitespace, KindLineComment, KindBlockComment, KindDocComment:
		return false
	default:
		return strings.TrimSpace(token.Text) != ""
	}
}

func lineCount(content []byte) int {
	count := 1
	for _, value := range content {
		if value == '\n' {
			count++
		}
	}
	return count
}

type phpImport struct {
	Kind   string
	Target string
	Alias  string
}

type phpSymbolIndex struct {
	byQualified map[string]model.Symbol
	byName      map[string][]model.Symbol
	byDecl      map[*phpDecl]model.Symbol
	aliases     map[string]map[string]string
}

func (b *phpBuilder) buildRelations() {
	index := b.indexSymbols()
	for _, decl := range b.parsed.Declarations {
		symbol, ok := b.byDecl[decl]
		if !ok {
			continue
		}
		for _, target := range append(append([]string(nil), decl.Extends...), decl.Implements...) {
			b.addResolvedOrUnresolved(index, symbol, decl.Namespace, target, "references", "inheritance")
		}
		b.buildDeclarationTypeRelations(index, decl, symbol)
		if isClassLike(decl.Kind) {
			b.buildTraitRelations(index, decl, symbol)
		}
		if decl.Kind == "function" || classParent(decl) != nil {
			b.buildBodyRelations(index, decl, symbol)
		}
	}
	b.buildFileOwnerRelations()
	for _, use := range b.parsed.Uses {
		owner := b.ownerForNamespace(index, use.Namespace)
		for _, imported := range b.importsInRange(use.Start, use.End) {
			target := imported.Target
			if resolved, ok := index.byQualified[normalizeLookup(target)]; ok {
				b.addRelation(model.Relation{FromHandle: owner.Handle, ToHandle: resolved.Handle, Kind: "imports", Confidence: .9, Source: "php-structural"})
				continue
			}
			b.addRelation(model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .55, Source: "php-structural"})
		}
	}
}

func (b *phpBuilder) buildFileOwnerRelations() {
	rawIndexes := significantRawIndexes(b.parsed.Tokens, 0, len(b.parsed.Tokens))
	compact := make([]Token, len(rawIndexes))
	for position, raw := range rawIndexes {
		compact[position] = b.parsed.Tokens[raw]
	}
	for position := 0; position < len(compact); position++ {
		if err := b.ctx.Err(); err != nil {
			return
		}
		text := strings.ToLower(compact[position].Text)
		if text != "include" && text != "include_once" && text != "require" && text != "require_once" {
			continue
		}
		if b.insideFunction(rawIndexes[position]) {
			continue
		}
		semi := nextCompactTokenText(compact, position+1, ";")
		if semi < 0 {
			continue
		}
		target, ok := b.resolveInclude(rawIndexes[position+1], rawIndexes[semi], nil)
		confidence := .8
		if !ok {
			target = shortExpression(b.parsed.Tokens, rawIndexes[position+1], rawIndexes[semi])
			confidence = .25
		}
		b.addRelation(model.Relation{FromHandle: b.fileOwner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: confidence, Source: text})
		position = semi
	}
}

func (b *phpBuilder) insideFunction(raw int) bool {
	for _, decl := range b.parsed.Declarations {
		if decl.Kind == "function" && decl.Start <= raw && raw < decl.End {
			return true
		}
	}
	return false
}

func (b *phpBuilder) buildDeclarationTypeRelations(index phpSymbolIndex, decl *phpDecl, symbol model.Symbol) {
	for _, target := range classReferencesInHeader(b.parsed.Tokens, decl.Start, decl.HeaderEnd) {
		b.addResolvedOrUnresolved(index, symbol, decl.Namespace, target, "references", "type")
	}
	if decl.Kind == "property" {
		for _, target := range propertyTypesInHeader(b.parsed.Tokens, decl.Start, decl.HeaderEnd) {
			b.addResolvedOrUnresolved(index, symbol, decl.Namespace, target, "references", "property-type")
		}
	}
	if decl.Kind == "function" {
		for _, target := range functionTypesInHeader(b.parsed.Tokens, decl.Start, decl.HeaderEnd) {
			b.addResolvedOrUnresolved(index, symbol, decl.Namespace, target, "references", "signature-type")
		}
	}
}

func (b *phpBuilder) indexSymbols() phpSymbolIndex {
	index := phpSymbolIndex{byQualified: make(map[string]model.Symbol), byName: make(map[string][]model.Symbol), byDecl: b.byDecl, aliases: make(map[string]map[string]string)}
	for _, symbol := range b.result.Symbols {
		if symbol.Kind == "file" {
			continue
		}
		index.byQualified[normalizeLookup(symbol.QualifiedName)] = symbol
		index.byName[strings.ToLower(symbol.Name)] = append(index.byName[strings.ToLower(symbol.Name)], symbol)
	}
	for _, use := range b.parsed.Uses {
		for _, imported := range b.importsInRange(use.Start, use.End) {
			aliases := index.aliases[use.Namespace]
			if aliases == nil {
				aliases = make(map[string]string)
				index.aliases[use.Namespace] = aliases
			}
			aliases[strings.ToLower(imported.Alias)] = imported.Target
		}
	}
	return index
}

func (b *phpBuilder) ownerForNamespace(index phpSymbolIndex, namespace string) model.Symbol {
	if symbol, ok := index.byQualified[normalizeLookup(namespace)]; ok {
		return symbol
	}
	return b.fileOwner
}

func (b *phpBuilder) buildTraitRelations(index phpSymbolIndex, decl *phpDecl, symbol model.Symbol) {
	start, end := decl.BodyOpen, decl.BodyClose
	if start < 0 || end <= start {
		return
	}
	for position := start; position < end; position++ {
		if strings.EqualFold(b.parsed.Tokens[position].Text, "use") {
			semi := nextTokenText(b.parsed.Tokens, position+1, ";")
			if semi < 0 || semi > end {
				continue
			}
			for _, target := range namesInRawRange(b.parsed.Tokens, position+1, semi) {
				b.addResolvedOrUnresolved(index, symbol, decl.Namespace, target, "references", "trait-use")
			}
			position = semi
		}
	}
}

func (b *phpBuilder) buildBodyRelations(index phpSymbolIndex, decl *phpDecl, symbol model.Symbol) {
	start, end := decl.BodyOpen+1, decl.BodyClose
	if decl.BodyOpen < 0 || decl.BodyClose < start {
		return
	}
	rawIndexes := significantRawIndexes(b.parsed.Tokens, start, end)
	compact := make([]Token, len(rawIndexes))
	for position, raw := range rawIndexes {
		compact[position] = b.parsed.Tokens[raw]
	}
	for position := 0; position < len(compact); position++ {
		if err := b.ctx.Err(); err != nil {
			return
		}
		text := strings.ToLower(compact[position].Text)
		if position+2 < len(compact) && compact[position+1].Text == "::" && strings.EqualFold(compact[position+2].Text, "class") {
			name, next := qualifiedNameAt(compact, position, len(compact))
			if name != "" {
				b.addResolvedOrUnresolved(index, symbol, decl.Namespace, name, "references", "class-reference")
				position = next
			}
			continue
		}
		if text == "include" || text == "include_once" || text == "require" || text == "require_once" {
			semi := nextCompactTokenText(compact, position+1, ";")
			if semi < 0 {
				continue
			}
			target, ok := b.resolveInclude(rawIndexes[position+1], rawIndexes[semi], decl)
			if ok {
				b.addRelation(model.Relation{FromHandle: symbol.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .8, Source: text})
			} else {
				b.addRelation(model.Relation{FromHandle: symbol.Handle, UnresolvedTo: shortExpression(b.parsed.Tokens, rawIndexes[position+1], rawIndexes[semi]), Kind: "imports", Confidence: .25, Source: text})
			}
			position = semi
			continue
		}
		if text == "new" {
			name, next := qualifiedNameAt(compact, position+1, len(compact))
			if name != "" {
				b.addResolvedOrUnresolved(index, symbol, decl.Namespace, name, "references", "new")
				position = next - 1
			}
			continue
		}
		if target, next, source, ok := callAt(compact, position, len(compact)); ok {
			if target == "" {
				position = next - 1
				continue
			}
			if resolved, ok := b.resolveCall(index, decl, target); ok {
				kind := "calls"
				if symbol.Kind == "test" {
					kind = "tests"
				}
				b.addRelation(model.Relation{FromHandle: symbol.Handle, ToHandle: resolved.Handle, Kind: kind, Confidence: .85, Source: source})
			} else {
				kind := "calls"
				if symbol.Kind == "test" {
					kind = "tests"
				}
				b.addRelation(model.Relation{FromHandle: symbol.Handle, UnresolvedTo: terminalCallName(target), Kind: kind, Confidence: .3, Source: source})
			}
			position = next - 1
		}
	}
}

func (b *phpBuilder) addResolvedOrUnresolved(index phpSymbolIndex, from model.Symbol, namespace, target, kind, source string) {
	if resolved, ok := resolveType(index, namespace, target); ok {
		b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: resolved.Handle, Kind: kind, Confidence: .8, Source: source})
		return
	}
	b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: canonicalName(target), Kind: kind, Confidence: .35, Source: source})
}

func (b *phpBuilder) resolveCall(index phpSymbolIndex, decl *phpDecl, target string) (model.Symbol, bool) {
	class := classParent(decl)
	rawTarget := strings.TrimSpace(target)
	absolute := strings.HasPrefix(rawTarget, "\\")
	target = canonicalName(rawTarget)
	lookup := target
	if strings.HasPrefix(strings.ToLower(target), "self::") || strings.HasPrefix(strings.ToLower(target), "static::") {
		if class == nil {
			return model.Symbol{}, false
		}
		lookup = qualifiedName(class) + target[strings.Index(target, "::"):]
	} else if strings.HasPrefix(strings.ToLower(target), "parent::") {
		if class == nil || len(class.Extends) == 0 {
			return model.Symbol{}, false
		}
		lookup = resolveAlias(index, decl.Namespace, class.Extends[0]) + target[strings.Index(target, "::"):]
	} else if strings.Contains(target, "::") {
		parts := strings.SplitN(target, "::", 2)
		for _, candidate := range callNameCandidates(index, decl.Namespace, parts[0], absolute) {
			if resolved, ok := index.byQualified[normalizeLookup(candidate+"::"+parts[1])]; ok && isCallableSymbol(resolved.Kind) {
				return resolved, true
			}
		}
		return model.Symbol{}, false
	} else if strings.HasPrefix(target, "$this->") {
		if class == nil {
			return model.Symbol{}, false
		}
		lookup = qualifiedName(class) + "::" + strings.TrimPrefix(target, "$this->")
	} else {
		for _, candidateName := range callNameCandidates(index, decl.Namespace, target, absolute) {
			if resolved, ok := index.byQualified[normalizeLookup(candidateName)]; ok && isCallableSymbol(resolved.Kind) {
				return resolved, true
			}
		}
		if !absolute && !strings.Contains(target, "\\") {
			matches := make([]model.Symbol, 0, len(index.byName[strings.ToLower(lastName(target))]))
			for _, candidate := range index.byName[strings.ToLower(lastName(target))] {
				if isCallableSymbol(candidate.Kind) {
					matches = append(matches, candidate)
				}
			}
			if len(matches) == 1 {
				return matches[0], true
			}
		}
		return model.Symbol{}, false
	}
	resolved, ok := index.byQualified[normalizeLookup(lookup)]
	return resolved, ok && isCallableSymbol(resolved.Kind)
}

func isCallableSymbol(kind string) bool {
	return kind == "function" || kind == "method" || kind == "test"
}

func callNameCandidates(index phpSymbolIndex, namespace, target string, absolute bool) []string {
	target = canonicalName(target)
	if absolute {
		return []string{target}
	}
	result := make([]string, 0, 3)
	appendUnique := func(value string) {
		if value == "" {
			return
		}
		for _, existing := range result {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		result = append(result, value)
	}
	appendUnique(resolveAlias(index, namespace, target))
	if namespace != "" {
		appendUnique(namespace + "\\" + target)
	}
	appendUnique(target)
	return result
}

func resolveType(index phpSymbolIndex, namespace, target string) (model.Symbol, bool) {
	lookup := resolveAlias(index, namespace, target)
	if symbol, ok := index.byQualified[normalizeLookup(lookup)]; ok {
		return symbol, true
	}
	for _, symbol := range index.byName[strings.ToLower(strings.TrimPrefix(lookup, "\\"))] {
		if isClassLike(symbol.Kind) {
			return symbol, true
		}
	}
	return model.Symbol{}, false
}

func resolveAlias(index phpSymbolIndex, namespace, target string) string {
	target = canonicalName(target)
	if target == "" {
		return target
	}
	if strings.Contains(target, "\\") {
		first, rest := target, ""
		if slash := strings.IndexByte(target, '\\'); slash >= 0 {
			first, rest = target[:slash], target[slash+1:]
		}
		if alias := index.aliases[namespace][strings.ToLower(first)]; alias != "" {
			if rest != "" {
				return alias + "\\" + rest
			}
			return alias
		}
	}
	if alias := index.aliases[namespace][strings.ToLower(target)]; alias != "" {
		return alias
	}
	if strings.Contains(target, "\\") || namespace == "" {
		return target
	}
	return namespace + "\\" + target
}

func (b *phpBuilder) importsInRange(start, end int) []phpImport {
	values := significantTokensInRange(b.parsed.Tokens, start, end)
	if len(values) == 0 || !strings.EqualFold(values[0].Text, "use") {
		return nil
	}
	position := 1
	kind := "class"
	if position < len(values) && (strings.EqualFold(values[position].Text, "function") || strings.EqualFold(values[position].Text, "const")) {
		kind = strings.ToLower(values[position].Text)
		position++
	}
	result := make([]phpImport, 0)
	for position < len(values) {
		base := make([]string, 0)
		for position < len(values) && values[position].Text != "," && values[position].Text != ";" && values[position].Text != "{" && !strings.EqualFold(values[position].Text, "as") {
			base = append(base, values[position].Text)
			position++
		}
		if position < len(values) && values[position].Text == "{" {
			prefix := strings.TrimSuffix(strings.Join(base, ""), "\\")
			position++
			for position < len(values) && values[position].Text != "}" {
				name := values[position].Text
				position++
				if name == "," {
					continue
				}
				alias := lastName(name)
				if position+1 < len(values) && strings.EqualFold(values[position].Text, "as") {
					alias = values[position+1].Text
					position += 2
				}
				result = append(result, phpImport{Kind: kind, Target: canonicalName(prefix + "\\" + name), Alias: alias})
			}
			if position < len(values) && values[position].Text == "}" {
				position++
			}
		} else {
			name := strings.Join(base, "")
			alias := lastName(name)
			if position < len(values) && strings.EqualFold(values[position].Text, "as") && position+1 < len(values) {
				alias = values[position+1].Text
				position += 2
			}
			if name != "" {
				result = append(result, phpImport{Kind: kind, Target: canonicalName(name), Alias: alias})
			}
		}
		for position < len(values) && values[position].Text != "," && values[position].Text != ";" {
			position++
		}
		if position < len(values) && values[position].Text == "," {
			position++
			continue
		}
		break
	}
	return result
}

func (b *phpBuilder) resolveInclude(start, end int, decl *phpDecl) (string, bool) {
	values := significantTokensInRange(b.parsed.Tokens, start, end)
	if len(values) == 0 {
		return "", false
	}
	result := ""
	baseApplied := false
	for position := 0; position < len(values); position++ {
		value := values[position].Text
		if value == "." {
			continue
		}
		if strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				unquoted = strings.Trim(value, "'\"")
			}
			result += unquoted
			continue
		}
		switch strings.ToLower(value) {
		case "__dir__":
			result += path.Dir(b.file.Path)
			baseApplied = true
		case "__file__":
			result += b.file.Path
			baseApplied = true
		default:
			compact := strings.ToLower(strings.Join(tokenTexts(values), ""))
			if strings.HasPrefix(compact, "dirname(__file__)") {
				result += path.Dir(b.file.Path)
				baseApplied = true
				position++
				for position < len(values) && values[position].Text != "." {
					position++
				}
				position--
				continue
			}
			return strings.TrimSpace(shortExpression(b.parsed.Tokens, start, end)), false
		}
	}
	result = strings.ReplaceAll(result, "\\", "/")
	if isAbsoluteIncludePath(result) {
		return "", false
	}
	if !baseApplied {
		result = path.Join(path.Dir(b.file.Path), result)
	} else {
		result = path.Clean(result)
	}
	if result == "." || result == "" || result == ".." || strings.HasPrefix(result, "../") || strings.HasPrefix(result, "/") {
		return "", false
	}
	return result, true
}

func isAbsoluteIncludePath(value string) bool {
	return strings.HasPrefix(value, "/") || (len(value) >= 2 && value[1] == ':')
}

func significantTokensInRange(tokens []Token, start, end int) []Token {
	result := make([]Token, 0)
	for index := start; index < end && index < len(tokens); index++ {
		token := tokens[index]
		switch token.Kind {
		case KindWhitespace, KindLineComment, KindBlockComment, KindDocComment:
			continue
		default:
			result = append(result, token)
		}
	}
	return result
}

func tokenTexts(tokens []Token) []string {
	result := make([]string, len(tokens))
	for index, token := range tokens {
		result[index] = token.Text
	}
	return result
}

func namesInRawRange(tokens []Token, start, end int) []string {
	result := make([]string, 0)
	current := make([]string, 0)
	flush := func() {
		if value := canonicalName(strings.Join(current, "")); value != "" {
			result = append(result, value)
		}
		current = current[:0]
	}
	for _, token := range significantTokensInRange(tokens, start, end) {
		if token.Text == "," {
			flush()
			continue
		}
		if token.Kind == KindIdentifier || token.Kind == KindKeyword || token.Text == "\\" {
			current = append(current, token.Text)
		}
	}
	flush()
	return result
}

func nextTokenText(tokens []Token, start int, text string) int {
	for index := start; index < len(tokens); index++ {
		if tokens[index].Text == text {
			return index
		}
	}
	return -1
}

func significantRawIndexes(tokens []Token, start, end int) []int {
	result := make([]int, 0)
	for index := start; index < end && index < len(tokens); index++ {
		switch tokens[index].Kind {
		case KindWhitespace, KindLineComment, KindBlockComment, KindDocComment:
			continue
		default:
			result = append(result, index)
		}
	}
	return result
}

func nextCompactTokenText(tokens []Token, start int, text string) int {
	for index := start; index < len(tokens); index++ {
		if tokens[index].Text == text {
			return index
		}
	}
	return -1
}

func classReferencesInHeader(tokens []Token, start, end int) []string {
	values := significantTokensInRange(tokens, start, end)
	result := make([]string, 0)
	for position := 0; position+2 < len(values); position++ {
		if values[position].Text == "#" && values[position+1].Text == "[" {
			if attribute := attributeName(values, position+2); attribute != "" {
				result = append(result, attribute)
			}
		}
		if values[position+1].Text != "::" || !strings.EqualFold(values[position+2].Text, "class") {
			continue
		}
		if target := qualifiedNameBefore(values, position+1); target != "" {
			result = append(result, target)
		}
	}
	return result
}

func attributeName(values []Token, start int) string {
	if start >= len(values) {
		return ""
	}
	name := ""
	position := start
	if values[position].Text == "\\" {
		name = "\\"
		position++
	}
	if position >= len(values) || values[position].Kind != KindIdentifier {
		return ""
	}
	name += values[position].Text
	for position += 1; position+1 < len(values) && values[position].Text == "\\" && values[position+1].Kind == KindIdentifier; position += 2 {
		name += "\\" + values[position+1].Text
	}
	return canonicalName(name)
}

func propertyTypesInHeader(tokens []Token, start, end int) []string {
	values := significantTokensInRange(tokens, start, end)
	result := make([]string, 0)
	segmentStart := 0
	for position, token := range values {
		if token.Text == "]" {
			segmentStart = position + 1
			continue
		}
		if token.Text == "," {
			segmentStart = position + 1
			continue
		}
		if token.Kind != KindVariable {
			continue
		}
		result = append(result, typeNames(values, segmentStart, position)...)
		segmentStart = position + 1
	}
	return result
}

func functionTypesInHeader(tokens []Token, start, end int) []string {
	values := significantTokensInRange(tokens, start, end)
	function := -1
	for position, token := range values {
		if strings.EqualFold(token.Text, "function") {
			function = position
			break
		}
	}
	if function < 0 {
		return nil
	}
	open := -1
	for position := function + 1; position < len(values); position++ {
		if values[position].Text == "(" {
			open = position
			break
		}
	}
	if open < 0 {
		return nil
	}
	close := matchingToken(values, open, "(", ")")
	if close < 0 {
		return nil
	}
	result := make([]string, 0)
	segmentStart := open + 1
	for position := open + 1; position < close; position++ {
		if values[position].Text == "," {
			segmentStart = position + 1
			continue
		}
		if values[position].Kind != KindVariable {
			continue
		}
		result = append(result, typeNames(values, segmentStart, position)...)
		segmentStart = position + 1
	}
	for position := close + 1; position < len(values); position++ {
		if values[position].Text != ":" {
			continue
		}
		finish := position + 1
		for finish < len(values) && values[finish].Text != "{" && values[finish].Text != ";" && values[finish].Text != "=>" {
			finish++
		}
		result = append(result, typeNames(values, position+1, finish)...)
		break
	}
	return result
}

func typeNames(tokens []Token, start, end int) []string {
	result := make([]string, 0)
	for position := start; position < end; {
		if !isTypeNameToken(tokens[position]) {
			position++
			continue
		}
		begin := position
		position++
		for position < end && (tokens[position].Text == "\\" || isTypeNameToken(tokens[position])) {
			position++
		}
		name := canonicalName(joinTokenTexts(tokens[begin:position]))
		if name != "" && !phpBuiltinTypes[strings.ToLower(name)] {
			result = append(result, name)
		}
	}
	return result
}

func isTypeNameToken(token Token) bool {
	if token.Text == "\\" || token.Kind == KindIdentifier {
		return true
	}
	if token.Kind != KindKeyword {
		return false
	}
	text := strings.ToLower(token.Text)
	return text == "self" || text == "static" || text == "parent" || !phpKeywords[text]
}

func qualifiedNameBefore(tokens []Token, end int) string {
	if end <= 0 || !isTypeNameToken(tokens[end-1]) {
		return ""
	}
	start := end - 1
	for start >= 2 && tokens[start-1].Text == "\\" && isTypeNameToken(tokens[start-2]) {
		start -= 2
	}
	return canonicalName(joinTokenTexts(tokens[start:end]))
}

func matchingToken(tokens []Token, open int, opening, closing string) int {
	depth := 0
	for position := open; position < len(tokens); position++ {
		switch tokens[position].Text {
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return position
			}
		}
	}
	return -1
}

func joinTokenTexts(tokens []Token) string {
	var result strings.Builder
	for _, token := range tokens {
		result.WriteString(token.Text)
	}
	return result.String()
}

var phpBuiltinTypes = map[string]bool{
	"array": true, "bool": true, "callable": true, "false": true, "float": true,
	"int": true, "iterable": true, "mixed": true, "never": true, "null": true,
	"object": true, "resource": true, "string": true, "true": true, "void": true,
}

func qualifiedNameAt(tokens []Token, start, end int) (string, int) {
	parts := make([]string, 0)
	position := start
	for position < end && position < len(tokens) {
		text := tokens[position].Text
		if tokens[position].Kind == KindIdentifier || tokens[position].Kind == KindKeyword || text == "\\" {
			parts = append(parts, text)
			position++
			continue
		}
		break
	}
	return canonicalName(strings.Join(parts, "")), position
}

func callAt(tokens []Token, position, end int) (string, int, string, bool) {
	if position >= end || position >= len(tokens) {
		return "", position, "", false
	}
	text := tokens[position].Text
	if tokens[position].Kind == KindVariable && position+3 < end && tokens[position+1].Text == "->" && tokens[position+2].Kind == KindIdentifier && tokens[position+3].Text == "(" {
		return text + "->" + tokens[position+2].Text, position + 4, text + "->" + tokens[position+2].Text, true
	}
	if tokens[position].Kind == KindIdentifier || tokens[position].Text == "\\" {
		name, next := qualifiedNameAt(tokens, position, end)
		if name != "" && next+3 < end && tokens[next].Text == "::" && tokens[next+1].Kind == KindIdentifier && tokens[next+2].Text == "(" {
			rawName := joinTokenTexts(tokens[position:next])
			target := rawName + "::" + tokens[next+1].Text
			return target, next + 3, target, true
		}
	}
	if position+3 < end && (text == "self" || text == "static" || text == "parent" || tokens[position].Kind == KindIdentifier) && tokens[position+1].Text == "::" && tokens[position+2].Kind == KindIdentifier && tokens[position+3].Text == "(" {
		return text + "::" + tokens[position+2].Text, position + 4, text + "::" + tokens[position+2].Text, true
	}
	if tokens[position].Kind == KindIdentifier || tokens[position].Text == "\\" {
		name, next := qualifiedNameAt(tokens, position, end)
		if name != "" && next < end && tokens[next].Text == "(" && !phpKeywords[strings.ToLower(lastName(name))] {
			rawName := joinTokenTexts(tokens[position:next])
			return rawName, next + 1, rawName + "()", true
		}
	}
	return "", position, "", false
}

func terminalCallName(target string) string {
	if arrow := strings.LastIndex(target, "->"); arrow >= 0 {
		return target[arrow+2:]
	}
	if colon := strings.LastIndex(target, "::"); colon >= 0 {
		return target[colon+2:]
	}
	return target
}

func shortExpression(tokens []Token, start, end int) string {
	parts := make([]string, 0)
	for _, token := range significantTokensInRange(tokens, start, end) {
		parts = append(parts, token.Text)
		if len(strings.Join(parts, "")) >= 80 {
			break
		}
	}
	return strings.Join(parts, "")
}

func lastName(value string) string {
	if slash := strings.LastIndexByte(value, '\\'); slash >= 0 {
		return value[slash+1:]
	}
	return value
}

func normalizeLookup(value string) string {
	return strings.ToLower(canonicalName(value))
}
