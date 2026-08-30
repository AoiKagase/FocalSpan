package vb

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

var (
	vbImportsPattern    = regexp.MustCompile(`(?i)^imports\s+(.+)$`)
	vbInheritsPattern   = regexp.MustCompile(`(?i)^inherits\s+(.+)$`)
	vbImplementsPattern = regexp.MustCompile(`(?i)^implements\s+(.+)$`)
	vbWithEventsPattern = regexp.MustCompile(`(?i)\bwithevents\s+[a-z_][a-z0-9_]*\s+as\s+([a-z_][a-z0-9_.]*)`)
	vbHandlesPattern    = regexp.MustCompile(`(?i)\bhandles\s+(.+)$`)
	vbHandlerPattern    = regexp.MustCompile(`(?i)\b(?:addhandler|removehandler)\s+([^,]+),\s*addressof\s+([a-z_][a-z0-9_]*)`)
	vbRaisePattern      = regexp.MustCompile(`(?i)\braiseevent\s+([a-z_][a-z0-9_]*)`)
	vbCallPattern       = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*\(`)
)

func extractVB(ctx context.Context, file model.SourceFile, net bool) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	_, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed := parseVB6(file)
	if net {
		parsed = parseVBNet(file)
	}
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	for index := range parsed.Diagnostics {
		parsed.Diagnostics[index].FilePath = file.Path
	}
	result := model.Extraction{Diagnostics: append(diagnostics, parsed.Diagnostics...)}
	allocator := model.NewHandleAllocator()
	language := "vb6"
	source := "vb6-structural"
	if net {
		language = "vbnet"
		source = "vbnet-structural"
	}
	owner := parsed.Owner
	ownerSymbol := model.Symbol{Handle: allocator.Allocate("sym", file.Path, language, owner.Kind, owner.Qualified, model.NormalizeSignature(owner.Header)), FilePath: file.Path, Language: file.Language, Kind: owner.Kind, Name: owner.Name, QualifiedName: owner.Qualified, Signature: owner.Header, StartLine: 1, EndLine: vbLineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, ownerSymbol)
	result.Chunks = append(result.Chunks, vbSyntheticChunk(file, ownerSymbol, "module-outline", owner.Header))
	if parsed.LayoutEnd > 0 && !net {
		layoutLine := vbLineNumber(file.Content, parsed.LayoutEnd)
		if layoutLine < 1 {
			layoutLine = 1
		}
		result.Chunks = append(result.Chunks, vbSourceChunk(file, ownerSymbol, "form-layout", 0, parsed.LayoutEnd, 1, layoutLine))
	}
	byDecl := make(map[*vbDecl]model.Symbol, len(parsed.Decls))
	for _, decl := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		start, end := vbDeclSpan(parsed.Lines, decl)
		handle := allocator.Allocate("sym", file.Path, language, decl.Kind, decl.Qualified, model.NormalizeSignature(decl.Header))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .95}
		if decl.Parent != nil {
			if parent, ok := byDecl[decl.Parent]; ok {
				symbol.ParentHandle = parent.Handle
			}
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		if !strings.Contains(strings.ToLower(string(file.Content[start:end])), ".frx") {
			result.Chunks = append(result.Chunks, vbSourceChunk(file, symbol, decl.Kind, start, end, decl.StartLine, decl.EndLine))
		}
	}
	buildVBRelations(&result, file, parsed, byDecl, ownerSymbol, source, net)
	sortVB(&result)
	return result, nil
}

func vbDeclSpan(lines []vbLine, decl *vbDecl) (int, int) {
	start := 0
	end := 0
	if decl.StartLine > 0 && decl.StartLine <= len(lines) {
		start = lines[decl.StartLine-1].Start
	}
	if decl.EndLine > 0 && decl.EndLine <= len(lines) {
		end = lines[decl.EndLine-1].End
	}
	if end < start {
		end = start
	}
	return start, end
}

func buildVBRelations(result *model.Extraction, file model.SourceFile, parsed vbParseResult, byDecl map[*vbDecl]model.Symbol, owner model.Symbol, source string, net bool) {
	symbols := make([]model.Symbol, 0, len(byDecl)+1)
	symbols = append(symbols, owner)
	for _, decl := range parsed.Decls {
		symbols = append(symbols, byDecl[decl])
		parent := owner
		if decl.Parent != nil {
			parent = byDecl[decl.Parent]
		}
		addVBRelation(result, model.Relation{FromHandle: parent.Handle, ToHandle: byDecl[decl].Handle, Kind: "contains", Confidence: .9, Source: source})
	}
	index := vbIndex(symbols)
	for lineIndex, line := range parsed.Lines {
		text := strings.TrimSpace(vbCode(line.Text))
		if text == "" || strings.HasPrefix(text, "'") {
			continue
		}
		from := owner
		if current := currentVBDecl(parsed.Decls, lineIndex+1); current != nil {
			from = byDecl[current]
		}
		lower := strings.ToLower(text)
		if match := vbImportsPattern.FindStringSubmatch(text); len(match) == 2 {
			for _, target := range splitVBTargets(match[1]) {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: source})
			}
		}
		if match := vbInheritsPattern.FindStringSubmatch(text); len(match) == 2 {
			addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: strings.TrimSpace(match[1]), Kind: "references", Confidence: .9, Source: source})
		}
		if match := vbImplementsPattern.FindStringSubmatch(text); len(match) == 2 {
			for _, target := range splitVBTargets(match[1]) {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: target, Kind: "references", Confidence: .9, Source: source})
			}
		}
		if match := vbWithEventsPattern.FindStringSubmatch(text); len(match) == 2 {
			addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "references", Confidence: .8, Source: source})
		}
		if match := vbHandlesPattern.FindStringSubmatch(text); len(match) == 2 {
			for _, target := range splitVBTargets(match[1]) {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: strings.TrimSpace(target), Kind: "references", Confidence: .9, Source: source})
			}
		}
		if match := vbHandlerPattern.FindStringSubmatch(text); len(match) == 3 {
			addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: strings.TrimSpace(match[1]), Kind: "references", Confidence: .85, Source: source})
			if target, ok := vbUnique(index, match[2]); ok {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: "references", Confidence: .85, Source: source})
			} else {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[2], Kind: "references", Confidence: .4, Source: source})
			}
		}
		if match := vbRaisePattern.FindStringSubmatch(text); len(match) == 2 {
			addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[1], Kind: "references", Confidence: .8, Source: source})
		}
		if strings.HasPrefix(lower, "imports ") || strings.HasPrefix(lower, "inherits ") || strings.HasPrefix(lower, "implements ") || strings.HasPrefix(lower, "addhandler ") || strings.HasPrefix(lower, "removehandler ") {
			continue
		}
		for _, match := range vbCallPattern.FindAllStringSubmatch(text, -1) {
			name := match[1]
			if vbIgnoredCall(name) || vbLooksLikeDeclaration(text, name) {
				continue
			}
			kind := "calls"
			if from.Kind == "test" {
				kind = "tests"
			}
			if target, ok := vbUnique(index, name); ok {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .85, Source: source})
			} else {
				addVBRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: source})
			}
		}
		_ = net
	}
}

func splitVBTargets(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func vbIgnoredCall(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "select", "catch", "typeof", "nameof", "new", "sub", "function", "property", "event", "raiseevent", "addhandler", "removehandler", "handles":
		return true
	}
	return false
}

func vbLooksLikeDeclaration(text, name string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, " sub "+strings.ToLower(name)+"(") || strings.Contains(lower, " function "+strings.ToLower(name)+"(") || strings.Contains(lower, "property "+strings.ToLower(name))
}

// Keep fmt imported in this file's generated-style source chunks stable when
// future relation fields are added; it also makes malformed input diagnostics
// easy to extend without changing the builder's public contract.
var _ = fmt.Sprintf
var _ = sort.SliceStable
