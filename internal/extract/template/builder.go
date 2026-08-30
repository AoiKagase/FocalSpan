package template

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type templateBuilder struct {
	ctx       context.Context
	file      model.SourceFile
	regions   []Region
	parsed    parseResult
	allocator *model.HandleAllocator
	result    model.Extraction
	root      model.Symbol
	nodeSyms  map[int]model.Symbol
}

func build(ctx context.Context, file model.SourceFile, regions []Region, parsed parseResult) (model.Extraction, error) {
	b := &templateBuilder{ctx: ctx, file: file, regions: regions, parsed: parsed, allocator: model.NewHandleAllocator(), nodeSyms: make(map[int]model.Symbol)}
	b.result.Diagnostics = append(b.result.Diagnostics, parsed.Diagnostics...)
	b.root = b.makeRoot()
	b.result.Symbols = append(b.result.Symbols, b.root)
	b.appendRootOutline()
	if err := b.appendNamedSymbols(); err != nil {
		return model.Extraction{}, err
	}
	if err := b.appendEmbedded(); err != nil {
		return model.Extraction{}, err
	}
	b.appendStaticChunks()
	b.appendRelations()
	b.sortResult()
	return b.result, nil
}

func (b *templateBuilder) makeRoot() model.Symbol {
	qualified := strings.ReplaceAll(b.file.Path, "\\", "/")
	handle := b.allocator.Allocate("sym", qualified, "template", "template", qualified, qualified)
	return model.Symbol{Handle: handle, FilePath: b.file.Path, Language: b.file.Language, Kind: "template", Name: b.file.Path, QualifiedName: qualified, Signature: "template " + qualified, StartLine: 1, EndLine: lineCount(b.file.Content), StartByte: 0, EndByte: len(b.file.Content), Confidence: .9}
}

func (b *templateBuilder) appendRootOutline() {
	lines := []string{"template " + b.file.Path}
	for _, tag := range b.parsed.Tags {
		if tag.Closing {
			continue
		}
		switch tag.Name {
		case "extends", "include":
			target := tagTargetFor(b.file.Path, tag)
			if target != "" {
				label := tag.Name + " " + target
				lines = append(lines, label)
			}
		}
	}
	for _, node := range b.parsed.Nodes {
		name := nodeName(node.Tag)
		if name != "" {
			lines = append(lines, node.Tag.Name+" "+name)
		}
	}
	for _, region := range b.regions {
		if region.Kind != KindScriptOpen {
			continue
		}
		if src := htmlAttributes(region.Content)["src"]; src != "" {
			lines = append(lines, "script "+src)
		}
	}
	content := strings.Join(lines, "\n")
	chunk := b.newChunk("template-outline", b.root.Handle, b.root.Name, 0, 0, content)
	chunk.Signature = "synthetic outline (not a source slice)"
	b.result.Chunks = append(b.result.Chunks, chunk)
}

func (b *templateBuilder) appendNamedSymbols() error {
	for index := range b.parsed.Nodes {
		if index&63 == 0 {
			if err := b.ctx.Err(); err != nil {
				return err
			}
		}
		node := &b.parsed.Nodes[index]
		name := nodeName(node.Tag)
		if name == "" {
			continue
		}
		qualified := b.file.Path + "::" + node.Tag.Name + "::" + name
		handle := b.allocator.Allocate("sym", b.file.Path, "template", "template-"+node.Tag.Name, qualified, model.NormalizeSignature(string(node.Tag.Region.Content)))
		start, end := node.Tag.Region.StartByte, node.EndByte
		startLine, endLine := lineRange(b.file.Content, start, end)
		symbol := model.Symbol{Handle: handle, FilePath: b.file.Path, Language: b.file.Language, Kind: "template-" + node.Tag.Name, Name: name, QualifiedName: qualified, Signature: strings.TrimSpace(string(node.Tag.Region.Content)), StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Confidence: 1}
		parent := node.Parent
		for parent >= 0 {
			if parentSymbol, ok := b.nodeSyms[parent]; ok {
				symbol.ParentHandle = parentSymbol.Handle
				break
			}
			parent = b.parsed.Nodes[parent].Parent
		}
		b.nodeSyms[index] = symbol
		b.result.Symbols = append(b.result.Symbols, symbol)
		content := string(b.file.Content[start:end])
		if strings.TrimSpace(content) != "" {
			b.result.Chunks = append(b.result.Chunks, b.newChunk("template-"+node.Tag.Name, handle, name, start, end, content))
		}
	}
	return nil
}

