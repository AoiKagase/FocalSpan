package cpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "cpp-structural" }

func (Extractor) Supports(path, language string) bool {
	if language == "c" || language == "cpp" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".c++", ".hh", ".hpp", ".hxx", ".inl", ".ipp", ".tpp", ".ixx", ".cppm":
		return true
	default:
		return false
	}
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	tokens, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed, err := parseCPP(ctx, tokens)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed.Diagnostics = append(diagnostics, parsed.Diagnostics...)
	for index := range parsed.Diagnostics {
		parsed.Diagnostics[index].FilePath = file.Path
	}
	result := build(ctx, file, parsed)
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	if len(result.Symbols) > 1 && len(parsed.Diagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: file.Path, Level: "warning", Code: "cpp_partial_extraction", Message: "C/C++ extraction recovered from malformed source"})
	}
	return result, nil
}

type symbolIndex struct {
	byQualified map[string][]model.Symbol
	byName      map[string][]model.Symbol
	byDecl      map[*declaration]model.Symbol
}

type builder struct {
	ctx       context.Context
	file      model.SourceFile
	parsed    parseResult
	mapa      sourceutil.SourceMap
	allocator *model.HandleAllocator
	result    model.Extraction
	byDecl    map[*declaration]model.Symbol
	owner     model.Symbol
}

func build(ctx context.Context, file model.SourceFile, parsed parseResult) model.Extraction {
	b := &builder{ctx: ctx, file: file, parsed: parsed, mapa: sourceutil.NewSourceMap(file.Content), allocator: model.NewHandleAllocator(), byDecl: make(map[*declaration]model.Symbol)}
	b.result.Diagnostics = append(b.result.Diagnostics, parsed.Diagnostics...)
	b.owner = b.makeOwner()
	b.result.Symbols = append(b.result.Symbols, b.owner)
	b.addOwnerChunk()
	chosen := preferredDeclarations(parsed.Declarations)
	for _, decl := range parsed.Declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		if decl.Name == "" || decl.Start < 0 || decl.Start >= len(parsed.Tokens) {
			continue
		}
		if !chosen[decl] {
			continue
		}
		if symbol, ok := b.byDecl[decl]; ok {
			_ = symbol
			continue
		}
		parent := decl.Parent
		if methodLike(decl.Kind) && strings.Contains(decl.Qualified, "::") {
			if receiver := b.receiverParent(decl); receiver != nil {
				parent = receiver
			}
		}
		if parent == nil {
			parent = b.lexicalParent(decl)
		}
		decl.Parent = parent
		kind := decl.Kind
		qualified := decl.Qualified
		if qualified == "" {
			qualified = decl.Name
		}
		signature := b.text(decl.Start, decl.HeaderEnd)
		if strings.TrimSpace(signature) == "" {
			signature = decl.Name
		}
		span := b.declSpan(decl)
		confidence := 1.0
		if decl.Recovered || span.EndByte <= span.StartByte {
			confidence = .55
		}
		handle := b.allocator.Allocate("sym", file.Path, file.Language, kind, qualified, model.NormalizeSignature(signature))
		parentHandle := ""
		if parent != nil {
			parentHandle = b.byDecl[parent].Handle
		}
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: kind, Name: decl.Name, QualifiedName: qualified, Signature: strings.TrimSpace(signature), StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, ParentHandle: parentHandle, Confidence: confidence}
		b.byDecl[decl] = symbol
		b.result.Symbols = append(b.result.Symbols, symbol)
	}
	for _, decl := range parsed.Declarations {
		if !chosen[decl] {
			continue
		}
		symbol, ok := b.byDecl[decl]
		if !ok {
			continue
		}
		b.addChunk(decl, symbol)
		parent := decl.Parent
		if parent == nil {
			b.addRelation(model.Relation{FromHandle: b.owner.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "cpp-structural"})
		} else if parentSymbol, exists := b.byDecl[parent]; exists {
			b.addRelation(model.Relation{FromHandle: parentSymbol.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "cpp-structural"})
		}
	}
	b.buildRelations()
	b.addDeclarationDefinitionRelations()
	b.sortResult()
	return b.result
}

