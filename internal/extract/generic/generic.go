package generic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor { return Extractor{} }

func (Extractor) Name() string { return "generic-structural" }

func (Extractor) Supports(path, language string) bool {
	if language == "go" || strings.EqualFold(filepath.Ext(path), ".go") {
		return false
	}
	return true
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	switch file.Language {
	case "markdown":
		return markdown(file), nil
	case "powershell", "shell":
		return indentation(file), nil
	case "csharp":
		return csharpStructured(file), nil
	case "javascript", "typescript":
		return javascriptStructured(file), nil
	case "c", "cpp", "java", "php":
		return braceBalanced(file), nil
	default:
		return windows(file, 80, 10), nil
	}
}

var declarationPattern = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|async|final|virtual|override|export|unsafe)\s+)*(class|struct|interface|enum|namespace|trait|impl|fn|function)\s+([A-Za-z_][A-Za-z0-9_]*)`)

type bracePair struct {
	openLine   int
	openColumn int
	closeLine  int
}

type languageDeclaration struct {
	kind       string
	name       string
	startLine  int
	bodyColumn int
	needsBrace bool
}

type declarationMatcher func(string) (languageDeclaration, bool)

var (
	csharpTypePattern     = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|partial|unsafe|new|file|readonly)\s+)*(class|struct|interface|enum|record(?:\s+(?:class|struct))?|namespace)\s+([A-Za-z_][A-Za-z0-9_.]*)`)
	csharpMethodPattern   = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|virtual|override|async|extern|unsafe|new|partial|readonly|ref|out|in)\s+)*(?:[A-Za-z_][A-Za-z0-9_<>,.?\[\]]*\s+)*([A-Za-z_][A-Za-z0-9_]*)(?:<[^>{}()]*>)?\s*\([^)]*\)\s*(?:where\b[^{}]*)?(?:\{|=>|;)?`)
	csharpPropertyPattern = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|virtual|override|readonly|required|new)\s+)*(?:[A-Za-z_][A-Za-z0-9_<>,.?\[\]]*\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\{|=>)`)

	javascriptTypePattern      = regexp.MustCompile(`^\s*(?:(?:export|default|abstract|declare)\s+)*(class|interface|enum|namespace|module)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	javascriptTypeAliasPattern = regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?type\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	javascriptFunctionPattern  = regexp.MustCompile(`^\s*(?:(?:export|default|async|declare)\s+)*function\s*\*?\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	javascriptFunctionValue    = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?function\s*\*?\s*\(`)
	javascriptTypedArrow       = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*:\s*\([^)]*\)\s*=>\s*[A-Za-z_$][A-Za-z0-9_$<>,.?\[\] ]*\s*=\s*`)
	javascriptArrowPattern     = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*(?::[^=]+)?=>`)
	javascriptArrowProperty    = regexp.MustCompile(`^\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*:\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*(?::[^=]+)?=>`)
	javascriptMethodPattern    = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|async|get|set|abstract|override|readonly)\s+)*([A-Za-z_$][A-Za-z0-9_$]*)(?:<[^>{}()]*>)?\s*\([^)]*\)\s*(?::[^{}]+)?(?:\{|=>)?`)
)

var controlDeclarationWords = map[string]bool{
	"break": true, "catch": true, "checked": true, "do": true, "else": true,
	"finally": true, "fixed": true, "for": true, "foreach": true, "if": true,
	"lock": true, "nameof": true, "return": true, "sizeof": true, "switch": true,
	"throw": true, "try": true, "typeof": true, "unchecked": true, "unless": true,
	"using": true, "while": true,
}

func csharpStructured(file model.SourceFile) model.Extraction {
	return structured(file, matchCSharpDeclaration)
}

func javascriptStructured(file model.SourceFile) model.Extraction {
	return structured(file, matchJavaScriptDeclaration)
}

