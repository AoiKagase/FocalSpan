package pawn

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

func buildPawn(ctx context.Context, file model.SourceFile, tokens []Token, diagnostics []model.Diagnostic) (model.Extraction, error) {
	lines := pawnLines(file.Content, tokens)
	result := model.Extraction{Diagnostics: diagnostics}
	allocator := model.NewHandleAllocator()
	ownerName := filepath.Base(filepath.ToSlash(file.Path))
	owner := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "pawn", "pawn_unit", ownerName), FilePath: file.Path, Language: file.Language, Kind: "pawn_unit", Name: ownerName, QualifiedName: ownerName, Signature: "pawn_unit " + ownerName, StartLine: 1, EndLine: pawnLineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	result.Chunks = append(result.Chunks, pawnSyntheticChunk(owner, file.Path, file.Language))

	declarations := parsePawn(lines)
	byDecl := make(map[*pawnDeclaration]model.Symbol, len(declarations))
	byName := make(map[string][]model.Symbol)
	for _, decl := range declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		start := lines[decl.StartLine-1].Start
		end := lines[decl.EndLine-1].End
		handle := allocator.Allocate("sym", file.Path, "pawn", decl.Kind, decl.Qualified, model.NormalizeSignature(decl.Header))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .9}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		if decl.Kind != "forward" && decl.Kind != "native" {
			byName[strings.ToLower(symbol.Name)] = append(byName[strings.ToLower(symbol.Name)], symbol)
		}
		result.Chunks = append(result.Chunks, pawnSourceChunk(file, symbol, decl.Kind, start, end, decl.StartLine, decl.EndLine))
	}
	for lineIndex, line := range lines {
		trimmed := strings.TrimSpace(line.Raw)
		if strings.HasPrefix(trimmed, "#include") {
			if target := pawnIncludeTarget(trimmed); target != "" {
				result.Chunks = append(result.Chunks, pawnSourceChunk(file, owner, "import", line.Start, line.End, lineIndex+1, lineIndex+1))
				addPawnRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "pawn-include"})
			}
		}
	}
	for _, decl := range declarations {
		from := byDecl[decl]
		addPawnRelation(&result, model.Relation{FromHandle: owner.Handle, ToHandle: from.Handle, Kind: "contains", Confidence: from.Confidence, Source: "pawn-structural"})
		for line := decl.StartLine; line <= decl.EndLine && line <= len(lines); line++ {
			text := lines[line-1].Code
			if match := pawnHandler.FindStringSubmatch(lines[line-1].Raw); len(match) == 2 {
				if target, ok := pawnUnique(byName, match[1]); ok {
					addPawnRelation(&result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: "references", Confidence: .9, Source: "pawn-handler"})
				}
			}
			for _, match := range pawnCall.FindAllStringSubmatch(text, -1) {
				name := match[1]
				if pawnIgnoredCall(name) {
					continue
				}
				if target, ok := pawnUnique(byName, name); ok {
					addPawnRelation(&result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: "calls", Confidence: .8, Source: "pawn-call"})
				}
			}
		}
	}
	sortPawn(&result)
	return result, nil
}

func pawnIncludeTarget(line string) string {
	start := strings.IndexAny(line, "<\"")
	if start < 0 {
		return ""
	}
	endChar := byte('>')
	if line[start] == '"' {
		endChar = '"'
	}
	if end := strings.IndexByte(line[start+1:], endChar); end >= 0 {
		return line[start+1 : start+1+end]
	}
	return ""
}

func pawnUnique(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := index[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

func pawnIgnoredCall(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "switch", "sizeof", "register_plugin", "register_clcmd", "register_concmd", "register_event", "register_logevent", "set_task", "client_print":
		return true
	}
	return false
}

func addPawnRelation(result *model.Extraction, relation model.Relation) {
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

func pawnSyntheticChunk(symbol model.Symbol, path, language string) model.Chunk {
	content := symbol.Signature
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, "pawn-unit-outline", content), FilePath: path, Language: language, Kind: "pawn-unit-outline", SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func pawnSourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end, startLine, endLine int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func pawnLineCount(source []byte) int { return 1 + strings.Count(string(source), "\n") }

func sortPawn(result *model.Extraction) {
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