func (b *builder) makeOwner() model.Symbol {
	handle := b.allocator.Allocate("sym", b.file.Path, b.file.Language, "translation_unit", b.file.Path)
	return model.Symbol{Handle: handle, FilePath: b.file.Path, Language: b.file.Language, Kind: "translation_unit", Name: b.file.Path, QualifiedName: b.file.Path, StartLine: 1, EndLine: b.mapa.LineCount(), StartByte: 0, EndByte: len(b.file.Content), Confidence: .7}
}

func (b *builder) addOwnerChunk() {
	lines := []string{"// FocalSpan synthetic translation unit outline: " + b.file.Path}
	for _, item := range b.parsed.Includes {
		lines = append(lines, "include: "+item.Target)
	}
	for _, item := range b.parsed.Modules {
		lines = append(lines, "module: "+item.Target)
	}
	for _, decl := range b.parsed.Declarations {
		if decl.Parent == nil {
			lines = append(lines, decl.Kind+": "+decl.Qualified)
		}
	}
	content := strings.Join(lines, "\n")
	digest := sha256.Sum256([]byte(content))
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", b.owner.Handle, "translation-unit-outline", content), FilePath: b.file.Path, Language: b.file.Language, Kind: "translation-unit-outline", SymbolHandle: b.owner.Handle, SymbolName: b.file.Path, Signature: "synthetic outline (not a source slice)", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 0, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}

func (b *builder) addChunk(decl *declaration, symbol model.Symbol) {
	start, end := decl.Start, decl.End
	if start < 0 || start >= len(b.parsed.Tokens) {
		return
	}
	if decl.Kind == "class" || decl.Kind == "struct" || decl.Kind == "union" || decl.Kind == "enum" || decl.Kind == "namespace" {
		end = decl.HeaderEnd
	}
	span := b.declSpanBytes(start, end)
	if span.EndByte <= span.StartByte {
		return
	}
	content, ok := b.mapa.Slice(span)
	if !ok || strings.TrimSpace(content) == "" {
		return
	}
	digest := sha256.Sum256([]byte(content))
	chunkKind := symbol.Kind
	if decl.Kind == "class" || decl.Kind == "struct" || decl.Kind == "union" || decl.Kind == "enum" || decl.Kind == "namespace" {
		chunkKind += "-outline"
	}
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, chunkKind, fmt.Sprint(span.StartByte), fmt.Sprint(span.EndByte), content), FilePath: b.file.Path, Language: b.file.Language, Kind: chunkKind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}

func (b *builder) buildRelations() {
	index := symbolIndex{byQualified: make(map[string][]model.Symbol), byName: make(map[string][]model.Symbol), byDecl: b.byDecl}
	for _, symbol := range b.result.Symbols {
		if symbol.Handle == b.owner.Handle {
			continue
		}
		index.byQualified[strings.ToLower(symbol.QualifiedName)] = append(index.byQualified[strings.ToLower(symbol.QualifiedName)], symbol)
		index.byName[strings.ToLower(symbol.Name)] = append(index.byName[strings.ToLower(symbol.Name)], symbol)
	}
	for _, item := range b.parsed.Includes {
		target := item.Target
		if item.Source == "cpp:include:quote" {
			if resolved, ok := normalizeInclude(b.file.Path, target); ok {
				target = resolved
			}
		}
		b.addRelation(model.Relation{FromHandle: b.owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .8, Source: item.Source})
	}
	for _, item := range b.parsed.Modules {
		b.addRelation(model.Relation{FromHandle: b.owner.Handle, UnresolvedTo: item.Target, Kind: "imports", Confidence: .75, Source: item.Source})
	}
	for decl, symbol := range b.byDecl {
		b.buildReferences(index, decl, symbol)
		if decl.BodyOpen >= 0 && (decl.Kind == "function" || decl.Kind == "method" || decl.Kind == "constructor" || decl.Kind == "destructor" || decl.Kind == "operator" || decl.Kind == "test") {
			b.buildBodyRelations(index, decl, symbol)
		}
	}
}

