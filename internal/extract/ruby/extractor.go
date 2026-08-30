package ruby

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "ruby-structural" }
func (Extractor) Supports(path, language string) bool {
	base := strings.ToLower(filepath.Base(path))
	return language == "ruby" || strings.EqualFold(filepath.Ext(path), ".rb") || strings.EqualFold(filepath.Ext(path), ".rake") || strings.HasSuffix(strings.ToLower(path), ".gemspec") || base == "rakefile" || base == "gemfile"
}

type rubyLine struct {
	Start, End int
	Indent     int
	Text       string
}
type rubyDeclaration struct {
	Kind, Name, Qualified, Header string
	StartLine, EndLine, Depth     int
	Parent                        *rubyDeclaration
}

var (
	rubyModule   = regexp.MustCompile(`^module\s+([A-Za-z_]\w*(?:::\w+)*)`)
	rubyClass    = regexp.MustCompile(`^class\s+(.+?)\s*(?:<\s*([A-Za-z_:][\w:]*)\s*)?$`)
	rubyDef      = regexp.MustCompile(`^def\s+(.+)$`)
	rubyAttr     = regexp.MustCompile(`^attr_(?:reader|writer|accessor)\s+(.+)$`)
	rubyConstant = regexp.MustCompile(`^([A-Z][A-Za-z0-9_]*)\s*=`)
	rubyAlias    = regexp.MustCompile(`^alias\s+([A-Za-z_]\w*)\s+([A-Za-z_]\w*)`)
	rubyDefine   = regexp.MustCompile(`^define_method\s*\(\s*:([A-Za-z_]\w*)`)
	rubyTest     = regexp.MustCompile(`^(?:it|specify|test)\s+(?:\(?\s*["']([^"']+)["']|["']([^"']+)["'])`)
	rubyCall     = regexp.MustCompile(`\b([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*(?:\(|$)`)
)

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	_, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	return buildRuby(ctx, file, diagnostics), nil
}

func buildRuby(ctx context.Context, file model.SourceFile, diagnostics []model.Diagnostic) model.Extraction {
	result := model.Extraction{Diagnostics: diagnostics}
	allocator := model.NewHandleAllocator()
	ownerName := filepath.Base(filepath.ToSlash(file.Path))
	owner := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "ruby", "module", ownerName), FilePath: file.Path, Language: file.Language, Kind: "module", Name: ownerName, QualifiedName: ownerName, Signature: "module " + ownerName, StartLine: 1, EndLine: rubyLineCount(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	result.Chunks = append(result.Chunks, rubySyntheticChunk(file, owner, "module-outline", "module "+ownerName))
	lines := rubyLines(file.Content)
	declarations := parseRuby(lines)
	byDecl := make(map[*rubyDeclaration]model.Symbol)
	for _, decl := range declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		start, end := lines[decl.StartLine-1].Start, lines[decl.EndLine-1].End
		handle := allocator.Allocate("sym", file.Path, "ruby", decl.Kind, decl.Qualified, model.NormalizeSignature(decl.Header))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: decl.Qualified, Signature: decl.Header, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .95}
		if decl.Parent != nil {
			symbol.ParentHandle = byDecl[decl.Parent].Handle
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		chunkEnd := end
		chunkKind := decl.Kind
		if decl.Kind == "module" || decl.Kind == "class" || decl.Kind == "singleton_class" {
			chunkEnd = lines[decl.StartLine-1].End
			chunkKind += "-outline"
		}
		result.Chunks = append(result.Chunks, rubyChunk(file, symbol, chunkKind, start, chunkEnd))
	}
	index := rubyIndex(result.Symbols)
	for lineIndex, line := range lines {
		text := strings.TrimSpace(line.Text)
		if strings.HasPrefix(text, "require") || strings.HasPrefix(text, "require_relative") {
			result.Chunks = append(result.Chunks, rubySourceChunk(file, owner, "import", line.Start, line.End, lineIndex+1, lineIndex+1))
			if target := rubyRequireTarget(text); target != "" {
				addRubyRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "ruby-require"})
			}
		}
	}
	for _, decl := range declarations {
		from := byDecl[decl]
		parent := owner
		if decl.Parent != nil {
			parent = byDecl[decl.Parent]
		}
		addRubyRelation(&result, model.Relation{FromHandle: parent.Handle, ToHandle: from.Handle, Kind: "contains", Confidence: from.Confidence, Source: "ruby-structural"})
		if strings.HasPrefix(decl.Header, "class ") {
			if match := rubyClass.FindStringSubmatch(decl.Header); len(match) == 3 && match[2] != "" {
				addRubyRelation(&result, model.Relation{FromHandle: from.Handle, UnresolvedTo: match[2], Kind: "references", Confidence: .8, Source: "ruby-inheritance"})
			}
		}
		for line := decl.StartLine; line <= decl.EndLine && line <= len(lines); line++ {
			text := strings.TrimSpace(lines[line-1].Text)
			for _, keyword := range []string{"include ", "extend ", "prepend "} {
				if strings.HasPrefix(text, keyword) {
					addRubyRelation(&result, model.Relation{FromHandle: from.Handle, UnresolvedTo: strings.TrimSpace(strings.TrimPrefix(text, keyword)), Kind: "references", Confidence: .8, Source: "ruby-mixin"})
				}
			}
			for _, match := range rubyCall.FindAllStringSubmatch(text, -1) {
				name := match[1]
				if name == "def" || name == "class" || name == "module" || name == "if" || name == "unless" || name == "while" || name == "require" || name == "puts" || name == "describe" || name == "it" || name == "test" {
					continue
				}
				kind := "calls"
				if decl.Kind == "test" {
					kind = "tests"
				}
				if target, ok := uniqueRuby(index, name); ok {
					addRubyRelation(&result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .85, Source: "ruby-call"})
				} else {
					addRubyRelation(&result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: "ruby-call"})
				}
			}
		}
	}
	sortRuby(&result)
	return result
}

