package lua

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

func buildLua(ctx context.Context, file model.SourceFile, tokens []Token, diagnostics []model.Diagnostic) (model.Extraction, error) {
	lines := luaLines(file.Content, tokens)
	result := model.Extraction{Diagnostics: diagnostics}
	allocator := model.NewHandleAllocator()
	ownerName := filepath.Base(filepath.ToSlash(file.Path))
	owner := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "lua", "module", ownerName), FilePath: file.Path, Language: file.Language, Kind: "module", Name: ownerName, QualifiedName: ownerName, Signature: "module " + ownerName, StartLine: 1, EndLine: luaLineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	result.Chunks = append(result.Chunks, luaSyntheticChunk(owner, file.Path, file.Language))

	declarations := parseLua(lines)
	byDecl := make(map[*luaDeclaration]model.Symbol, len(declarations))
	byName := make(map[string][]model.Symbol)
	for _, decl := range declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		start := lines[decl.StartLine-1].Start
		end := lines[decl.EndLine-1].End
		handle := allocator.Allocate("sym", file.Path, "lua", decl.Kind, decl.Qualified, model.NormalizeSignature(decl.Header))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .9}
		if decl.Parent != nil {
			symbol.ParentHandle = byDecl[decl.Parent].Handle
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		byName[strings.ToLower(symbol.Name)] = append(byName[strings.ToLower(symbol.Name)], symbol)
		chunkEnd := end
		chunkKind := decl.Kind
		if decl.Kind == "table" {
			chunkEnd = lines[decl.StartLine-1].End
			chunkKind = "table-outline"
		}
		result.Chunks = append(result.Chunks, luaSourceChunk(file, symbol, chunkKind, start, chunkEnd, decl.StartLine, decl.EndLine))
	}

	for lineIndex, line := range lines {
		trimmed := strings.TrimSpace(line.Raw)
		if luaRequireTarget(trimmed) != "" {
			result.Chunks = append(result.Chunks, luaSourceChunk(file, owner, "import", line.Start, line.End, lineIndex+1, lineIndex+1))
			addLuaRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: luaRequireTarget(trimmed), Kind: "imports", Confidence: .9, Source: "lua-require"})
		}
		if target := luaSetmetatableTarget(trimmed); target != "" {
			addLuaRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "references", Confidence: .35, Source: "lua-setmetatable"})
		}
	}
	for _, decl := range declarations {
		from := byDecl[decl]
		addLuaRelation(&result, model.Relation{FromHandle: owner.Handle, ToHandle: from.Handle, Kind: "contains", Confidence: from.Confidence, Source: "lua-structural"})
		for line := decl.StartLine; line <= decl.EndLine && line <= len(lines); line++ {
			for _, match := range luaCall.FindAllStringSubmatch(lines[line-1].Code, -1) {
				name := match[1]
				if luaIgnoredCall(name) {
					continue
				}
				kind := "calls"
				if decl.Kind == "test" {
					kind = "tests"
				}
				if target, ok := luaUnique(byName, name); ok {
					addLuaRelation(&result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .8, Source: "lua-call"})
				} else if decl.Kind == "test" || strings.Contains(name, ".") || strings.Contains(name, ":") {
					addLuaRelation(&result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: "lua-call"})
				}
			}
		}
	}
	sortLua(&result)
	return result, nil
}

func luaRequireTarget(line string) string {
	marker := strings.Index(line, "require")
	if marker < 0 || (marker > 0 && isLuaNameByte(line[marker-1])) || marker+len("require") < len(line) && isLuaNameByte(line[marker+len("require")]) {
		return ""
	}
	start := strings.IndexAny(line[marker+len("require"):], "\"'")
	if start < 0 {
		return ""
	}
	start += marker + len("require")
	rest := line[start+1:]
	if end := strings.IndexAny(rest, "\"'"); end >= 0 {
		return rest[:end]
	}
	return ""
}

func luaSetmetatableTarget(line string) string {
	marker := strings.Index(line, "__index")
	if marker < 0 {
		return ""
	}
	rest := line[marker+len("__index"):]
	if equal := strings.IndexByte(rest, '='); equal >= 0 {
		rest = strings.TrimSpace(rest[equal+1:])
		end := 0
		for end < len(rest) && (isLuaNameByte(rest[end]) || rest[end] == '.' || rest[end] == ':') {
			end++
		}
		return rest[:end]
	}
	return ""
}

func isLuaNameByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func luaIgnoredCall(name string) bool {
	last := strings.ToLower(name)
	if index := strings.LastIndexAny(last, ".:"); index >= 0 {
		last = last[index+1:]
	}
	switch last {
	case "assert", "describe", "do", "function", "if", "it", "pairs", "pcall", "require", "setmetatable", "specify", "tostring", "type":
		return true
	}
	return false
}

func luaUnique(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	if at := strings.LastIndexAny(name, ".:"); at >= 0 {
		name = name[at+1:]
	}
	items := index[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func addLuaRelation(result *model.Extraction, relation model.Relation) {
	if relation.FromHandle == "" || relation.FromHandle == relation.ToHandle {
		return
	}
	for _, old := range result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	result.Relations = append(result.Relations, relation)
}

func luaSyntheticChunk(symbol model.Symbol, path, language string) model.Chunk {
	content := symbol.Signature
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, "module-outline", content), FilePath: path, Language: language, Kind: "module-outline", SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func luaSourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end, startLine, endLine int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func luaLineCount(source []byte) int { return 1 + strings.Count(string(source), "\n") }

func sortLua(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		if result.Relations[i].Kind != result.Relations[j].Kind {
			return result.Relations[i].Kind < result.Relations[j].Kind
		}
		return result.Relations[i].UnresolvedTo < result.Relations[j].UnresolvedTo
	})
}
