package nim

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type nimLine struct {
	Start  int
	End    int
	Indent int
	Text   string
}

type nimDecl struct {
	Kind      string
	Name      string
	Qualified string
	Header    string
	StartLine int
	EndLine   int
	Indent    int
	Parent    *nimDecl
}

type nimParseResult struct {
	Owner       *nimDecl
	Decls       []*nimDecl
	Lines       []nimLine
	Diagnostics []model.Diagnostic
}

var (
	nimRoutinePattern   = regexp.MustCompile(`(?i)^(proc|func|method|iterator|converter|template|macro)\s+([^\s(:=]+|` + "`[^`]+`" + `)`)
	nimNamedTypePattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*\*?)\s*=\s*(enum|object|distinct|concept)\b`)
	nimValuePattern     = regexp.MustCompile(`(?i)^(const|let|var)\s+([a-z_][a-z0-9_]*\*?)\b`)
	nimTestPattern      = regexp.MustCompile(`(?i)^test\s+["']([^"']+)["']`)
	nimSuitePattern     = regexp.MustCompile(`(?i)^suite\s+["']([^"']+)["']`)
)

func parseNim(file model.SourceFile) nimParseResult {
	lines := nimSourceLines(file.Content)
	result := nimParseResult{Lines: lines}
	name := filepath.Base(file.Path)
	result.Owner = &nimDecl{Kind: "module", Name: name, Qualified: name, Header: "module " + name, StartLine: 1, EndLine: len(lines), Indent: -1}
	stack := make([]*nimDecl, 0, 8)
	for index, line := range lines {
		lineNo := index + 1
		text := strings.TrimSpace(nimCode(line.Text))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		for len(stack) > 0 && line.Indent <= stack[len(stack)-1].Indent {
			if stack[len(stack)-1].EndLine < lineNo-1 {
				stack[len(stack)-1].EndLine = lineNo - 1
			}
			stack = stack[:len(stack)-1]
		}
		parent := (*nimDecl)(nil)
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		var decl *nimDecl
		if match := nimRoutinePattern.FindStringSubmatch(text); len(match) == 3 {
			decl = newNimDecl(strings.ToLower(match[1]), nimTrimExport(nimTrimBacktick(match[2])), text, lineNo, line.Indent, parent)
		} else if match := nimNamedTypePattern.FindStringSubmatch(text); len(match) == 3 {
			decl = newNimDecl(strings.ToLower(match[2]), nimTrimExport(match[1]), text, lineNo, line.Indent, parent)
		} else if match := nimValuePattern.FindStringSubmatch(text); len(match) == 3 {
			decl = newNimDecl(strings.ToLower(match[1]), nimTrimExport(match[2]), text, lineNo, line.Indent, parent)
		} else if match := nimTestPattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newNimDecl("test", match[1], text, lineNo, line.Indent, parent)
		} else if match := nimSuitePattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newNimDecl("suite", match[1], text, lineNo, line.Indent, parent)
		}
		if decl == nil {
			continue
		}
		result.Decls = append(result.Decls, decl)
		stack = append(stack, decl)
	}
	for _, decl := range stack {
		decl.EndLine = len(lines)
	}
	for _, decl := range result.Decls {
		if decl.EndLine < decl.StartLine {
			decl.EndLine = decl.StartLine
		}
	}
	return result
}

func newNimDecl(kind, name, header string, line, indent int, parent *nimDecl) *nimDecl {
	qualified := name
	if parent != nil && parent.Qualified != "" {
		qualified = parent.Qualified + "." + name
	}
	return &nimDecl{Kind: kind, Name: name, Qualified: qualified, Header: header, StartLine: line, EndLine: line, Indent: indent, Parent: parent}
}

func nimSourceLines(source []byte) []nimLine {
	starts := []int{0}
	for at, value := range source {
		if value == '\n' && at+1 < len(source) {
			starts = append(starts, at+1)
		}
	}
	lines := make([]nimLine, 0, len(starts))
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for end > start && (source[end-1] == '\r' || source[end-1] == '\n') {
			end--
		}
		indent := 0
		for at := start; at < end; at++ {
			if source[at] == ' ' {
				indent++
			} else if source[at] == '\t' {
				indent += 2
			} else {
				break
			}
		}
		lines = append(lines, nimLine{Start: start, End: end, Indent: indent, Text: string(source[start:end])})
	}
	return lines
}

func nimCode(text string) string {
	for at := 0; at < len(text); at++ {
		if text[at] == '"' {
			at++
			for at < len(text) {
				if text[at] == '"' {
					if at+1 < len(text) && text[at+1] == '"' {
						at += 2
						continue
					}
					break
				}
				at++
			}
			continue
		}
		if text[at] == '#' {
			return text[:at]
		}
	}
	return text
}

func nimTrimExport(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), "*")
}

func nimTrimBacktick(value string) string {
	return strings.Trim(value, "`")
}

func nimLineCount(source []byte) int {
	return 1 + strings.Count(string(source), "\n")
}

func nimFileOwner(file model.SourceFile) *nimDecl {
	name := filepath.Base(file.Path)
	return &nimDecl{Kind: "module", Name: name, Qualified: name, Header: "module " + name, StartLine: 1, EndLine: nimLineCount(file.Content), Indent: -1}
}
