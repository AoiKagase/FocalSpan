package python

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
func (Extractor) Name() string { return "python-structural" }
func (Extractor) Supports(path, language string) bool {
	return language == "python" || strings.EqualFold(filepath.Ext(path), ".py") || strings.EqualFold(filepath.Ext(path), ".pyw") || strings.EqualFold(filepath.Ext(path), ".pyi")
}

type pyDeclaration struct {
	Kind       string
	Name       string
	Qualified  string
	StartLine  int
	EndLine    int
	Indent     int
	Parent     *pyDeclaration
	Header     string
	Decorators []string
}

var (
	pyClassPattern    = regexp.MustCompile(`^class\s+([A-Za-z_]\w*)`)
	pyFunctionPattern = regexp.MustCompile(`^(?:(async)\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyAliasPattern    = regexp.MustCompile(`^([A-Za-z_]\w*)\s*=\s*([A-Za-z_]\w*)\b`)
	pyClassVarPattern = regexp.MustCompile(`^([A-Za-z_]\w*)\s*=`)
	pyCallPattern     = regexp.MustCompile(`\b([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\(`)
)

type pyLine struct {
	Start, End, ContentStart int
	Indent                   int
	Text                     string
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	_, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	return buildPython(ctx, file, diagnostics), nil
}

func buildPython(ctx context.Context, file model.SourceFile, diagnostics []model.Diagnostic) model.Extraction {
	lines := pythonLines(file.Content)
	result := model.Extraction{}
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	allocator := model.NewHandleAllocator()
	ownerName := filepath.Base(filepath.ToSlash(file.Path))
	owner := model.Symbol{Handle: allocator.Allocate("sym", file.Path, "python", "module", ownerName), FilePath: file.Path, Language: file.Language, Kind: "module", Name: ownerName, QualifiedName: ownerName, Signature: "module " + ownerName, StartLine: 1, EndLine: lineCountPython(file.Content), StartByte: 0, EndByte: len(file.Content), Confidence: .9}
	result.Symbols = append(result.Symbols, owner)
	result.Chunks = append(result.Chunks, pythonSyntheticChunk(file, owner, "module-outline", "module "+ownerName))
	declarations := parsePythonDeclarations(lines)
	byDecl := make(map[*pyDeclaration]model.Symbol)
	for _, decl := range declarations {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}
		}
		start := lines[decl.StartLine-1].Start
		end := lines[decl.EndLine-1].End
		qualified := decl.Qualified
		signature := strings.TrimSpace(string(file.Content[start:lines[decl.StartLine-1].End]))
		if len(decl.Decorators) > 0 {
			signature = strings.Join(decl.Decorators, " ") + " " + signature
		}
		handle := allocator.Allocate("sym", file.Path, "python", decl.Kind, qualified, model.NormalizeSignature(signature))
		symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: decl.Kind, Name: decl.Name, QualifiedName: qualified, Signature: signature, StartLine: decl.StartLine, EndLine: decl.EndLine, StartByte: start, EndByte: end, Confidence: .95}
		if decl.Parent != nil {
			symbol.ParentHandle = byDecl[decl.Parent].Handle
		}
		byDecl[decl] = symbol
		result.Symbols = append(result.Symbols, symbol)
		chunkEnd := end
		chunkKind := decl.Kind
		if decl.Kind == "class" || decl.Kind == "protocol" {
			chunkEnd = lines[decl.StartLine-1].End
			chunkKind += "-outline"
		}
		result.Chunks = append(result.Chunks, pythonChunk(file, symbol, chunkKind, start, chunkEnd))
	}
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if strings.HasPrefix(text, "import ") || strings.HasPrefix(text, "from ") {
			if target := pythonImportTarget(text); target != "" {
				addPythonRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "python-import"})
			}
		}
	}
	index := pythonIndex(result.Symbols)
	for _, decl := range declarations {
		from := byDecl[decl]
		parent := owner
		if decl.Parent != nil {
			parent = byDecl[decl.Parent]
		}
		addPythonRelation(&result, model.Relation{FromHandle: parent.Handle, ToHandle: from.Handle, Kind: "contains", Confidence: from.Confidence, Source: "python-structural"})
		if decl.Kind == "class" || decl.Kind == "protocol" {
			for _, base := range pythonBases(decl.Header) {
				addPythonResolvedOrUnresolved(&result, index, from, base, "references", "python-base")
			}
		}
		for line := decl.StartLine; line <= decl.EndLine && line <= len(lines); line++ {
			text := strings.TrimSpace(lines[line-1].Text)
			if decl.Parent == nil && (strings.HasPrefix(text, "import ") || strings.HasPrefix(text, "from ")) {
				if target := pythonImportTarget(text); target != "" {
					addPythonRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "python-import"})
				}
			}
			for _, match := range pyCallPattern.FindAllStringSubmatch(text, -1) {
				name := match[1]
				if strings.HasPrefix(name, "def") || name == "if" || name == "for" || name == "while" || name == "class" || name == "print" {
					continue
				}
				short := name
				if dot := strings.LastIndexByte(short, '.'); dot >= 0 {
					short = short[dot+1:]
				}
				kind := "calls"
				if decl.Kind == "test" {
					kind = "tests"
				}
				if target, ok := uniquePython(index.byName, short); ok {
					addPythonRelation(&result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .9, Source: "python-call"})
				} else {
					addPythonRelation(&result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .25, Source: "python-call"})
				}
			}
		}
	}
	sortPython(&result)
	return result
}

type pythonIndexData struct{ byName map[string][]model.Symbol }

func pythonIndex(symbols []model.Symbol) pythonIndexData {
	index := pythonIndexData{byName: make(map[string][]model.Symbol)}
	for _, symbol := range symbols {
		if symbol.Kind != "module" {
			index.byName[strings.ToLower(symbol.Name)] = append(index.byName[strings.ToLower(symbol.Name)], symbol)
		}
	}
	return index
}
func uniquePython(values map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := values[strings.ToLower(name)]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}
func addPythonResolvedOrUnresolved(result *model.Extraction, index pythonIndexData, from model.Symbol, name, kind, source string) {
	if target, ok := uniquePython(index.byName, name); ok {
		addPythonRelation(result, model.Relation{FromHandle: from.Handle, ToHandle: target.Handle, Kind: kind, Confidence: .8, Source: source})
		return
	}
	addPythonRelation(result, model.Relation{FromHandle: from.Handle, UnresolvedTo: name, Kind: kind, Confidence: .3, Source: source})
}
func addPythonRelation(result *model.Extraction, relation model.Relation) {
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

func parsePythonDeclarations(lines []pyLine) []*pyDeclaration {
	declarations := make([]*pyDeclaration, 0)
	decorators := make([]string, 0)
	for index, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "@") {
			decorators = append(decorators, text)
			continue
		}
		var decl *pyDeclaration
		if match := pyClassPattern.FindStringSubmatch(text); len(match) == 2 {
			kind := "class"
			if strings.Contains(text, "(Protocol") {
				kind = "protocol"
			}
			decl = &pyDeclaration{Kind: kind, Name: match[1], StartLine: index + 1, Indent: line.Indent, Header: text, Decorators: decorators}
		} else if match := pyFunctionPattern.FindStringSubmatch(text); len(match) == 3 {
			kind := "function"
			if match[1] == "async" {
				kind = "async_function"
			}
			if strings.HasPrefix(match[2], "test_") || strings.Contains(strings.Join(decorators, " "), "pytest.mark") {
				kind = "test"
			}
			if containsDecorator(decorators, "pytest.fixture") {
				kind = "fixture"
			}
			parent := nearestPythonParent(declarations, line.Indent, index+1)
			if parent != nil && (parent.Kind == "class" || parent.Kind == "protocol") {
				kind = "method"
				if match[1] == "async" {
					kind = "async_method"
				}
				if containsDecorator(decorators, "@property") {
					kind = "property"
				}
				if containsDecorator(decorators, "pytest.fixture") {
					kind = "fixture"
				}
			} else if parent != nil && (parent.Kind == "function" || parent.Kind == "async_function" || parent.Kind == "method" || parent.Kind == "async_method") {
				kind = "nested_function"
			}
			decl = &pyDeclaration{Kind: kind, Name: match[2], StartLine: index + 1, Indent: line.Indent, Header: text, Parent: parent, Decorators: decorators}
		} else if line.Indent == 0 && pyAliasPattern.MatchString(text) && !strings.Contains(text, "(") {
			match := pyAliasPattern.FindStringSubmatch(text)
			decl = &pyDeclaration{Kind: "type_alias", Name: match[1], StartLine: index + 1, Indent: line.Indent, Header: text, Decorators: decorators}
		} else if parent := nearestPythonParent(declarations, line.Indent, index+1); parent != nil && (parent.Kind == "class" || parent.Kind == "protocol") && pyClassVarPattern.MatchString(text) {
			match := pyClassVarPattern.FindStringSubmatch(text)
			decl = &pyDeclaration{Kind: "class_variable", Name: match[1], StartLine: index + 1, Indent: line.Indent, Header: text, Parent: parent, Decorators: decorators}
		}
		if decl != nil {
			if decl.Parent != nil {
				decl.Qualified = decl.Parent.Qualified + "." + decl.Name
			} else {
				decl.Qualified = decl.Name
			}
			declarations = append(declarations, decl)
			decorators = nil
		}
	}
	for index, decl := range declarations {
		end := len(lines)
		for line := decl.StartLine; line < len(lines); line++ {
			if strings.TrimSpace(lines[line].Text) == "" || strings.HasPrefix(strings.TrimSpace(lines[line].Text), "#") {
				continue
			}
			if lines[line].Indent <= decl.Indent {
				end = line
				break
			}
		}
		decl.EndLine = end
		_ = index
	}
	return declarations
}

func nearestPythonParent(declarations []*pyDeclaration, indent, beforeLine int) *pyDeclaration {
	var parent *pyDeclaration
	for _, decl := range declarations {
		if decl.StartLine >= beforeLine || decl.Indent >= indent {
			continue
		}
		if parent == nil || decl.StartLine > parent.StartLine {
			parent = decl
		}
	}
	return parent
}
func containsDecorator(decorators []string, value string) bool {
	for _, decorator := range decorators {
		if strings.Contains(decorator, value) {
			return true
		}
	}
	return false
}
func pythonBases(header string) []string {
	open := strings.IndexByte(header, '(')
	close := strings.LastIndexByte(header, ')')
	if open < 0 || close <= open {
		return nil
	}
	result := make([]string, 0)
	for _, value := range strings.Split(header[open+1:close], ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
func pythonImportTarget(text string) string {
	if strings.HasPrefix(text, "from ") {
		value := strings.TrimPrefix(text, "from ")
		fields := strings.Fields(value)
		if len(fields) >= 3 && fields[1] == "import" {
			imported := strings.TrimSpace(strings.Split(fields[2], ",")[0])
			if imported != "" && imported != "*" {
				return fields[0] + "." + imported
			}
		}
		if len(fields) > 0 {
			return fields[0]
		}
		return value
	}
	value := strings.TrimPrefix(text, "import ")
	if space := strings.IndexAny(value, " \t"); space >= 0 {
		return value[:space]
	}
	return value
}

func pythonLines(source []byte) []pyLine {
	lines := make([]pyLine, 0)
	starts := lineStarts(source)
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		contentEnd := end
		for contentEnd > start && (source[contentEnd-1] == '\n' || source[contentEnd-1] == '\r') {
			contentEnd--
		}
		contentStart := start
		indent := 0
		for contentStart < contentEnd && (source[contentStart] == ' ' || source[contentStart] == '\t') {
			if source[contentStart] == '\t' {
				indent = (indent/4 + 1) * 4
			} else {
				indent++
			}
			contentStart++
		}
		lines = append(lines, pyLine{Start: start, End: contentEnd, ContentStart: contentStart, Indent: indent, Text: string(source[contentStart:contentEnd])})
	}
	return lines
}
func lineCountPython(source []byte) int { return 1 + strings.Count(string(source), "\n") }
func pythonSyntheticChunk(file model.SourceFile, symbol model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}
func pythonChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end int) model.Chunk {
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: symbol.StartLine, EndLine: symbol.EndLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}
func sortPython(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].StartByte < result.Symbols[j].StartByte })
	sort.SliceStable(result.Chunks, func(i, j int) bool { return result.Chunks[i].StartByte < result.Chunks[j].StartByte })
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		return result.Relations[i].Kind < result.Relations[j].Kind
	})
}