func (b *templateBuilder) appendEmbedded() error {
	ordinal := 0
	scriptOrdinal := 0
	for _, region := range b.regions {
		if region.Kind != KindScriptBody && region.Kind != KindDataScript && region.Kind != KindStyleBody && region.Kind != KindSmartyLiteral && region.Kind != KindPHPBlock || region.EndByte <= region.StartByte {
			continue
		}
		if err := b.ctx.Err(); err != nil {
			return err
		}
		ordinal++
		if region.Kind == KindStyleBody {
			b.appendBoundedChunk("style", "", ordinal, region.StartByte, region.EndByte)
			continue
		}
		if region.Kind == KindDataScript {
			b.appendBoundedChunk("data-script", "", ordinal, region.StartByte, region.EndByte)
			continue
		}
		if region.Kind == KindSmartyLiteral {
			b.appendBoundedChunk("literal", "", ordinal, region.StartByte, region.EndByte)
			continue
		}
		if region.Kind == KindPHPBlock {
			b.appendBoundedChunk("embedded-php", "", ordinal, region.StartByte, region.EndByte)
			continue
		}
		scriptOrdinal++
		attrs := htmlAttributes(b.scriptOpenFor(region).Content)
		language := scriptLanguage(attrs)
		parent := b.ownerForOffset(region.StartByte)
		contextName := fmt.Sprintf("%d", scriptOrdinal)
		if id := attrs["id"]; id != "" {
			contextName = "id-" + id
		}
		if err := appendEmbeddedScript(b.ctx, b.file, region, language, contextName, parent, b.allocator, &b.result); err != nil {
			return err
		}
	}
	return nil
}

func (b *templateBuilder) scriptOpenFor(body Region) Region {
	for index := range b.regions {
		if b.regions[index].Kind == KindScriptOpen && b.regions[index].EndByte <= body.StartByte {
			if index+1 < len(b.regions) && b.regions[index+1].StartByte == body.StartByte {
				return b.regions[index]
			}
		}
	}
	return Region{}
}

func scriptLanguage(attrs map[string]string) string {
	typeValue := strings.ToLower(attrs["type"])
	if typeValue == "text/typescript" || strings.EqualFold(attrs["lang"], "ts") || strings.EqualFold(attrs["lang"], "typescript") {
		return "typescript"
	}
	return "javascript"
}

func (b *templateBuilder) appendStaticChunks() {
	intervals := make([][2]int, 0, len(b.parsed.Nodes)+len(b.regions))
	for _, node := range b.parsed.Nodes {
		intervals = append(intervals, [2]int{node.Tag.Region.StartByte, node.EndByte})
	}
	for _, region := range b.regions {
		switch region.Kind {
		case KindStatic, KindTemplateTag, KindSmartyTag, KindSmartyVar:
			continue
		default:
			intervals = append(intervals, [2]int{region.StartByte, region.EndByte})
		}
	}
	intervals = mergeIntervals(intervals, len(b.file.Content))
	start := 0
	for _, interval := range intervals {
		b.appendStaticRange(start, interval[0])
		if interval[1] > start {
			start = interval[1]
		}
	}
	b.appendStaticRange(start, len(b.file.Content))
}

func (b *templateBuilder) appendStaticRange(start, end int) {
	if end <= start || strings.TrimSpace(string(b.file.Content[start:end])) == "" {
		return
	}
	for start < end {
		windowEnd := end
		line := 0
		for index := start; index < end; index++ {
			if b.file.Content[index] == '\n' {
				line++
				if line >= 80 {
					windowEnd = index + 1
					break
				}
			}
		}
		content := string(b.file.Content[start:windowEnd])
		if strings.TrimSpace(content) != "" {
			owner := b.ownerForOffset(start)
			b.result.Chunks = append(b.result.Chunks, b.newChunk("static", owner.Handle, owner.Name, start, windowEnd, content))
		}
		if windowEnd >= end {
			break
		}
		start = windowEnd
	}
}

