package zig

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type zigLine struct {
	Start int
	End   int
	Text  string
}

type zigDecl struct {
	Kind      string
	Name      string
	Qualified string
	Header    string
	StartLine int
	EndLine   int
	Depth     int
	Block     bool
	Parent    *zigDecl
}

type zigParseResult struct {
	Owner       *zigDecl
	Decls       []*zigDecl
	Lines       []zigLine
	DepthAfter  []int
	Diagnostics []model.Diagnostic
}

var (
	zigTypePattern     = regexp.MustCompile(`(?i)^\s*(?:(?:pub|export|extern)\s+)*const\s+([a-z_][a-z0-9_]*)\s*=\s*(struct|enum|union|opaque)\b`)
	zigFunctionPattern = regexp.MustCompile(`(?i)^\s*(?:(?:pub|export|extern)\s+)*(?:(?:inline|noinline|callconv\([^)]*\))\s+)*(?:fn)\s+([a-z_][a-z0-9_]*)`)
	zigConstPattern    = regexp.MustCompile(`(?i)^\s*(?:(?:pub|export)\s+)?const\s+([a-z_][a-z0-9_]*)\b`)
	zigVarPattern      = regexp.MustCompile(`(?i)^\s*(?:(?:pub)\s+)?var\s+([a-z_][a-z0-9_]*)\b`)
	zigTestPattern     = regexp.MustCompile(`(?i)^\s*test\s+["']([^"']+)["']`)
	zigImportPattern   = regexp.MustCompile(`@import\s*\(\s*["']([^"']+)["']\s*\)`)
)

func parseZig(file model.SourceFile) zigParseResult {
	lines := zigLines(file.Content)
	result := zigParseResult{Lines: lines, DepthAfter: make([]int, len(lines))}
	name := filepath.Base(file.Path)
	result.Owner = &zigDecl{Kind: "module", Name: name, Qualified: name, Header: "module " + name, StartLine: 1, EndLine: len(lines)}
	depth := 0
	for index, line := range lines {
		text := strings.TrimSpace(zigCode(line.Text))
		if text == "" || strings.HasPrefix(text, "//") {
			result.DepthAfter[index] = depth
			continue
		}
		before := depth
		var decl *zigDecl
		if match := zigTypePattern.FindStringSubmatch(text); len(match) == 3 {
			decl = newZigDecl(strings.ToLower(match[2]), match[1], text, index+1, before, strings.Contains(text, "{"), nil)
		} else if match := zigFunctionPattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newZigDecl("function", match[1], text, index+1, before, strings.Contains(text, "{"), nil)
		} else if match := zigTestPattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newZigDecl("test", match[1], text, index+1, before, strings.Contains(text, "{"), nil)
		} else if strings.HasPrefix(strings.ToLower(text), "comptime") {
			decl = newZigDecl("comptime", "comptime", text, index+1, before, strings.Contains(text, "{"), nil)
		} else if strings.HasPrefix(strings.ToLower(text), "usingnamespace ") {
			decl = newZigDecl("usingnamespace", "usingnamespace", text, index+1, before, false, nil)
		} else if match := zigConstPattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newZigDecl("constant", match[1], text, index+1, before, false, nil)
		} else if match := zigVarPattern.FindStringSubmatch(text); len(match) == 2 {
			decl = newZigDecl("variable", match[1], text, index+1, before, false, nil)
		}
		if decl != nil {
			result.Decls = append(result.Decls, decl)
		}
		depth += strings.Count(text, "{") - strings.Count(text, "}")
		if depth < 0 {
			depth = 0
		}
		result.DepthAfter[index] = depth
	}
	for _, decl := range result.Decls {
		if !decl.Block {
			decl.EndLine = decl.StartLine
			continue
		}
		found := false
		for index := decl.StartLine - 1; index < len(lines); index++ {
			if result.DepthAfter[index] <= decl.Depth && strings.Contains(lines[index].Text, "}") {
				decl.EndLine = index + 1
				found = true
				break
			}
		}
		if !found {
			decl.EndLine = len(lines)
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "zig_unclosed_block", Message: "Zig block " + decl.Name + " is not closed; span recovered to end of file"})
		}
	}
	for _, decl := range result.Decls {
		for _, candidate := range result.Decls {
			if candidate == decl || candidate.StartLine >= decl.StartLine || !candidate.Block || candidate.EndLine < decl.EndLine {
				continue
			}
			if decl.Parent == nil || candidate.StartLine > decl.Parent.StartLine {
				decl.Parent = candidate
			}
		}
		if decl.Parent != nil {
			decl.Qualified = decl.Parent.Qualified + "." + decl.Name
		}
	}
	return result
}

func newZigDecl(kind, name, header string, line, depth int, block bool, parent *zigDecl) *zigDecl {
	qualified := name
	if parent != nil {
		qualified = parent.Qualified + "." + name
	}
	return &zigDecl{Kind: kind, Name: name, Qualified: qualified, Header: header, StartLine: line, EndLine: line, Depth: depth, Block: block, Parent: parent}
}

func zigLines(source []byte) []zigLine {
	starts := []int{0}
	for at, value := range source {
		if value == '\n' && at+1 < len(source) {
			starts = append(starts, at+1)
		}
	}
	lines := make([]zigLine, 0, len(starts))
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for end > start && (source[end-1] == '\r' || source[end-1] == '\n') {
			end--
		}
		lines = append(lines, zigLine{Start: start, End: end, Text: string(source[start:end])})
	}
	return lines
}

func zigCode(text string) string {
	for at := 0; at < len(text); at++ {
		if text[at] == '"' {
			at++
			for at < len(text) {
				if text[at] == '\\' {
					at += 2
					continue
				}
				if text[at] == '"' {
					break
				}
				at++
			}
			continue
		}
		if text[at] == '/' && at+1 < len(text) && text[at+1] == '/' {
			return text[:at]
		}
	}
	return text
}
