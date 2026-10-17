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
	if language != "" {
		return language == "python"
	}
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

const pyIdentifier = `[\p{L}\p{Nl}_\x{1885}\x{1886}\x{2118}\x{212e}\x{309b}\x{309c}][\p{L}\p{N}\p{M}\p{Pc}_\x{00b7}\x{0387}\x{1369}-\x{1371}\x{19da}\x{2118}\x{212e}]*`

var (
	pyParameterPattern  = regexp.MustCompile(`^` + pyIdentifier)
	pyBindingIdentifier = regexp.MustCompile(`^` + pyIdentifier + `$`)
	pyClassPattern      = regexp.MustCompile(`^class\s+(` + pyIdentifier + `)`)
	pyFunctionPattern   = regexp.MustCompile(`^(?:(async)\s+)?def\s+(` + pyIdentifier + `)\s*\(`)
	pyAliasPattern      = regexp.MustCompile(`^(` + pyIdentifier + `)\s*=\s*(` + pyIdentifier + `)`)
	pyClassVarPattern   = regexp.MustCompile(`^(` + pyIdentifier + `)\s*=`)
	pyCallPattern       = regexp.MustCompile(`(` + pyIdentifier + `(?:\.` + pyIdentifier + `)*)\s*\(`)
)

type pyLine struct {
	Start, End, ContentStart int
	Indent                   int
	Text                     string
	Original                 string
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	tokens, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	result := buildPython(ctx, file, diagnostics, tokens)
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	return result, nil
}

