package template

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/model"
)

func appendEmbeddedScript(ctx context.Context, file model.SourceFile, region Region, language, contextName string, parent model.Symbol, allocator *model.HandleAllocator, result *model.Extraction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if region.EndByte <= region.StartByte {
		return nil
	}
	region = trimLeadingBlankLines(region)
	if region.EndByte <= region.StartByte {
		return nil
	}
	local := model.SourceFile{Path: file.Path, Language: language, Content: region.Content}
	delegated, err := generic.NewExtractor().Extract(ctx, local)
	if err != nil {
		return err
	}
	remapped := make(map[string]string, len(delegated.Symbols))
	for _, old := range delegated.Symbols {
		if err := ctx.Err(); err != nil {
			return err
		}
		contextPrefix := file.Path + "::script::" + contextName + "::js::"
		if parent.Handle != "" && parent.Kind != "template" {
			contextPrefix = parent.QualifiedName + "::js::"
		}
		qualified := contextPrefix + old.Name
		newHandle := allocator.Allocate("sym", file.Path, language, old.Kind, qualified, model.NormalizeSignature(old.Signature))
		remapped[old.Handle] = newHandle
		start, end := embeddedSpan(file.Content, region, old.StartLine, old.EndLine)
		copySymbol := old
		copySymbol.Handle = newHandle
		copySymbol.FilePath = file.Path
		copySymbol.Language = language
		copySymbol.QualifiedName = qualified
		copySymbol.ParentHandle = parent.Handle
		copySymbol.StartLine, copySymbol.EndLine = lineRange(file.Content, start, end)
		copySymbol.StartByte, copySymbol.EndByte = start, end
		copySymbol.Confidence = .8
		result.Symbols = append(result.Symbols, copySymbol)
		if parent.Handle != "" {
			addRelation(result, model.Relation{FromHandle: parent.Handle, ToHandle: newHandle, Kind: "contains", Confidence: .8, Source: "template-embedded"})
		}
	}
	for _, old := range delegated.Relations {
		relation := old
		if mapped, ok := remapped[relation.FromHandle]; ok {
			relation.FromHandle = mapped
		}
		if mapped, ok := remapped[relation.ToHandle]; ok {
			relation.ToHandle = mapped
		}
		if relation.FromHandle != "" {
			addRelation(result, relation)
		}
	}
	for _, old := range delegated.Chunks {
		start, end := embeddedSpan(file.Content, region, old.StartLine, old.EndLine)
		content := string(file.Content[start:end])
		if strings.TrimSpace(content) == "" {
			continue
		}
		symbolHandle := remapped[old.SymbolHandle]
		chunk := old
		chunk.Handle = allocator.Allocate("chunk", file.Path, language, contextName, old.Kind, content)
		chunk.FilePath = file.Path
		chunk.Language = language
		chunk.SymbolHandle = symbolHandle
		chunk.StartByte, chunk.EndByte = start, end
		chunk.StartLine, chunk.EndLine = lineRange(file.Content, start, end)
		chunk.Content = content
		digest := sha256.Sum256([]byte(content))
		chunk.ContentHash = hex.EncodeToString(digest[:])
		result.Chunks = append(result.Chunks, chunk)
	}
	return nil
}

func trimLeadingBlankLines(region Region) Region {
	offset := 0
	for offset < len(region.Content) {
		lineEnd := offset
		for lineEnd < len(region.Content) && region.Content[lineEnd] != '\n' {
			lineEnd++
		}
		line := strings.TrimSuffix(string(region.Content[offset:lineEnd]), "\r")
		if strings.TrimSpace(line) != "" {
			break
		}
		if lineEnd < len(region.Content) {
			lineEnd++
		}
		offset = lineEnd
	}
	if offset == 0 {
		return region
	}
	region.StartByte += offset
	region.Content = region.Content[offset:]
	return region
}

func embeddedSpan(source []byte, region Region, startLine, endLine int) (int, int) {
	if startLine < 1 {
		startLine = 1
	}
	if endLine < startLine {
		endLine = startLine
	}
	localStart := localLineStart(region.Content, startLine)
	localEnd := localLineEnd(region.Content, endLine)
	return region.StartByte + localStart, region.StartByte + localEnd
}

func localLineStart(source []byte, line int) int {
	if line <= 1 {
		return 0
	}
	current := 1
	for index, value := range source {
		if value == '\n' {
			current++
			if current == line {
				return index + 1
			}
		}
	}
	return len(source)
}

func localLineEnd(source []byte, line int) int {
	start := localLineStart(source, line)
	for index := start; index < len(source); index++ {
		if source[index] == '\n' {
			return index + 1
		}
	}
	return len(source)
}
