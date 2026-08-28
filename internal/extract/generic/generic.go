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
	case "python", "ruby", "powershell", "shell":
		return indentation(file), nil
	case "c", "cpp", "csharp", "rust", "java", "javascript", "typescript", "php":
		return braceBalanced(file), nil
	default:
		return windows(file, 80, 10), nil
	}
}

var declarationPattern = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|async|final|virtual|override|export|unsafe)\s+)*(class|struct|interface|enum|namespace|trait|impl|fn|function)\s+([A-Za-z_][A-Za-z0-9_]*)`)

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