func buildPython(ctx context.Context, file model.SourceFile, diagnostics []model.Diagnostic, tokens []Token) model.Extraction {
	lines := pythonLines(file.Content)
	code := pythonCodeView(file.Content, tokens)
	for i := range lines {
		lines[i].Original = lines[i].Text
		lines[i].Text = strings.TrimRight(string(code[lines[i].Start:lines[i].End]), "\r\n")
	}
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
	imports := pythonImportRecords(lines)
	for index := range lines {
		for _, target := range imports[index] {
			addPythonRelation(&result, model.Relation{FromHandle: owner.Handle, UnresolvedTo: target, Kind: "imports", Confidence: .9, Source: "python-import"})
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
			if end := pythonNestedScopeEnd(decl, line, byDecl); end > 0 {
				line = end
				continue
			}
			text := strings.TrimSpace(lines[line-1].Text)
			for _, match := range pyCallPattern.FindAllStringSubmatch(text, -1) {
				name := match[1]
				if strings.HasPrefix(name, "def") || name == "if" || name == "for" || name == "while" || name == "class" || name == "print" {
					continue
				}
				kind := "calls"
				if decl.Kind == "test" {
					kind = "tests"
				}
				if target, ok := pythonCallTarget(index.byName, name, decl, lines, byDecl); ok {
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
			index.byName[symbol.Name] = append(index.byName[symbol.Name], symbol)
		}
	}
	return index
}
func uniquePython(values map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := values[name]
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
	active := make([]*pyDeclaration, 0)
	statements := make([]bool, len(lines))
	depth := 0
	decorators := make([]string, 0)
	for index, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		continuation := depth > 0
		for _, ch := range text {
			switch ch {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth > 0 {
					depth--
				}
			}
		}
		if continuation {
			continue
		}
		statements[index] = true
		for len(active) > 0 && active[len(active)-1].Indent >= line.Indent {
			active = active[:len(active)-1]
		}
		if strings.HasPrefix(text, "@") {
			decorator := strings.TrimSpace(line.Original)
			if decorator == "" {
				decorator = text
			}
			decorators = append(decorators, decorator)
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
			parent := nearestPythonParent(active, line.Indent, index+1)
			if parent != nil && (parent.Kind == "class" || parent.Kind == "protocol") {
				if kind != "test" {
					kind = "method"
					if match[1] == "async" {
						kind = "async_method"
					}
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
		} else if parent := nearestPythonParent(active, line.Indent, index+1); parent != nil && (parent.Kind == "class" || parent.Kind == "protocol") && pyClassVarPattern.MatchString(text) {
			match := pyClassVarPattern.FindStringSubmatch(text)
			decl = &pyDeclaration{Kind: "class_variable", Name: match[1], StartLine: index + 1, Indent: line.Indent, Header: text, Parent: parent, Decorators: decorators}
		}
		if decl != nil {
			if decl.Parent == nil && (decl.Kind == "class" || decl.Kind == "protocol") {
				decl.Parent = nearestPythonParent(active, line.Indent, index+1)
			}
			if decl.Parent != nil {
				decl.Qualified = decl.Parent.Qualified + "." + decl.Name
			} else {
				decl.Qualified = decl.Name
			}
			declarations = append(declarations, decl)
			if decl.Kind != "type_alias" && decl.Kind != "class_variable" {
				active = append(active, decl)
			}
			decorators = nil
		}
	}
	for index, decl := range declarations {
		end := len(lines)
		for line := decl.StartLine; line < len(lines); line++ {
			if !statements[line] {
				continue
			}
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

// pythonCallTarget resolves only when the lexical binding and receiver agree.
func pythonCallTarget(index map[string][]model.Symbol, name string, decl *pyDeclaration, lines []pyLine, symbols map[*pyDeclaration]model.Symbol) (model.Symbol, bool) {
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		receiver, member := name[:dot], name[dot+1:]
		if (receiver != "self" && receiver != "cls") || strings.Contains(member, ".") {
			return model.Symbol{}, false
		}
		for owner := decl.Parent; owner != nil; owner = owner.Parent {
			if owner.Kind == "class" || owner.Kind == "protocol" {
				var target model.Symbol
				count := 0
				for _, candidate := range index[member] {
					if candidate.ParentHandle == symbols[owner].Handle {
						target = candidate
						count++
					}
				}
				return target, count == 1
			}
		}
		return model.Symbol{}, false
	}
	for scope := decl; scope != nil; scope = scope.Parent {
		if scope.Kind == "class" || scope.Kind == "protocol" {
			continue
		}
		if pythonBindsName(scope, name, lines, symbols) {
			return model.Symbol{}, false
		}
		var target model.Symbol
		count := 0
		for nested, candidate := range symbols {
			if nested.Parent == scope && nested.Name == name {
				target = candidate
				count++
			}
		}
		if count > 0 {
			return target, count == 1
		}
	}
	var target model.Symbol
	count := 0
	for _, candidate := range index[name] {
		if candidate.ParentHandle == "" {
			target = candidate
			count++
		}
	}
	return target, count == 1
}

func pythonBindsName(decl *pyDeclaration, name string, lines []pyLine, symbols map[*pyDeclaration]model.Symbol) bool {
	header := ""
	for line := decl.StartLine - 1; line < decl.EndLine && line < len(lines); line++ {
		header += lines[line].Text + " "
		if strings.Contains(lines[line].Text, ")") {
			break
		}
	}
	if open := strings.IndexByte(header, '('); open >= 0 {
		depth, start := 0, open+1
		for cursor := start; cursor < len(header); cursor++ {
			ch := header[cursor]
			if depth == 0 && (ch == ',' || ch == ')') {
				parameter := strings.TrimLeft(strings.TrimSpace(header[start:cursor]), "*")
				if pyParameterPattern.FindString(parameter) == name {
					return true
				}
				if ch == ')' {
					break
				}
				start = cursor + 1
			}
			switch ch {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
	}
	for line := decl.StartLine; line < decl.EndLine && line < len(lines); line++ {
		if end := pythonNestedScopeEnd(decl, line+1, symbols); end > 0 {
			line = end - 1
			continue
		}
		text := strings.TrimSpace(lines[line].Text)
		if strings.HasPrefix(text, "import ") || strings.HasPrefix(text, "from ") {
			for (strings.Count(text, "(") > strings.Count(text, ")") || strings.HasSuffix(strings.TrimSpace(text), `\`)) && line+1 < decl.EndLine && line+1 < len(lines) {
				line++
				text += " " + strings.TrimSpace(lines[line].Text)
			}
			if pythonImportBindsName(text, name) {
				return true
			}
		}
		if pythonStatementBindsName(text, name) {
			return true
		}
	}
	return false
}

func pythonImportBindsName(text, name string) bool {
	text = strings.NewReplacer("(", " ", ")", " ", `\`, " ").Replace(text)
	words := strings.Fields(text)
	if len(words) < 2 {
		return false
	}
	from := words[0] == "from"
	start := 1
	if from {
		if len(words) < 4 || words[2] != "import" {
			return false
		}
		start = 3
	}
	for _, item := range strings.Split(strings.Join(words[start:], " "), ",") {
		fields := strings.Fields(item)
		if len(fields) == 0 {
			continue
		}
		binding := fields[0]
		if len(fields) >= 3 && fields[1] == "as" {
			binding = fields[2]
		} else if !from {
			binding = strings.Split(binding, ".")[0]
		}
		if binding == name || binding == "*" {
			return true
		}
	}
	return false
}

func pythonNestedScopeEnd(decl *pyDeclaration, line int, symbols map[*pyDeclaration]model.Symbol) int {
	for nested := range symbols {
		if nested.Parent == decl && nested.Kind != "type_alias" && nested.Kind != "class_variable" && line >= nested.StartLine && line <= nested.EndLine {
			return nested.EndLine
		}
	}
	return 0
}

// Binding targets are identifiers or destructuring targets. Attribute and index
// assignment changes an object, so it does not shadow a bare function name.
func pythonTargetBindsName(target, name string) bool {
	target = strings.TrimSpace(target)
	for len(target) >= 2 && (target[0] == '(' && target[len(target)-1] == ')' || target[0] == '[' && target[len(target)-1] == ']') {
		target = strings.TrimSpace(target[1 : len(target)-1])
	}
	parts := pythonTopLevelParts(target, ',')
	if len(parts) > 1 {
		for _, part := range parts {
			if pythonTargetBindsName(part, name) {
				return true
			}
		}
		return false
	}
	target = strings.TrimLeft(target, "*")
	return pyBindingIdentifier.MatchString(target) && target == name
}

func pythonTopLevelParts(text string, separator byte) []string {
	depth, start := 0, 0
	parts := make([]string, 0)
	for at := 0; at < len(text); at++ {
		switch text[at] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
		if text[at] == separator && depth == 0 {
			parts = append(parts, text[start:at])
			start = at + 1
		}
	}
	return append(parts, text[start:])
}

func pythonStatementBindsName(text, name string) bool {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "async ")
	if strings.HasPrefix(text, "for ") {
		target := strings.TrimPrefix(text, "for ")
		if end := strings.Index(target, " in "); end >= 0 {
			return pythonTargetBindsName(target[:end], name)
		}
		return false
	}
	if strings.HasPrefix(text, "with ") {
		for _, item := range pythonTopLevelParts(strings.TrimSuffix(strings.TrimPrefix(text, "with "), ":"), ',') {
			if at := strings.LastIndex(item, " as "); at >= 0 && pythonTargetBindsName(item[at+4:], name) {
				return true
			}
		}
		return false
	}
	depth := 0
	for at := 0; at < len(text); at++ {
		switch text[at] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 {
			continue
		}
		if text[at] == ':' {
			return pythonTargetBindsName(text[:at], name)
		}
		if text[at] == '=' && (at == 0 || !strings.ContainsRune("=<>!:", rune(text[at-1]))) && (at+1 == len(text) || text[at+1] != '=') {
			return pythonTargetBindsName(text[:at], name)
		}
	}
	return false
}
