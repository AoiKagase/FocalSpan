package jsts

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
func (Extractor) Name() string { return "jsts-structural" }
func (Extractor) Supports(path, language string) bool {
	if language == "javascript" || language == "typescript" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".d.ts":
		return true
	}
	return false
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	tokens, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed, err := parseJSTS(ctx, tokens)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed.Diagnostics = append(diagnostics, parsed.Diagnostics...)
	for index := range parsed.Diagnostics {
		parsed.Diagnostics[index].FilePath = file.Path
	}
	result := build(ctx, file, parsed)
	if len(parsed.Diagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: file.Path, Level: "warning", Code: "jsts_partial_extraction", Message: "JavaScript/TypeScript extraction recovered from malformed source"})
	}
	return result, nil
}

type symbolIndex struct {
	byQualified map[string][]model.Symbol
	byName      map[string][]model.Symbol
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
		if chosen[declarationKey(decl)] != decl || decl.Start < 0 || decl.Start >= len(parsed.Tokens) {
			continue
		}
		span := b.declSpan(decl)
		signature := b.text(decl.Start, decl.HeaderEnd)
		if strings.TrimSpace(signature) == "" {
			signature = decl.Name
		}
		parentHandle := ""
		if decl.Parent != nil {
			parentHandle = b.byDecl[decl.Parent].Handle
		}
		handle := b.allocator.Allocate("sym", file.Path, "jsts", decl.Kind, decl.Qualified, model.NormalizeSignature(signature))
		confidence := 1.0
		if span.EndByte <= span.StartByte {
			confidence = .55
		}
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: strings.TrimSpace(signature), StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, ParentHandle: parentHandle, Confidence: confidence}
		b.byDecl[decl] = symbol
		b.result.Symbols = append(b.result.Symbols, symbol)
	}
	for _, decl := range parsed.Declarations {
		if chosen[declarationKey(decl)] != decl {
			continue
		}
		symbol, ok := b.byDecl[decl]
		if !ok {
			continue
		}
		b.addChunk(decl, symbol)
		if decl.Parent != nil {
			if parent, exists := b.byDecl[decl.Parent]; exists {
				b.addRelation(model.Relation{FromHandle: parent.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "jsts-structural"})
			}
		} else {
			b.addRelation(model.Relation{FromHandle: b.owner.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: symbol.Confidence, Source: "jsts-structural"})
		}
	}
	b.buildRelations()
	b.sortResult()
	return b.result
}

func (b *builder) makeOwner() model.Symbol {
	handle := b.allocator.Allocate("sym", b.file.Path, "jsts", "module", b.file.Path)
	return model.Symbol{Handle: handle, FilePath: b.file.Path, Language: b.file.Language, Kind: "module", Name: b.file.Path, QualifiedName: b.file.Path, StartLine: 1, EndLine: b.mapa.LineCount(), StartByte: 0, EndByte: len(b.file.Content), Confidence: .7}
}
func (b *builder) addOwnerChunk() {
	lines := []string{"// FocalSpan synthetic module outline: " + b.file.Path}
	for _, item := range b.parsed.Modules {
		lines = append(lines, "module: "+item.Source+" "+item.Target)
	}
	for _, decl := range b.parsed.Declarations {
		if decl.Parent == nil {
			lines = append(lines, decl.Kind+": "+decl.Qualified)
		}
	}
	content := strings.Join(lines, "\n")
	digest := sha256.Sum256([]byte(content))
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", b.owner.Handle, "module-outline", content), FilePath: b.file.Path, Language: b.file.Language, Kind: "module-outline", SymbolHandle: b.owner.Handle, SymbolName: b.file.Path, Signature: "synthetic outline (not a source slice)", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 0, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}
func (b *builder) addChunk(decl *declaration, symbol model.Symbol) {
	end := decl.End
	kind := symbol.Kind
	if decl.Kind == "class" || decl.Kind == "interface" || decl.Kind == "enum" || decl.Kind == "namespace" || decl.Kind == "type" {
		end = decl.HeaderEnd
		kind += "-outline"
	}
	span := b.declSpanBytes(decl.Start, end)
	content, ok := b.mapa.Slice(span)
	if !ok || span.EndByte <= span.StartByte || strings.TrimSpace(content) == "" {
		return
	}
	digest := sha256.Sum256([]byte(content))
	b.result.Chunks = append(b.result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(span.StartByte), fmt.Sprint(span.EndByte), content), FilePath: b.file.Path, Language: b.file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, Content: content, ContentHash: hex.EncodeToString(digest[:])})
}