func structured(file model.SourceFile, match declarationMatcher) model.Extraction {
	lines := splitLines(string(file.Content))
	pairs, codeLines := scanSource(lines)
	result := model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "generic_extraction", Message: "language-aware structural extraction is syntax-approximate"}}}
	allocator := model.NewHandleAllocator()
	for line := range codeLines {
		declaration, code, ok := declarationAt(codeLines, line, match)
		if !ok {
			continue
		}
		declaration.startLine = line
		declaration.bodyColumn = bodyColumn(code, declaration.kind)
		end := declaration.startLine
		if declaration.needsBrace {
			if pair, found := bodyPair(pairs, declaration.startLine, declaration.bodyColumn); found {
				end = pair.closeLine
			} else {
				end = len(lines) - 1
			}
		}
		result = appendGeneric(result, file, allocator, lines, declaration.startLine, end, declaration.kind, declaration.name)
	}
	if len(result.Chunks) == 0 {
		return windows(file, 80, 10)
	}
	return result
}

func declarationAt(lines []string, start int, match declarationMatcher) (languageDeclaration, string, bool) {
	code := lines[start]
	if declaration, ok := match(code); ok {
		return declaration, code, true
	}
	for line := start + 1; line < len(lines) && line < start+16; line++ {
		code += " " + strings.TrimSpace(lines[line])
		if declaration, ok := match(code); ok {
			return declaration, code, true
		}
	}
	return languageDeclaration{}, "", false
}

func matchCSharpDeclaration(line string) (languageDeclaration, bool) {
	if match := csharpTypePattern.FindStringSubmatch(line); len(match) == 3 {
		kind := strings.ToLower(match[1])
		if strings.HasPrefix(kind, "record") {
			kind = "record"
		}
		return languageDeclaration{kind: kind, name: match[2], needsBrace: !strings.Contains(line, ";")}, true
	}
	if match := csharpMethodPattern.FindStringSubmatch(line); len(match) == 2 && !controlDeclarationWords[strings.ToLower(match[1])] {
		return languageDeclaration{kind: "method", name: match[1], needsBrace: strings.Contains(line, "{") || !strings.Contains(line, ";") && !strings.Contains(line, "=>")}, true
	}
	if match := csharpPropertyPattern.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "property", name: match[1], needsBrace: strings.Contains(line, "{")}, true
	}
	return languageDeclaration{}, false
}

func matchJavaScriptDeclaration(line string) (languageDeclaration, bool) {
	if match := javascriptTypePattern.FindStringSubmatch(line); len(match) == 3 {
		return languageDeclaration{kind: strings.ToLower(match[1]), name: match[2], needsBrace: !strings.Contains(line, ";")}, true
	}
	if match := javascriptTypeAliasPattern.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "type", name: match[1], needsBrace: strings.Contains(line, "{")}, true
	}
	if match := javascriptFunctionPattern.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "function", name: match[1], needsBrace: strings.Contains(line, "{") || !strings.Contains(line, ";")}, true
	}
	if match := javascriptFunctionValue.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "function", name: match[1], needsBrace: strings.Contains(line, "{")}, true
	}
	if match := javascriptTypedArrow.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "arrow_function", name: match[1], needsBrace: strings.Contains(line, "{")}, true
	}
	if match := javascriptArrowPattern.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "arrow_function", name: match[1], needsBrace: strings.Contains(line, "{") || !strings.Contains(line, ";")}, true
	}
	if match := javascriptArrowProperty.FindStringSubmatch(line); len(match) == 2 {
		return languageDeclaration{kind: "arrow_function", name: match[1], needsBrace: strings.Contains(line, "{")}, true
	}
	if match := javascriptMethodPattern.FindStringSubmatch(line); len(match) == 2 && !controlDeclarationWords[strings.ToLower(match[1])] {
		return languageDeclaration{kind: "method", name: match[1], needsBrace: strings.Contains(line, "{") || !strings.Contains(line, ";") && !strings.Contains(line, "=>")}, true
	}
	return languageDeclaration{}, false
}

func bodyColumn(line, kind string) int {
	if kind == "class" || kind == "struct" || kind == "interface" || kind == "enum" || kind == "record" || kind == "namespace" || kind == "type" {
		return 0
	}
	if kind == "arrow_function" {
		return strings.Index(line, "=>") + 2
	}
	if end := strings.LastIndex(line, ")"); end >= 0 {
		if open := strings.IndexByte(line[end+1:], '{'); open >= 0 {
			return end + 1 + open
		}
	}
	return 0
}