func (b *templateBuilder) appendBoundedChunk(kind, contextName string, ordinal, start, end int) {
	if end <= start || strings.TrimSpace(string(b.file.Content[start:end])) == "" {
		return
	}
	owner := b.ownerForOffset(start)
	symbolHandle := owner.Handle
	content := string(b.file.Content[start:end])
	chunkKind := kind
	if contextName != "" {
		chunkKind += "-" + contextName
	}
	chunk := b.newChunk(chunkKind, symbolHandle, "", start, end, content)
	chunk.Handle = model.StableHandle("chunk", b.file.Path, kind, fmt.Sprintf("%d", ordinal), content)
	b.result.Chunks = append(b.result.Chunks, chunk)
}

func (b *templateBuilder) appendRelations() {
	for index, symbol := range b.nodeSyms {
		parent := b.parsed.Nodes[index].Parent
		parentSymbol := b.root
		for parent >= 0 {
			if candidate, ok := b.nodeSyms[parent]; ok {
				parentSymbol = candidate
				break
			}
			parent = b.parsed.Nodes[parent].Parent
		}
		addRelation(&b.result, model.Relation{FromHandle: parentSymbol.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: 1, Source: "template-structural"})
	}
	for _, tag := range b.parsed.Tags {
		if tag.Closing {
			continue
		}
		switch tag.Name {
		case "extends", "include":
			target := tagTargetFor(b.file.Path, tag)
			if target == "" {
				continue
			}
			addRelation(&b.result, model.Relation{FromHandle: b.root.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .8, Source: tag.Name})
		case "call":
			target := tagValue(tag, "name")
			if target == "" {
				continue
			}
			from := b.ownerForOffset(tag.Region.StartByte).Handle
			matches := make([]model.Symbol, 0, 1)
			for _, symbol := range b.nodeSyms {
				if symbol.Kind == "template-function" && symbol.Name == target {
					matches = append(matches, symbol)
				}
			}
			relation := model.Relation{FromHandle: from, Kind: "calls", Confidence: .7, Source: "template-call"}
			if len(matches) == 1 {
				relation.ToHandle = matches[0].Handle
			} else {
				relation.UnresolvedTo = shortLexeme(target)
			}
			addRelation(&b.result, relation)
		}
	}
	for _, region := range b.regions {
		if region.Kind != KindScriptOpen {
			continue
		}
		attrs := htmlAttributes(region.Content)
		src := attrs["src"]
		if src == "" {
			continue
		}
		addRelation(&b.result, model.Relation{FromHandle: b.root.Handle, UnresolvedTo: normalizeExternalTarget(b.file.Path, src), Kind: "imports", Confidence: .75, Source: "script-src"})
	}
}

func (b *templateBuilder) ownerForOffset(offset int) model.Symbol {
	best := b.root
	bestSpan := len(b.file.Content) + 1
	for index, symbol := range b.nodeSyms {
		node := b.parsed.Nodes[index]
		if node.Tag.Region.StartByte <= offset && offset < node.EndByte {
			span := node.EndByte - node.Tag.Region.StartByte
			if span < bestSpan {
				best, bestSpan = symbol, span
			}
		}
	}
	return best
}