func (b *builder) buildRelations() {
	index := symbolIndex{byQualified: make(map[string][]model.Symbol), byName: make(map[string][]model.Symbol)}
	for _, symbol := range b.result.Symbols {
		if symbol.Handle != b.owner.Handle {
			index.byQualified[strings.ToLower(symbol.QualifiedName)] = append(index.byQualified[strings.ToLower(symbol.QualifiedName)], symbol)
			index.byName[strings.ToLower(symbol.Name)] = append(index.byName[strings.ToLower(symbol.Name)], symbol)
		}
	}
	for _, module := range b.parsed.Modules {
		for _, target := range normalizeModule(b.file.Path, module.Target) {
			b.addRelation(model.Relation{FromHandle: b.owner.Handle, UnresolvedTo: target, Kind: moduleKind(module.Source), Confidence: .75, Source: module.Source})
		}
	}
	for decl, symbol := range b.byDecl {
		b.buildReferences(index, decl, symbol)
		if decl.BodyOpen >= 0 && (decl.Kind == "function" || decl.Kind == "function-expression" || decl.Kind == "arrow_function" || decl.Kind == "method" || decl.Kind == "test") {
			b.buildBodyRelations(index, decl, symbol)
		}
	}
}
func moduleKind(source string) string {
	if strings.Contains(source, "export") {
		return "exports"
	}
	return "imports"
}
func (b *builder) buildReferences(index symbolIndex, decl *declaration, from model.Symbol) {
	if decl.Kind == "test" {
		return
	}
	for _, token := range b.parsed.Tokens {
		if token.StartByte < b.byteStart(decl) || token.StartByte >= decl.HeaderEnd || !token.significant() || token.Kind != Identifier {
			continue
		}
		name := strings.TrimPrefix(token.Text, "#")
		if name == decl.Name || primitive[name] || name == "export" || name == "default" || name == "from" {
			continue
		}
		if len(index.byName[strings.ToLower(name)]) > 0 || looksLikeType(name) {
			b.addResolvedOrUnresolved(index, from, name, "references", "type")
		}
	}
}
func (b *builder) buildBodyRelations(index symbolIndex, decl *declaration, from model.Symbol) {
	for position := decl.BodyOpen + 1; position < decl.BodyClose; position++ {
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
		if name == "" || control[name] || name == "function" || name == "import" || name == "require" {
			continue
		}
		kind := "calls"
		if from.Kind == "test" {
			kind = "tests"
		}
		if target, ok := b.resolveCall(index, decl, name, qualified); ok {
			b.addRelation(model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .95, Source: "jsts-call"})
		} else {
			lexical := qualified
			if lexical == "" {
				lexical = name
			}
			b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: lexical, Kind: kind, Confidence: .25, Source: "jsts-call"})
			for _, module := range b.parsed.Modules {
				if module.Source == "jsts:import-symbol" && module.Alias == name && module.Target != name {
					b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: module.Target, Kind: kind, Confidence: .2, Source: "jsts-import-alias-call"})
				}
			}
		}
	}
}
func (b *builder) resolveCall(index symbolIndex, decl *declaration, name, qualified string) (model.Symbol, bool) {
	if target, ok := unique(index.byQualified, strings.ToLower(qualified)); ok {
		return target, true
	}
	if decl.Parent != nil && decl.Parent.Kind == "class" {
		if target, ok := unique(index.byQualified, strings.ToLower(decl.Parent.Qualified+"."+name)); ok {
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
	if looksLikeType(name) {
		b.addRelation(model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .3, Source: source})
	}
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
func (b *builder) text(start, end int) string {
	value, _ := b.mapa.Slice(b.declSpanBytes(start, end))
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

func declarationKey(decl *declaration) string {
	return decl.Kind + "\x00" + strings.ToLower(decl.Qualified) + "\x00" + decl.Name + "\x00" + model.NormalizeSignature(decl.SignatureKey)
}
func preferredDeclarations(declarations []*declaration) map[string]*declaration {
	result := make(map[string]*declaration)
	for _, decl := range declarations {
		key := declarationKey(decl)
		old := result[key]
		if old == nil || old.BodyOpen < 0 && decl.BodyOpen >= 0 {
			result[key] = decl
		}
	}
	return result
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
func callName(tokens []Token, previous int) (string, string) {
	if previous < 0 || tokens[previous].Kind != Identifier {
		return "", ""
	}
	name := tokens[previous].Text
	parts := []string{name}
	for position := previous - 1; position >= 0; {
		if !tokens[position].significant() {
			position--
			continue
		}
		if tokens[position].Text != "." && tokens[position].Text != "?." {
			break
		}
		left := previousSignificant(tokens, position)
		if left < 0 || tokens[left].Kind != Identifier {
			break
		}
		parts = append([]string{tokens[left].Text, "."}, parts...)
		position = left - 1
	}
	full := strings.Join(parts, "")
	if dot := strings.LastIndexByte(full, '.'); dot >= 0 {
		return full[dot+1:], full
	}
	return name, ""
}

var primitive = map[string]bool{"string": true, "number": true, "boolean": true, "any": true, "unknown": true, "void": true, "never": true, "object": true}
var control = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "typeof": true, "with": true}

func looksLikeType(name string) bool {
	return name != "" && (name[0] >= 'A' && name[0] <= 'Z' || strings.Contains(name, "."))
}