func (b *builder) addDeclarationDefinitionRelations() {
	groups := make(map[string][]*declaration)
	for _, decl := range b.parsed.Declarations {
		groups[declarationKey(decl)] = append(groups[declarationKey(decl)], decl)
	}
	for _, group := range groups {
		for _, definition := range group {
			if definition.BodyOpen < 0 || !isCallableDeclaration(definition) {
				continue
			}
			from, ok := b.byDecl[definition]
			if !ok {
				continue
			}
			paired := false
			for _, declaration := range group {
				if declaration.BodyOpen >= 0 || !isCallableDeclaration(declaration) {
					continue
				}
				to, exists := b.byDecl[declaration]
				if !exists {
					continue
				}
				paired = true
				b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: to.Handle, Kind: "declares", Confidence: .95, Source: "cpp:definition"})
			}
			if !paired {
				b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: declarationTarget(from), Kind: "declaration", Confidence: .45, Source: "cpp:definition"})
			}
		}
		for _, declaration := range group {
			if declaration.BodyOpen >= 0 || !isCallableDeclaration(declaration) {
				continue
			}
			from, ok := b.byDecl[declaration]
			if !ok {
				continue
			}
			paired := false
			for _, definition := range group {
				if definition.BodyOpen < 0 || !isCallableDeclaration(definition) {
					continue
				}
				if _, exists := b.byDecl[definition]; exists {
					paired = true
				}
			}
			if !paired {
				b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: declarationTarget(from), Kind: "declaration", Confidence: .45, Source: "cpp:declaration"})
			}
		}
	}
}

func isCallableDeclaration(decl *declaration) bool {
	switch decl.Kind {
	case "function", "method", "constructor", "destructor", "operator":
		return true
	default:
		return false
	}
}

func declarationTarget(symbol model.Symbol) string {
	return symbol.QualifiedName + " " + model.NormalizeSignature(symbol.Signature)
}

func (b *builder) buildReferences(index symbolIndex, decl *declaration, from model.Symbol) {
	if decl.Kind == "test" || decl.Kind == "macro" {
		return
	}
	end := decl.HeaderEnd
	if end <= 0 {
		return
	}
	for _, token := range b.parsed.Tokens {
		if token.StartByte < b.byteStart(decl) || token.StartByte >= end || !token.significant() || token.Kind != Identifier {
			continue
		}
		name := token.Text
		if name == decl.Name || primitiveType[name] || controlWords[name] || name == "std" {
			continue
		}
		if !looksLikeTypeReference(name) && !hasSymbol(index, name) {
			continue
		}
		b.addResolvedOrUnresolved(index, from, name, "references", "type")
	}
}

func declarationKey(decl *declaration) string {
	return decl.Kind + "\x00" + strings.ToLower(decl.Qualified) + "\x00" + model.NormalizeSignature(decl.Name) + "\x00" + model.NormalizeSignature(decl.SignatureKey)
}

func preferredDeclarations(declarations []*declaration) map[*declaration]bool {
	groups := make(map[string][]*declaration)
	for _, decl := range declarations {
		groups[declarationKey(decl)] = append(groups[declarationKey(decl)], decl)
	}
	result := make(map[*declaration]bool, len(declarations))
	for _, group := range groups {
		if hasDeclarationAndDefinition(group) {
			for _, decl := range group {
				result[decl] = true
			}
			continue
		}
		var preferred *declaration
		for _, decl := range group {
			if preferred == nil || preferred.BodyOpen < 0 && decl.BodyOpen >= 0 {
				preferred = decl
			}
		}
		if preferred != nil {
			result[preferred] = true
		}
	}
	return result
}

func hasDeclarationAndDefinition(group []*declaration) bool {
	hasDeclaration, hasDefinition := false, false
	for _, decl := range group {
		if decl.BodyOpen < 0 {
			hasDeclaration = true
		} else {
			hasDefinition = true
		}
	}
	return hasDeclaration && hasDefinition
}