func parseRuby(lines []rubyLine) []*rubyDeclaration {
	declarations := make([]*rubyDeclaration, 0)
	stack := make([]struct {
		decl  *rubyDeclaration
		depth int
	}, 0)
	depth := 0
	decorators := make([]string, 0)
	for index, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "=") && strings.HasPrefix(text, "=begin") {
			continue
		}
		if strings.HasPrefix(text, "@") {
			decorators = append(decorators, text)
			continue
		}
		var decl *rubyDeclaration
		if match := rubyModule.FindStringSubmatch(text); len(match) == 2 {
			decl = &rubyDeclaration{Kind: "module", Name: match[1], Header: text, StartLine: index + 1, Depth: depth}
		} else if strings.HasPrefix(text, "class <<") {
			decl = &rubyDeclaration{Kind: "singleton_class", Name: "<< self", Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyClass.FindStringSubmatch(text); len(match) >= 2 {
			decl = &rubyDeclaration{Kind: "class", Name: strings.TrimSpace(match[1]), Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyDef.FindStringSubmatch(text); len(match) == 2 {
			name := rubyMethodName(match[1])
			kind := "function"
			if strings.HasPrefix(name, "self.") {
				name = strings.TrimPrefix(name, "self.")
				kind = "singleton_method"
			}
			if len(stack) > 0 && stack[len(stack)-1].decl != nil {
				p := stack[len(stack)-1].decl
				if p.Kind == "class" || p.Kind == "singleton_class" {
					if kind == "function" {
						kind = "method"
					}
				}
			}
			if kind == "method" && strings.HasPrefix(name, "test_") {
				kind = "test"
			}
			decl = &rubyDeclaration{Kind: kind, Name: name, Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyTest.FindStringSubmatch(text); len(match) >= 3 {
			name := match[1]
			if name == "" {
				name = match[2]
			}
			decl = &rubyDeclaration{Kind: "test", Name: name, Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyAttr.FindStringSubmatch(text); len(match) == 2 {
			name := strings.TrimSpace(strings.TrimPrefix(match[1], ":"))
			name = strings.Trim(name, "\"'")
			decl = &rubyDeclaration{Kind: "accessor", Name: name, Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyConstant.FindStringSubmatch(text); len(match) == 2 {
			decl = &rubyDeclaration{Kind: "constant", Name: match[1], Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyAlias.FindStringSubmatch(text); len(match) == 3 {
			decl = &rubyDeclaration{Kind: "alias", Name: match[1], Header: text, StartLine: index + 1, Depth: depth}
		} else if match := rubyDefine.FindStringSubmatch(text); len(match) == 2 {
			decl = &rubyDeclaration{Kind: "define_method", Name: match[1], Header: text, StartLine: index + 1, Depth: depth}
		}
		if decl != nil {
			if len(stack) > 0 {
				parent := stack[len(stack)-1].decl
				if parent != nil && parent.StartLine < decl.StartLine {
					decl.Parent = parent
				}
			}
			if decl.Parent != nil {
				decl.Qualified = decl.Parent.Qualified + "::" + decl.Name
			} else {
				decl.Qualified = decl.Name
			}
			if len(decorators) > 0 && containsRubyDecorator(decorators, "spec") && decl.Kind == "function" {
				decl.Kind = "test"
			}
			declarations = append(declarations, decl)
			decorators = nil
		}
		delta := rubyDepthDelta(text)
		depth += delta
		if decl != nil && delta > 0 {
			stack = append(stack, struct {
				decl  *rubyDeclaration
				depth int
			}{decl, depth})
		}
		if strings.HasPrefix(text, "end") || strings.HasSuffix(text, " end") {
			for len(stack) > 0 && depth < stack[len(stack)-1].depth {
				stack[len(stack)-1].decl.EndLine = index + 1
				stack = stack[:len(stack)-1]
			}
			if depth < 0 {
				depth = 0
			}
		}
	}
	for _, item := range stack {
		item.decl.EndLine = len(lines)
	}
	for _, decl := range declarations {
		if decl.EndLine == 0 {
			decl.EndLine = len(lines)
			for line := decl.StartLine; line < len(lines); line++ {
				if strings.TrimSpace(lines[line].Text) == "end" {
					decl.EndLine = line + 1
					break
				}
			}
		}
	}
	return declarations
}

func rubyMethodName(header string) string {
	name := strings.TrimSpace(header)
	if end := strings.IndexAny(name, "( \t"); end >= 0 {
		name = name[:end]
	}
	return name
}

func rubyDepthDelta(text string) int {
	delta := 0
	for _, word := range []string{"module", "class", "def", "do", "begin", "case", "if", "unless"} {
		if regexp.MustCompile(`(^|\s)` + word + `(\s|$)`).MatchString(text) {
			delta++
		}
	}
	if strings.HasPrefix(text, "end") {
		delta--
	}
	return delta
}
func containsRubyDecorator(values []string, value string) bool {
	for _, item := range values {
		if strings.Contains(item, value) {
			return true
		}
	}
	return false
}
func rubyRequireTarget(text string) string {
	if index := strings.IndexAny(text, "\"'"); index >= 0 {
		rest := text[index+1:]
		if end := strings.IndexAny(rest, "\"'"); end >= 0 {
			return rest[:end]
		}
	}
	return ""
}
func rubyLines(source []byte) []rubyLine {
	starts := []int{0}
	for index, value := range source {
		if value == '\n' && index+1 < len(source) {
			starts = append(starts, index+1)
		}
	}
	lines := make([]rubyLine, 0, len(starts))
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for end > start && (source[end-1] == '\n' || source[end-1] == '\r') {
			end--
		}
		indent := 0
		for at := start; at < end && (source[at] == ' ' || source[at] == '\t'); at++ {
			indent++
		}
		lines = append(lines, rubyLine{Start: start, End: end, Indent: indent, Text: string(source[start:end])})
	}
	return lines
}
func rubyLineCount(source []byte) int { return 1 + strings.Count(string(source), "\n") }
func rubyIndex(symbols []model.Symbol) map[string][]model.Symbol {
	index := make(map[string][]model.Symbol)
	for _, symbol := range symbols {
		if symbol.Kind != "module" || strings.Contains(symbol.Name, ".rb") {
			index[strings.ToLower(symbol.Name)] = append(index[strings.ToLower(symbol.Name)], symbol)
		}
	}
	return index
}
func uniqueRuby(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := index[strings.ToLower(strings.TrimPrefix(name, "self."))]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}
func rubySyntheticChunk(file model.SourceFile, symbol model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}
func rubyChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end int) model.Chunk {
	return rubySourceChunk(file, symbol, kind, start, end, symbol.StartLine, symbol.EndLine)
}

func rubySourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end, startLine, endLine int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}
func addRubyRelation(result *model.Extraction, relation model.Relation) {
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
func sortRuby(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		return result.Relations[i].Kind < result.Relations[j].Kind
	})
}
