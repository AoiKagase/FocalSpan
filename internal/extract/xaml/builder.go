package xaml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
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
	if len(parsed.Diagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: file.Path, Level: "warning", Code: "xaml_partial_extraction", Message: "XAML extraction is syntax-approximate and recovered from malformed tags"})
	}
	allocator := model.NewHandleAllocator()
	mapa := sourceutil.NewSourceMap(file.Content)
	ownerHandle := allocator.Allocate("sym", file.Path, "xaml", "xaml_document", file.Path)
	owner := model.Symbol{Handle: ownerHandle, FilePath: file.Path, Language: "xaml", Kind: "xaml_document", Name: file.Path, QualifiedName: file.Path, Signature: "xaml document " + file.Path, StartLine: 1, EndLine: mapa.LineCount(), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	outline := "xaml document " + file.Path
	if className := attributeValue(parsed.Elements, 0, "x:Class"); className != "" {
		outline += "\nclass: " + className
	}
	result.Chunks = append(result.Chunks, syntheticChunk(file, owner, "xaml-document-outline", outline))

	elementHandles := make(map[int]string)
	resources := make(map[string]model.Symbol)
	for index, current := range parsed.Elements {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		name := attributeValueForElement(current, "x:Name")
		if name == "" {
			name = attributeValueForElement(current, "Name")
		}
		key := attributeValueForElement(current, "x:Key")
		if key == "" {
			key = attributeValueForElement(current, "Key")
		}
		event := firstEvent(current)
		kind := ""
		symbolName := current.Name
		switch {
		case index == firstElementIndex(parsed.Elements):
			kind = "xaml_element"
		case key != "":
			kind, symbolName = "xaml_resource", key
		case name != "":
			kind, symbolName = "xaml_named_element", name
		case event != "":
			kind = "xaml_element"
		case hasBindingOrResource(current):
			kind = "xaml_element"
		}
		if kind == "" {
			continue
		}
		span, ok := mapa.Span(current.StartByte, current.EndByte)
		if !ok || span.EndByte <= span.StartByte {
			continue
		}
		qualified := file.Path + "::" + symbolName
		handle := allocator.Allocate("sym", file.Path, "xaml", kind, qualified, string(file.Content[span.StartByte:span.EndByte]))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: "xaml", Kind: kind, Name: symbolName, QualifiedName: qualified, Signature: strings.TrimSpace(string(file.Content[span.StartByte:span.EndByte])), StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, ParentHandle: owner.Handle, Confidence: .85}
		result.Symbols = append(result.Symbols, symbol)
		result.Chunks = append(result.Chunks, sourceChunk(file, symbol, span))
		elementHandles[index] = handle
		if key != "" {
			resources[key] = symbol
		}
	}
	className := attributeValueForElement(firstElement(parsed.Elements), "x:Class")
	if className != "" {
		addRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: className, Kind: "references", Confidence: .9, Source: "xaml:x-class"})
	}
	for index, current := range parsed.Elements {
		from := owner.Handle
		if handle := elementHandles[index]; handle != "" {
			from = handle
		}
		for _, attr := range current.Attributes {
			name := strings.ToLower(attr.Name)
			if isEventAttribute(name) && attr.Value != "" && !strings.HasPrefix(attr.Value, "{") {
				addRelation(&result, model.Relation{FromHandle: from, UnresolvedTo: attr.Value, Kind: "references", Confidence: .8, Source: "xaml:event"})
			}
			if target := bindingTarget(attr.Value); target != "" {
				addRelation(&result, model.Relation{FromHandle: from, UnresolvedTo: target, Kind: "references", Confidence: .65, Source: "xaml:binding"})
			}
			if target := resourceTarget(attr.Value); target != "" {
				relation := model.Relation{FromHandle: from, UnresolvedTo: target, Kind: "references", Confidence: .7, Source: "xaml:resource"}
				if resource, exists := resources[target]; exists {
					relation.ToHandle = resource.Handle
					relation.UnresolvedTo = ""
				}
				addRelation(&result, relation)
			}
			if name == "source" && attr.Value != "" {
				addRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: attr.Value, Kind: "imports", Confidence: .8, Source: "xaml:dictionary"})
			}
		}
	}
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	return result
}

func firstElement(elements []element) element {
	if len(elements) == 0 {
		return element{}
	}
	return elements[0]
}

func firstElementIndex(elements []element) int {
	if len(elements) == 0 {
		return -1
	}
	return 0
}

func attributeValue(elements []element, index int, name string) string {
	if index < 0 || index >= len(elements) {
		return ""
	}
	return attributeValueForElement(elements[index], name)
}

func attributeValueForElement(current element, name string) string {
	for _, attr := range current.Attributes {
		if strings.EqualFold(attr.Name, name) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func firstEvent(current element) string {
	for _, attr := range current.Attributes {
		if isEventAttribute(strings.ToLower(attr.Name)) && attr.Value != "" {
			return attr.Value
		}
	}
	return ""
}

func hasBindingOrResource(current element) bool {
	for _, attr := range current.Attributes {
		if bindingTarget(attr.Value) != "" || resourceTarget(attr.Value) != "" {
			return true
		}
	}
	return false
}

func isEventAttribute(name string) bool {
	if strings.Contains(name, ":") || name == "name" || name == "x:name" || name == "x:key" || name == "key" || name == "source" || name == "datacontext" {
		return false
	}
	for _, suffix := range []string{"click", "loaded", "load", "closing", "closed", "opened", "selectionchanged", "textchanged", "changed", "command"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

var bindingPattern = regexp.MustCompile(`(?i)^\{\s*(?:binding|x:bind)\s+([^,}\s]+)`)
var resourcePattern = regexp.MustCompile(`(?i)^\{\s*(?:staticresource|dynamicresource)\s+([^,}\s]+)`)

func bindingTarget(value string) string {
	match := bindingPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func resourceTarget(value string) string {
	match := resourcePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func syntheticChunk(file model.SourceFile, owner model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", owner.Handle, kind, content), FilePath: file.Path, Language: "xaml", Kind: kind, SymbolHandle: owner.Handle, SymbolName: owner.Name, Signature: "synthetic outline (not a source slice)", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 0, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func sourceChunk(file model.SourceFile, symbol model.Symbol, span sourceutil.Span) model.Chunk {
	content := string(file.Content[span.StartByte:span.EndByte])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, symbol.Kind, fmt.Sprint(span.StartByte), fmt.Sprint(span.EndByte)), FilePath: file.Path, Language: "xaml", Kind: symbol.Kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: span.StartLine, EndLine: span.EndLine, StartByte: span.StartByte, EndByte: span.EndByte, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func addRelation(result *model.Extraction, relation model.Relation) {
	for _, old := range result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	result.Relations = append(result.Relations, relation)
}