func (b *builder) buildBodyRelations(index symbolIndex, decl *declaration, from model.Symbol) {
	start := decl.BodyOpen + 1
	end := decl.BodyClose
	if start <= 0 || end <= start {
		return
	}
	for position := start; position < end; position++ {
		if err := b.ctx.Err(); err != nil {
			return
		}
		token := b.parsed.Tokens[position]
		if !token.significant() || token.Text != "(" {
			continue
		}
		previous := previousSignificant(b.parsed.Tokens, position)
		if previous < 0 {
			continue
		}
		name, qualified := callName(b.parsed.Tokens, previous)
		if name == "" || controlWords[name] || name == "sizeof" || name == "decltype" || name == "static_assert" {
			continue
		}
		kind := "calls"
		if from.Kind == "test" {
			kind = "tests"
		}
		if target, ok := b.resolveCall(index, decl, name, qualified); ok {
			b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .95, Source: "cpp-call"})
		} else {
			lexical := qualified
			if lexical == "" {
				lexical = name
			}
			b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: lexical, Kind: kind, Confidence: .25, Source: "cpp-call"})
		}
		if isCallbackRegistration(name) {
			b.addCallbackReferences(index, from, position, end)
		}
	}
	for position := start; position+1 < end; position++ {
		if !b.parsed.Tokens[position].significant() || b.parsed.Tokens[position].Text != "new" {
			continue
		}
		next := nextSignificant(b.parsed.Tokens, position+1, end)
		if next >= 0 && b.parsed.Tokens[next].Kind == Identifier {
			b.addResolvedOrUnresolved(index, from, b.parsed.Tokens[next].Text, "references", "new")
		}
	}
}

func (b *builder) addCallbackReferences(index symbolIndex, from model.Symbol, open, end int) {
	close := matchingCallClose(b.parsed.Tokens, open, end)
	if close <= open {
		return
	}
	seen := make(map[string]struct{})
	for position := open + 1; position < close; position++ {
		token := b.parsed.Tokens[position]
		if !token.significant() || token.Kind != Identifier {
			continue
		}
		if _, exists := seen[token.Text]; exists {
			continue
		}
		seen[token.Text] = struct{}{}
		target, ok := unique(index.byName, strings.ToLower(token.Text))
		if !ok || !isCallableKind(target.Kind) || target.Handle == from.Handle {
			continue
		}
		b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: "references", Confidence: .65, Source: "cpp:callback"})
	}
}

func matchingCallClose(tokens []Token, open, end int) int {
	depth := 0
	for position := open; position < end; position++ {
		if !tokens[position].significant() {
			continue
		}
		switch tokens[position].Text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return position
			}
		}
	}
	return -1
}

func isCallbackRegistration(name string) bool {
	switch strings.ToLower(name) {
	case "register_callback", "settimer", "signal":
		return true
	default:
		return false
	}
}

func isCallableKind(kind string) bool {
	switch kind {
	case "function", "method", "constructor", "destructor", "operator":
		return true
	default:
		return false
	}
}

func (b *builder) resolveCall(index symbolIndex, decl *declaration, name, qualified string) (model.Symbol, bool) {
	if qualified != "" {
		if target, ok := unique(index.byQualified, strings.ToLower(qualified)); ok {
			return target, true
		}
		if target, ok := unique(index.byQualified, strings.ToLower(joinQualified(decl.Namespace, qualified))); ok {
			return target, true
		}
	}
	if parent := decl.Parent; parent != nil && (parent.Kind == "class" || parent.Kind == "struct") {
		if target, ok := unique(index.byQualified, strings.ToLower(parent.Qualified+"::"+name)); ok {
			return target, true
		}
	}
	if target, ok := unique(index.byQualified, strings.ToLower(joinQualified(decl.Namespace, name))); ok {
		return target, true
	}
	if target, ok := unique(index.byName, strings.ToLower(name)); ok {
		return target, true
	}
	return model.Symbol{}, false
}

func (b *builder) addResolvedOrUnresolved(index symbolIndex, from model.Symbol, name, kind, source string) {
	if target, ok := unique(index.byQualified, strings.ToLower(name)); ok {
		b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .9, Source: source})
		return
	}
	if target, ok := unique(index.byName, strings.ToLower(name)); ok {
		b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .8, Source: source})
		return
	}
	if looksLikeTypeReference(name) {
		b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .3, Source: source})
	}
}

func (b *builder) lexicalParent(decl *declaration) *declaration {
	if receiver := b.receiverParent(decl); receiver != nil {
		return receiver
	}
	best := (*declaration)(nil)
	for _, candidate := range b.parsed.Declarations {
		if candidate == decl || candidate.BodyOpen < 0 || candidate.BodyClose <= candidate.BodyOpen {
			continue
		}
		candidateStart := b.byteStart(candidate)
		declStart := b.byteStart(decl)
		if candidateStart < declStart && declStart < candidate.End && (best == nil || candidateStart > b.byteStart(best)) && (candidate.Kind == "class" || candidate.Kind == "struct" || candidate.Kind == "union" || candidate.Kind == "namespace") {
			best = candidate
		}
	}
	if best == nil && strings.Contains(decl.Qualified, "::") {
		receiver := decl.Qualified[:strings.LastIndex(decl.Qualified, "::")]
		for _, candidate := range b.parsed.Declarations {
			if (candidate.Kind == "class" || candidate.Kind == "struct") && strings.EqualFold(candidate.Qualified, receiver) {
				return candidate
			}
		}
	}
	return best
}