func (b *templateBuilder) newChunk(kind, symbolHandle, symbolName string, start, end int, content string) model.Chunk {
	startLine, endLine := lineRange(b.file.Content, start, end)
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", b.file.Path, kind, fmt.Sprintf("%d", start), content), FilePath: b.file.Path, Language: b.file.Language, Kind: kind, SymbolHandle: symbolHandle, SymbolName: symbolName, StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func (b *templateBuilder) sortResult() {
	sort.SliceStable(b.result.Symbols, func(i, j int) bool {
		if b.result.Symbols[i].StartByte != b.result.Symbols[j].StartByte {
			return b.result.Symbols[i].StartByte < b.result.Symbols[j].StartByte
		}
		if b.result.Symbols[i].Kind != b.result.Symbols[j].Kind {
			return b.result.Symbols[i].Kind < b.result.Symbols[j].Kind
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
		a, c := b.result.Relations[i], b.result.Relations[j]
		if a.FromHandle != c.FromHandle {
			return a.FromHandle < c.FromHandle
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		if a.ToHandle != c.ToHandle {
			return a.ToHandle < c.ToHandle
		}
		return a.UnresolvedTo < c.UnresolvedTo
	})
}

func addRelation(result *model.Extraction, relation model.Relation) {
	if relation.FromHandle == "" || relation.ToHandle == relation.FromHandle {
		return
	}
	for _, old := range result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	result.Relations = append(result.Relations, relation)
}

func nodeName(tag smartyTag) string {
	if value := tagValue(tag, "name"); value != "" && isStaticName(value) {
		return value
	}
	if len(tag.Positionals) > 0 && isStaticName(tag.Positionals[0]) {
		return tag.Positionals[0]
	}
	return ""
}

func tagValue(tag smartyTag, key string) string {
	if value := tag.Attributes[key]; value != "" {
		return value
	}
	if len(tag.Positionals) > 0 {
		return tag.Positionals[0]
	}
	return ""
}

func tagTarget(tag smartyTag) string {
	value := tagValue(tag, "file")
	if value == "" {
		return ""
	}
	return normalizeTemplateTarget(value)
}

func tagTargetFor(filePath string, tag smartyTag) string {
	value := tagValue(tag, "file")
	if value == "" {
		return ""
	}
	value = normalizeTemplateTarget(value)
	if value == "" || strings.HasPrefix(value, "$") || strings.ContainsAny(value, "${}|") || strings.HasPrefix(value, "string:") || strings.HasPrefix(value, "eval:") || strings.HasPrefix(value, "db:") || strings.HasPrefix(value, "resource:") {
		return value
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return value
	}
	if !strings.HasPrefix(value, "./") && !strings.HasPrefix(value, "../") {
		return path.Clean(value)
	}
	joined := path.Clean(path.Join(path.Dir(strings.ReplaceAll(filePath, "\\", "/")), value))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return shortLexeme(value)
	}
	return joined
}

func normalizeTemplateTarget(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "${}") || strings.Contains(value, "|") || strings.HasPrefix(value, "eval:") || strings.HasPrefix(value, "db:") || strings.HasPrefix(value, "resource:") || strings.HasPrefix(value, "string:") {
		return shortLexeme(value)
	}
	if strings.HasPrefix(value, "file:") {
		value = strings.TrimPrefix(value, "file:")
	}
	return strings.ReplaceAll(value, "\\", "/")
}

func normalizeExternalTarget(filePath, value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "${}") {
		return shortLexeme(value)
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "data:") {
		return shortLexeme(value)
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	if !strings.HasPrefix(value, "./") && !strings.HasPrefix(value, "../") {
		return path.Clean(value)
	}
	joined := path.Clean(path.Join(path.Dir(strings.ReplaceAll(filePath, "\\", "/")), value))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return shortLexeme(value)
	}
	return joined
}

func isStaticName(value string) bool { return value != "" && !strings.ContainsAny(value, "$|{}") }

func shortLexeme(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 120 {
		return value[:120]
	}
	return value
}

func mergeIntervals(intervals [][2]int, limit int) [][2]int {
	filtered := intervals[:0]
	for _, interval := range intervals {
		if interval[0] < 0 {
			interval[0] = 0
		}
		if interval[1] > limit {
			interval[1] = limit
		}
		if interval[1] > interval[0] {
			filtered = append(filtered, interval)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i][0] < filtered[j][0] })
	merged := make([][2]int, 0, len(filtered))
	for _, interval := range filtered {
		if len(merged) == 0 || interval[0] > merged[len(merged)-1][1] {
			merged = append(merged, interval)
			continue
		}
		if interval[1] > merged[len(merged)-1][1] {
			merged[len(merged)-1][1] = interval[1]
		}
	}
	return merged
}
