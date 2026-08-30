package lua

import (
	"regexp"
	"strings"
)

type luaLine struct {
	Start, End int
	Raw, Code  string
}

type luaDeclaration struct {
	Kind, Name, Qualified, Header string
	StartLine, EndLine, Depth     int
	Parent                        *luaDeclaration
}

var (
	luaNamedFunction = regexp.MustCompile(`^\s*(?:local\s+)?function\s+([A-Za-z_]\w*(?:[.:][A-Za-z_]\w*)*)\s*\(`)
	luaFunctionValue = regexp.MustCompile(`^\s*(?:local\s+)?([A-Za-z_]\w*(?:[.:][A-Za-z_]\w*)*)\s*=\s*function\b`)
	luaTable         = regexp.MustCompile(`^\s*(?:local\s+)?([A-Za-z_]\w*)\s*=\s*\{`)
	luaBustedTest    = regexp.MustCompile(`^\s*(?:it|specify)\s*(?:\(\s*)?["']([^"']+)["']`)
	luaBlockWord     = regexp.MustCompile(`\b(function|do|then|repeat|end|until)\b`)
	luaCall          = regexp.MustCompile(`\b([A-Za-z_]\w*(?:[.:][A-Za-z_]\w*)?)\s*\(`)
)

func luaLines(source []byte, tokens []Token) []luaLine {
	starts := []int{0}
	for index, value := range source {
		if value == '\n' && index+1 < len(source) {
			starts = append(starts, index+1)
		}
	}
	lines := make([]luaLine, 0, len(starts))
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for end > start && (source[end-1] == '\n' || source[end-1] == '\r') {
			end--
		}
		raw := string(source[start:end])
		code := []byte(raw)
		for _, token := range tokens {
			if token.EndByte <= start || token.StartByte >= end || token.Kind == Identifier || token.Kind == BlockKeyword || token.Kind == Punctuation {
				continue
			}
			from := token.StartByte - start
			to := token.EndByte - start
			if from < 0 {
				from = 0
			}
			if to > len(code) {
				to = len(code)
			}
			for at := from; at < to; at++ {
				code[at] = ' '
			}
		}
		lines = append(lines, luaLine{Start: start, End: end, Raw: raw, Code: string(code)})
	}
	return lines
}

func parseLua(lines []luaLine) []*luaDeclaration {
	declarations := make([]*luaDeclaration, 0)
	type frame struct {
		decl  *luaDeclaration
		depth int
	}
	stack := make([]frame, 0)
	depth := 0
	for index, line := range lines {
		code := strings.TrimSpace(line.Code)
		if code == "" {
			continue
		}
		closing := luaClosingBlocks(code)
		depth -= closing
		if depth < 0 {
			depth = 0
		}
		var decl *luaDeclaration
		if match := luaBustedTest.FindStringSubmatch(line.Raw); len(match) == 2 {
			decl = &luaDeclaration{Kind: "test", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		} else if match := luaNamedFunction.FindStringSubmatch(code); len(match) == 2 {
			decl = luaFunctionDeclaration(match[1], strings.TrimSpace(line.Raw), index+1, depth)
		} else if match := luaFunctionValue.FindStringSubmatch(code); len(match) == 2 {
			decl = luaFunctionDeclaration(match[1], strings.TrimSpace(line.Raw), index+1, depth)
		} else if match := luaTable.FindStringSubmatch(code); len(match) == 2 {
			decl = &luaDeclaration{Kind: "table", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		}
		if decl != nil {
			if len(stack) > 0 && stack[len(stack)-1].decl != nil {
				decl.Parent = stack[len(stack)-1].decl
				decl.Qualified = decl.Parent.Qualified + "." + decl.Qualified
			}
			declarations = append(declarations, decl)
		}
		opening := luaOpeningBlocks(code)
		depth += opening
		if decl != nil && opening > closing {
			stack = append(stack, frame{decl: decl, depth: depth})
		}
		for len(stack) > 0 && depth < stack[len(stack)-1].depth {
			stack[len(stack)-1].decl.EndLine = index + 1
			stack = stack[:len(stack)-1]
		}
	}
	for _, item := range stack {
		item.decl.EndLine = len(lines)
	}
	for _, decl := range declarations {
		if decl.EndLine != 0 {
			continue
		}
		decl.EndLine = len(lines)
		for line := decl.StartLine; line < len(lines); line++ {
			if strings.TrimSpace(lines[line].Code) == "end" {
				decl.EndLine = line + 1
				break
			}
		}
	}
	return declarations
}

func luaFunctionDeclaration(qualified, header string, line, depth int) *luaDeclaration {
	name := qualified
	kind := "function"
	if index := strings.LastIndexAny(name, ".:"); index >= 0 {
		name = name[index+1:]
		kind = "method"
	}
	return &luaDeclaration{Kind: kind, Name: name, Qualified: qualified, Header: header, StartLine: line, Depth: depth}
}

func luaOpeningBlocks(code string) int {
	count := 0
	for _, word := range luaBlockWord.FindAllStringSubmatch(code, -1) {
		if word[1] == "function" || word[1] == "do" || word[1] == "repeat" || word[1] == "then" {
			count++
		}
	}
	return count
}

func luaClosingBlocks(code string) int {
	count := 0
	for _, word := range luaBlockWord.FindAllStringSubmatch(code, -1) {
		if word[1] == "end" || word[1] == "until" {
			count++
		}
	}
	return count
}