func bodyPair(pairs []bracePair, startLine, minColumn int) (bracePair, bool) {
	var best bracePair
	found := false
	for _, pair := range pairs {
		if pair.openLine == startLine && pair.openColumn < minColumn {
			continue
		}
		if pair.openLine < startLine || found && (pair.openLine > best.openLine || pair.openLine == best.openLine && pair.openColumn >= best.openColumn) {
			continue
		}
		best, found = pair, true
	}
	return best, found
}

func scanSource(lines []string) ([]bracePair, []string) {
	cleaned := make([]string, len(lines))
	pairs := make([]bracePair, 0)
	stack := make([]bracePair, 0)
	blockComment := false
	quote := ""
	escaped := false
	for lineIndex, line := range lines {
		raw := []byte(line)
		code := append([]byte(nil), raw...)
		for column := 0; column < len(raw); {
			if blockComment {
				code[column] = ' '
				if column+1 < len(raw) && raw[column] == '*' && raw[column+1] == '/' {
					code[column+1] = ' '
					blockComment = false
					column += 2
					continue
				}
				column++
				continue
			}
			if quote != "" {
				if strings.HasPrefix(line[column:], quote) {
					for end := column; end < column+len(quote) && end < len(code); end++ {
						code[end] = ' '
					}
					column += len(quote)
					quote = ""
					escaped = false
					continue
				}
				code[column] = ' '
				if escaped {
					escaped = false
					column++
					continue
				}
				if raw[column] == '\\' {
					escaped = true
				}
				column++
				continue
			}
			if column+1 < len(raw) && raw[column] == '/' && raw[column+1] == '/' {
				for end := column; end < len(code); end++ {
					code[end] = ' '
				}
				break
			}
			if column+1 < len(raw) && raw[column] == '/' && raw[column+1] == '*' {
				code[column], code[column+1] = ' ', ' '
				blockComment = true
				column += 2
				continue
			}
			if raw[column] == '"' || raw[column] == '\'' || raw[column] == '`' {
				quote = string(raw[column])
				if raw[column] == '"' && column+2 < len(raw) && raw[column+1] == '"' && raw[column+2] == '"' {
					quote = `"""`
				}
				for end := column; end < column+len(quote) && end < len(code); end++ {
					code[end] = ' '
				}
				column += len(quote)
				continue
			}
			switch raw[column] {
			case '{':
				open := bracePair{openLine: lineIndex, openColumn: column}
				stack = append(stack, open)
			case '}':
				if len(stack) > 0 {
					open := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					open.closeLine = lineIndex
					pairs = append(pairs, open)
				}
			}
			column++
		}
		cleaned[lineIndex] = string(code)
	}
	return pairs, cleaned
}

func braceBalanced(file model.SourceFile) model.Extraction {
	lines := splitLines(string(file.Content))
	result := model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "generic_extraction", Message: "generic structural extraction is syntax-approximate"}}}
	allocator := model.NewHandleAllocator()
	depth := 0
	start := -1
	kind, name := "", ""
	for i, line := range lines {
		if start < 0 && depth == 0 {
			if match := declarationPattern.FindStringSubmatch(line); len(match) == 3 {
				start, kind, name = i, strings.ToLower(match[1]), match[2]
			}
		}
		before := depth
		depth += braceDelta(line)
		if depth < 0 {
			depth = 0
		}
		if start >= 0 && before == 0 && depth == 0 {
			result = appendGeneric(result, file, allocator, lines, start, i, kind, name)
			start, kind, name = -1, "", ""
		} else if start >= 0 && depth == 0 && before > 0 {
			result = appendGeneric(result, file, allocator, lines, start, i, kind, name)
			start, kind, name = -1, "", ""
		}
	}
	if start >= 0 {
		result = appendGeneric(result, file, allocator, lines, start, len(lines)-1, kind, name)
	}
	if len(result.Chunks) == 0 {
		return windows(file, 80, 10)
	}
	return result
}

