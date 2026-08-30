package pawn

import (
	"regexp"
	"strings"
)

type pawnLine struct {
	Start, End int
	Raw, Code  string
}

type pawnDeclaration struct {
	Kind, Name, Qualified, Header string
	StartLine, EndLine, Depth     int
}

var (
	pawnEnumDecl     = regexp.MustCompile(`^\s*enum\s+([A-Za-z_]\w*)`)
	pawnDefineDecl   = regexp.MustCompile(`^\s*#define\s+([A-Za-z_]\w*)`)
	pawnConstDecl    = regexp.MustCompile(`^\s*const\s+(?:[A-Za-z_]\w*\s*:\s*)?([A-Za-z_]\w*)\s*=`)
	pawnGlobalDecl   = regexp.MustCompile(`^\s*(?:static\s+)?new\s+(?:[A-Za-z_]\w*\s*:\s*)?([A-Za-z_]\w*)`)
	pawnFunctionDecl = regexp.MustCompile(`^\s*(?:(public|stock|native|forward)\s+)?(?:(?:const|static)\s+)*(?:[A-Za-z_]\w*\s*:\s*)?([A-Za-z_]\w*)\s*\(`)
	pawnCall         = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*\(`)
	pawnHandler      = regexp.MustCompile(`\b(?:register_clcmd|register_concmd|register_event|register_logevent|set_task|menu_create)\b[^\n]*["']([A-Za-z_]\w*)["']`)
)

func pawnLines(source []byte, tokens []Token) []pawnLine {
	starts := []int{0}
	for index, value := range source {
		if value == '\n' && index+1 < len(source) {
			starts = append(starts, index+1)
		}
	}
	lines := make([]pawnLine, 0, len(starts))
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
			if token.EndByte <= start || token.StartByte >= end || token.Kind == Identifier || token.Kind == Keyword || token.Kind == Number || token.Kind == Punctuation || token.Kind == Directive {
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
		lines = append(lines, pawnLine{Start: start, End: end, Raw: raw, Code: string(code)})
	}
	return lines
}

func parsePawn(lines []pawnLine) []*pawnDeclaration {
	declarations := make([]*pawnDeclaration, 0)
	depth := 0
	stack := make([]*pawnDeclaration, 0)
	for index, line := range lines {
		code := strings.TrimSpace(line.Code)
		if code == "" || strings.HasPrefix(strings.TrimSpace(line.Raw), "//") {
			continue
		}
		var decl *pawnDeclaration
		if match := pawnEnumDecl.FindStringSubmatch(line.Raw); len(match) == 2 {
			decl = &pawnDeclaration{Kind: "enum", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		} else if match := pawnDefineDecl.FindStringSubmatch(line.Raw); len(match) == 2 {
			decl = &pawnDeclaration{Kind: "constant", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		} else if match := pawnConstDecl.FindStringSubmatch(code); len(match) == 2 {
			decl = &pawnDeclaration{Kind: "constant", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		} else if match := pawnGlobalDecl.FindStringSubmatch(code); len(match) == 2 {
			decl = &pawnDeclaration{Kind: "global", Name: match[1], Qualified: match[1], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		} else if match := pawnFunctionDecl.FindStringSubmatch(code); len(match) == 3 && !pawnControlWord(match[2]) {
			kind := "function"
			switch strings.ToLower(match[1]) {
			case "public":
				kind = "callback"
			case "stock", "native", "forward":
				kind = strings.ToLower(match[1])
			}
			decl = &pawnDeclaration{Kind: kind, Name: match[2], Qualified: match[2], Header: strings.TrimSpace(line.Raw), StartLine: index + 1, Depth: depth}
		}
		if decl != nil {
			declarations = append(declarations, decl)
		}
		before := depth
		depth += pawnBraceDelta(code)
		if depth < 0 {
			depth = 0
		}
		if decl != nil && depth > before {
			stack = append(stack, decl)
		}
		for len(stack) > 0 && depth < len(stack) {
			stack[len(stack)-1].EndLine = index + 1
			stack = stack[:len(stack)-1]
		}
	}
	for _, decl := range declarations {
		if decl.EndLine == 0 {
			decl.EndLine = decl.StartLine
			if decl.Kind == "function" || decl.Kind == "callback" || decl.Kind == "stock" {
				decl.EndLine = len(lines)
			}
		}
	}
	return declarations
}

func pawnControlWord(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "switch", "sizeof", "return":
		return true
	}
	return false
}

func pawnBraceDelta(code string) int {
	delta := 0
	for _, value := range code {
		switch value {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}
