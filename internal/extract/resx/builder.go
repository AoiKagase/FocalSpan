package resx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

func build(ctx context.Context, file model.SourceFile, parsed scanResult) model.Extraction {
	result := model.Extraction{}
	for _, diagnostic := range parsed.Diagnostics {
		diagnostic.FilePath = file.Path
		result.Diagnostics = append(result.Diagnostics, diagnostic)
	}
	allocator := model.NewHandleAllocator()
	mapa := sourceutil.NewSourceMap(file.Content)
	ownerHandle := allocator.Allocate("sym", file.Path, "resx", "resx_document", file.Path)
	owner := model.Symbol{Handle: ownerHandle, FilePath: file.Path, Language: "dotnet-resource", Kind: "resx_document", Name: file.Path, QualifiedName: file.Path, Signature: "resource document " + file.Path, StartLine: 1, EndLine: mapa.LineCount(), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	outline := "resx document " + file.Path
	result.Chunks = append(result.Chunks, syntheticChunk(file, owner, outline))
	for _, current := range parsed.Items {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		nameAttr := findAttribute(current.Attributes, "name")
		if current.Name == "" || nameAttr.Value == "" {
			continue
		}
		span, ok := mapa.Span(nameAttr.ValueFrom, nameAttr.ValueTo)
		if !ok || span.EndByte <= span.StartByte {
			continue
		}
		qualified := file.Path + "::" + current.Name + "::" + current.Name
		handle := allocator.Allocate("sym", file.Path, "resx", current.Kind, qualified, nameAttr.Value)
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: "dotnet-resource", Kind: current.Kind, Name: nameAttr.Value, QualifiedName: qualified, Signature: nameAttr.Value, StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, ParentHandle: owner.Handle, Confidence: .85}
		result.Symbols = append(result.Symbols, symbol)
		result.Chunks = append(result.Chunks, sourceChunk(file, symbol, span))
		result.Relations = append(result.Relations, model.Relation{FromHandle: owner.Handle, ToHandle: symbol.Handle, Kind: "contains", Confidence: .9, Source: "resx-structural"})
		for _, attrName := range []string{"type", "mimetype", "scope"} {
			if attr := findAttribute(current.Attributes, attrName); attr.Value != "" {
				result.Relations = append(result.Relations, model.Relation{FromHandle: symbol.Handle, UnresolvedTo: attr.Value, Kind: "references", Confidence: .55, Source: "resx:" + attrName})
			}
		}
	}
	for _, target := range parsed.Imports {
		result.Relations = append(result.Relations, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .8, Source: "resx:ResXFileRef"})
	}
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	return result
}

func findAttribute(attrs []attribute, wanted string) attribute {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, wanted) {
			return attr
		}
	}
	return attribute{}
}

func syntheticChunk(file model.SourceFile, owner model.Symbol, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", owner.Handle, "resx-outline", content), FilePath: file.Path, Language: "dotnet-resource", Kind: "resx-outline", SymbolHandle: owner.Handle, SymbolName: owner.Name, Signature: "synthetic outline (not a source slice)", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 0, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func sourceChunk(file model.SourceFile, symbol model.Symbol, span sourceutil.Span) model.Chunk {
	content := string(file.Content[span.StartByte:span.EndByte])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, symbol.Kind, fmt.Sprint(span.StartByte), fmt.Sprint(span.EndByte)), FilePath: file.Path, Language: "dotnet-resource", Kind: symbol.Kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}
