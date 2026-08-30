package nim

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
	nimImportPattern       = regexp.MustCompile(`(?i)^(?:import|include)\s+(.+)$`)
	nimFromPattern         = regexp.MustCompile(`(?i)^from\s+([^\s]+)\s+import\s+(.+)$`)
	nimObjectParentPattern = regexp.MustCompile(`(?i)=\s*object\s+of\s+([a-z_][a-z0-9_.]*)`)
	nimTypePattern         = regexp.MustCompile(`(?i)\b(?:as|:)\s*([a-z_][a-z0-9_.]*)`)
	nimCallPattern         = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*\(`)
)

func buildNim(ctx context.Context, file model.SourceFile, parsed nimParseResult, diagnostics []model.Diagnostic) model.Extraction {
	result := model.Extraction{Diagnostics: diagnostics}
	allocator := model.NewHandleAllocator()
	owner := parsed.Owner
	if owner == nil {
		owner = nimFileOwner(file)
	}
	ownerSymbol := nimSymbol(allocator, file, owner, "module")
	result.Symbols = append(result.Symbols, ownerSymbol)
	result.Chunks = append(result.Chunks, nimSyntheticChunk(file, ownerSymbol, "module-outline", owner.Header))
	byDecl := make(map[*nimDecl]model.Symbol, len(parsed.Decls))
	for _, decl := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		start, end := nimDeclSpan(parsed.Lines, decl)
		symbol := nimSymbol(allocator, file, decl, "")
		symbol.StartByte, symbol.EndByte = start, end
		if decl.Parent != nil {
			if parent, ok := byDecl[decl.Parent]; ok {
				symbol.ParentHandle = parent.Handle
			}
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		result.Chunks = append(result.Chunks, nimSourceChunk(file, symbol, decl.Kind, start, end))
	}
	buildNimRelations(&result, file, parsed, byDecl, ownerSymbol)
	sortNim(&result)
	return result
}

func nimSymbol(allocator *model.HandleAllocator, file model.SourceFile, decl *nimDecl, kindOverride string) model.Symbol {
	kind := decl.Kind
	if kindOverride != "" {
		kind = kindOverride
	}
	return model.Symbol{Handle: allocator.Allocate("sym", file.Path, "nim", kind, decl.Qualified, model.NormalizeSignature(decl.Header)), FilePath: file.Path, Language: file.Language, Kind: kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, Confidence: .95}
}

func nimDeclSpan(lines []nimLine, decl *nimDecl) (int, int) {
	if len(lines) == 0 || decl.StartLine < 1 || decl.StartLine > len(lines) {
		return 0, 0
	}
	start := lines[decl.StartLine-1].Start
	endLine := decl.EndLine
	if endLine < decl.StartLine || endLine > len(lines) {
		endLine = decl.StartLine
	}
	return start, lines[endLine-1].End
}

func buildNimRelations(result *model.Extraction, file model.SourceFile, parsed nimParseResult, byDecl map[*nimDecl]model.Symbol, owner model.Symbol) {
	for _, decl := range parsed.Decls {
		parent := owner
		if decl.Parent != nil {
			parent = byDecl[decl.Parent]
		}
		addNimRelation(result, model.Relation{FromHandle: parent.Handle, ToHandle: byDecl[decl].Handle, Kind: "contains", Confidence: .9, Source: "nim-structural"})
	}
	symbols := []model.Symbol{owner}
	for _, decl := range parsed.Decls {
		symbols = append(symbols, byDecl[decl])
	}
	index := nimIndex(symbols)
	for lineIndex, line := range parsed.Lines {
		text := strings.TrimSpace(nimCode(line.Text))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		from := owner
		if decl := nimCurrentDecl(parsed.Decls, lineIndex+1); decl != nil {
			from = byDecl[decl]
		}
		if match := nimImportPattern.FindStringSubmatch(text); len(match) == 2 {
			for _, target := range strings.Split(match[1], ",") {
				target = strings.TrimSpace(target)
				if target != "" {
					addNimRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "nim-import"})
				}
			}
		}
		if match := nimFromPattern.FindStringSubmatch(text); len(match) == 3 {
			addNimRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "imports", Confidence: .9, Source: "nim-from"})
		}
		if match := nimObjectParentPattern.FindStringSubmatch(text); len(match) == 2 {
			addNimRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "references", Confidence: .85, Source: "nim-inheritance"})
		}
		for _, match := range nimTypePattern.FindAllStringSubmatch(text, -1) {
			if !nimBuiltinType(match[1]) {
				addNimRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "references", Confidence: .55, Source: "nim-type"})
			}
		}
		for _, match := range nimCallPattern.FindAllStringSubmatch(text, -1) {
			name := match[1]
			if nimIgnoredCall(name) || strings.HasPrefix(strings.ToLower(text), strings.ToLower(name)+" ") && !strings.Contains(text, "(") {
				continue
			}
			kind := "calls"
			if from.Kind == "test" {
				kind = "tests"
			}
			if target, ok := nimUnique(index, name); ok {
				addNimRelation(result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .85, Source: "nim-call"})
			} else {
				addNimRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: "nim-call"})
			}
		}
	}
}

func nimCurrentDecl(decls []*nimDecl, line int) *nimDecl {
	var current *nimDecl
	for _, decl := range decls {
		if decl.StartLine <= line && line <= decl.EndLine && (current == nil || decl.Indent >= current.Indent) {
			current = decl
		}
	}
	return current
}

func nimIndex(symbols []model.Symbol) map[string][]model.Symbol {
	result := make(map[string][]model.Symbol)
	for _, symbol := range symbols {
		result[strings.ToLower(symbol.Name)] = append(result[strings.ToLower(symbol.Name)], symbol)
	}
	return result
}

func nimUnique(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := index[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func nimBuiltinType(name string) bool {
	switch strings.ToLower(name) {
	case "string", "bool", "char", "int", "int8", "int16", "int32", "int64", "uint", "float", "float32", "float64", "void", "auto", "untyped":
		return true
	}
	return false
}

func nimIgnoredCall(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "case", "echo", "check", "require", "assert", "new", "suite", "test", "strip", "len":
		return true
	}
	return false
}

func nimSourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), fmt.Sprint(end)), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: symbol.StartLine, EndLine: symbol.EndLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func nimSyntheticChunk(file model.SourceFile, symbol model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: "synthetic outline (not a source slice): " + symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func addNimRelation(result *model.Extraction, relation model.Relation) {
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

func sortNim(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		return result.Relations[i].Kind < result.Relations[j].Kind
	})
}