func braceDelta(line string) int {
	delta := 0
	inBlockComment, inString, inChar, escaped := false, false, false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inBlockComment {
			if c == '*' && i+1 < len(line) && line[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString || inChar {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if inString && c == '"' {
				inString = false
			} else if inChar && c == '\'' {
				inChar = false
			}
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '\'' {
			inChar = true
			continue
		}
		if c == '{' {
			delta++
		} else if c == '}' {
			delta--
		}
	}
	return delta
}

func indentation(file model.SourceFile) model.Extraction {
	lines := splitLines(string(file.Content))
	result := model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "generic_extraction", Message: "indentation extraction is syntax-approximate"}}}
	allocator := model.NewHandleAllocator()
	starts := make([]int, 0)
	names := make([]string, 0)
	kinds := make([]string, 0)
	pattern := regexp.MustCompile(`^(def|class|function)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	for i, line := range lines {
		trim := strings.TrimLeftFunc(line, unicode.IsSpace)
		if len(line)-len(trim) != 0 {
			continue
		}
		if match := pattern.FindStringSubmatch(trim); len(match) == 3 {
			starts = append(starts, i)
			kinds = append(kinds, match[1])
			names = append(names, match[2])
		}
	}
	for i, start := range starts {
		end := len(lines) - 1
		if i+1 < len(starts) {
			end = starts[i+1] - 1
		}
		result = appendGeneric(result, file, allocator, lines, start, end, kinds[i], names[i])
	}
	if len(result.Chunks) == 0 {
		return windows(file, 80, 10)
	}
	return result
}

func markdown(file model.SourceFile) model.Extraction {
	lines := splitLines(string(file.Content))
	result := model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "markdown_extraction", Message: "Markdown is split by heading"}}}
	allocator := model.NewHandleAllocator()
	starts := make([]int, 0)
	names := make([]string, 0)
	for i, line := range lines {
		trim := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trim, "#") {
			continue
		}
		level := 0
		for level < len(trim) && trim[level] == '#' {
			level++
		}
		if level == len(trim) || !unicode.IsSpace(rune(trim[level])) {
			continue
		}
		starts = append(starts, i)
		names = append(names, strings.TrimSpace(trim[level:]))
	}
	for i, start := range starts {
		end := len(lines) - 1
		if i+1 < len(starts) {
			end = starts[i+1] - 1
		}
		result = appendGeneric(result, file, allocator, lines, start, end, "heading", names[i])
	}
	if len(result.Chunks) == 0 {
		return windows(file, 80, 10)
	}
	return result
}

func windows(file model.SourceFile, size, overlap int) model.Extraction {
	lines := splitLines(string(file.Content))
	result := model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "line_window", Message: "fallback line windows used"}}}
	allocator := model.NewHandleAllocator()
	if len(lines) == 0 {
		return result
	}
	for start := 0; start < len(lines); {
		end := start + size
		if end > len(lines) {
			end = len(lines)
		}
		result = appendGeneric(result, file, allocator, lines, start, end-1, "window", "")
		if end == len(lines) {
			break
		}
		start = end - overlap
	}
	return result
}

func appendGeneric(result model.Extraction, file model.SourceFile, allocator *model.HandleAllocator, lines []string, start, end int, kind, name string) model.Extraction {
	if start < 0 || end < start || start >= len(lines) {
		return result
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	content := strings.Join(lines[start:end+1], "\n")
	qualified := name
	handle := allocator.Allocate("sym", file.Path, file.Language, kind, qualified, model.NormalizeSignature(strings.TrimSpace(firstNonEmpty(lines[start:end+1]))))
	symbol := model.Symbol{Handle: handle, FilePath: file.Path, Language: file.Language, Kind: kind, Name: name, QualifiedName: qualified, Signature: strings.TrimSpace(firstNonEmpty(lines[start : end+1])), StartLine: start + 1, EndLine: end + 1, StartByte: byteOffset(lines, start), EndByte: byteOffset(lines, end+1), Confidence: .55}
	result.Symbols = append(result.Symbols, symbol)
	digest := sha256.Sum256([]byte(content))
	result.Chunks = append(result.Chunks, model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: handle, SymbolName: name, Signature: symbol.Signature, StartLine: start + 1, EndLine: end + 1, StartByte: symbol.StartByte, EndByte: symbol.EndByte, Content: content, ContentHash: hex.EncodeToString(digest[:])})
	return result
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{""}
	}
	return strings.Split(content, "\n")
}

func firstNonEmpty(lines []string) string {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

func byteOffset(lines []string, line int) int {
	offset := 0
	for i := 0; i < line && i < len(lines); i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}
