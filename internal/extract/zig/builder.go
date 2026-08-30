package zig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

var (
	zigTypeRefPattern = regexp.MustCompile(`(?i)(?::|\bof\b)\s*([a-z_][a-z0-9_]*)`)
	zigCallPattern    = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*\(`)
)

func buildZig(ctx context.Context, file model.SourceFile, parsed zigParseResult, diagnostics []model.Diagnostic) model.Extraction {
	result := model.Extraction{Diagnostics: diagnostics}
	allocator := model.NewHandleAllocator()
	owner := parsed.Owner
	ownerSymbol := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "zig", "module", owner.Qualified, owner.Header), FilePath: file.Path, Language: file.Language, Kind: "module", Name: owner.Name, QualifiedName: owner.Qualified, Signature: owner.Header, StartLine: 1, EndLine: zigLineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, ownerSymbol)
	result.Chunks = append(result.Chunks, zigSyntheticChunk(file, ownerSymbol, "module-outline", owner.Header))
	byDecl := make(map[*zigDecl]model.Symbol, len(parsed.Decls))
	for _, decl := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		start, end := zigDeclSpan(parsed.Lines, decl)
		symbol := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "zig", decl.Kind, decl.Qualified, model.NormalizeSignature(decl.Header)), FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .95}
		if decl.Parent != nil {
			symbol.ParentHandle = byDecl[decl.Parent].Handle
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		result.Chunks = append(result.Chunks, zigSourceChunk(file, symbol, decl.Kind, start, end))
	}
	buildZigRelations(&result, parsed, byDecl, ownerSymbol)
	sortZig(&result)
	return result
}

func zigDeclSpan(lines []zigLine, decl *zigDecl) (int, int) {
	start := lines[decl.StartLine-1].Start
	endLine := decl.EndLine
	if endLine < decl.StartLine || endLine > len(lines) {
		endLine = decl.StartLine
	}
	return start, lines[endLine-1].End
}

func buildZigRelations(result *model.Extraction, parsed zigParseResult, byDecl map[*zigDecl]model.Symbol, owner model.Symbol) {
	for _, decl := range parsed.Decls {
		parent := owner
		if decl.Parent != nil {
			parent = byDecl[decl.Parent]
		}
		addZigRelation(result, model.Relation{FromHandle: parent.Handle, ToHandle: byDecl[decl].Handle, Kind: "contains", Confidence: .9, Source: "zig-structural"})
	}
	symbols := []model.Symbol{owner}
	for _, decl := range parsed.Decls {
		symbols = append(symbols, byDecl[decl])
	}
	index := zigIndex(symbols)
	for lineIndex, line := range parsed.Lines {
		text := strings.TrimSpace(zigCode(line.Text))
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		from := owner
		if decl := zigCurrentDecl(parsed.Decls, lineIndex+1); decl != nil {
			from = byDecl[decl]
		}
		if match := zigImportPattern.FindStringSubmatch(text); len(match) == 2 {
			addZigRelation(result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: match[1], Kind: "imports", Confidence: .9, Source: "zig-import"})
		}
		for _, match := range zigTypeRefPattern.FindAllStringSubmatch(text, -1) {
			if !zigBuiltinType(match[1]) {
				addZigRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "references", Confidence: .55, Source: "zig-type"})
			}
		}
		for _, match := range zigCallPattern.FindAllStringSubmatch(text, -1) {
			name := match[1]
			if zigIgnoredCall(name) || zigDeclarationLine(text, name) {
				continue
			}
			kind := "calls"
			if from.Kind == "test" {
				kind = "tests"
			}
			if target, ok := zigUnique(index, name); ok {
				addZigRelation(result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .85, Source: "zig-call"})
			} else {
				addZigRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: "zig-call"})
			}
		}
	}
}

func zigCurrentDecl(decls []*zigDecl, line int) *zigDecl {
	var current *zigDecl
	for _, decl := range decls {
		if decl.StartLine <= line && line <= decl.EndLine && (current == nil || decl.StartLine >= current.StartLine) {
			current = decl
		}
	}
	return current
}

func zigDeclarationLine(text, name string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, " fn "+strings.ToLower(name)) || strings.HasPrefix(lower, "fn "+strings.ToLower(name)) || strings.HasPrefix(lower, "pub fn "+strings.ToLower(name)) || strings.HasPrefix(lower, "test ")
}

func zigIgnoredCall(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "switch", "catch", "orelse", "try", "return", "expect", "expectequal", "import", "enum", "union", "struct", "opaque":
		return true
	}
	return false
}

func zigBuiltinType(name string) bool {
	switch strings.ToLower(name) {
	case "u8", "u16", "u32", "u64", "usize", "i8", "i16", "i32", "i64", "isize", "bool", "void", "anytype", "anyerror":
		return true
	}
	return false
}

func zigIndex(symbols []model.Symbol) map[string][]model.Symbol {
	result := make(map[string][]model.Symbol)
	for _, symbol := range symbols {
		result[strings.ToLower(symbol.Name)] = append(result[strings.ToLower(symbol.Name)], symbol)
	}
	return result
}

func zigUnique(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := index[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func zigSourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), fmt.Sprint(end)), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: symbol.StartLine, EndLine: symbol.EndLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func zigSyntheticChunk(file model.SourceFile, symbol model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: "synthetic outline (not a source slice): " + symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func zigLineCount(source []byte) int { return 1 + strings.Count(string(source), "\n") }

func addZigRelation(result *model.Extraction, relation model.Relation) {
	if relation.FromHandle == "" || relation.FromHandle == relation.ToHandle || relation.ToHandle != "" && relation.UnresolvedTo != "" {
		return
	}
	for _, old := range result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	result.Relations = append(result.Relations, relation)
}

func sortZig(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		return result.Relations[i].Kind < result.Relations[j].Kind
	})
}