func (b *builder) receiverParent(decl *declaration) *declaration {
	if !strings.Contains(decl.Qualified, "::") {
		return nil
	}
	receiver := decl.Qualified[:strings.LastIndex(decl.Qualified, "::")]
	for _, candidate := range b.parsed.Declarations {
		if (candidate.Kind == "class" || candidate.Kind == "struct" || candidate.Kind == "union") && strings.EqualFold(candidate.Qualified, receiver) {
			return candidate
		}
	}
	return nil
}

func methodLike(kind string) bool {
	return kind == "method" || kind == "constructor" || kind == "destructor" || kind == "operator"
}

func (b *builder) declSpan(decl *declaration) sourceutil.Span {
	return b.declSpanBytes(decl.Start, decl.End)
}

func (b *builder) declSpanBytes(start, end int) sourceutil.Span {
	if start < 0 || start >= len(b.parsed.Tokens) {
		return sourceutil.Span{}
	}
	startByte := b.parsed.Tokens[start].StartByte
	if end < startByte {
		end = startByte
	}
	if end > len(b.file.Content) {
		end = len(b.file.Content)
	}
	span, _ := b.mapa.Span(startByte, end)
	return span
}

func (b *builder) byteStart(decl *declaration) int {
	if decl.Start >= 0 && decl.Start < len(b.parsed.Tokens) {
		return b.parsed.Tokens[decl.Start].StartByte
	}
	return 0
}

func (b *builder) text(start, endByte int) string {
	span := b.declSpanBytes(start, endByte)
	value, _ := b.mapa.Slice(span)
	return value
}

func (b *builder) addRelation(relation model.Relation) {
	for _, old := range b.result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	b.result.Relations = append(b.result.Relations, relation)
}

func (b *builder) sortResult() {
	sort.SliceStable(b.result.Symbols, func(i, j int) bool { return b.result.Symbols[i].StartByte < b.result.Symbols[j].StartByte })
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

var primitiveType = map[string]bool{"bool": true, "char": true, "char8_t": true, "char16_t": true, "char32_t": true, "double": true, "float": true, "int": true, "long": true, "short": true, "signed": true, "unsigned": true, "void": true, "auto": true, "size_t": true, "std": true}

func looksLikeTypeReference(value string) bool {
	return value != "" && (value[0] >= 'A' && value[0] <= 'Z' || strings.Contains(value, "::"))
}

func hasSymbol(index symbolIndex, name string) bool {
	return len(index.byName[strings.ToLower(name)]) > 0
}

func unique(values map[string][]model.Symbol, key string) (model.Symbol, bool) {
	items := values[key]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func previousSignificant(tokens []Token, position int) int {
	for position--; position >= 0; position-- {
		if tokens[position].significant() {
			return position
		}
	}
	return -1
}

func nextSignificant(tokens []Token, position, end int) int {
	for ; position < end; position++ {
		if tokens[position].significant() {
			return position
		}
	}
	return -1
}

func callName(tokens []Token, previous int) (string, string) {
	name := tokens[previous].Text
	if tokens[previous].Kind != Identifier && name != ")" {
		return "", ""
	}
	parts := []string{name}
	for position := previous - 1; position >= 1; {
		if !tokens[position].significant() {
			position--
			continue
		}
		operator := tokens[position].Text
		if operator != "::" && operator != "->" && operator != "." {
			break
		}
		left := previousSignificant(tokens, position)
		if left < 0 {
			break
		}
		parts = append([]string{tokens[left].Text, operator}, parts...)
		previous = left
		position = left - 1
	}
	full := strings.Join(parts, "")
	if position := strings.LastIndexAny(full, ":->."); position >= 0 {
		return full[position+1:], strings.ReplaceAll(full, "->", "::")
	}
	return name, ""
}
