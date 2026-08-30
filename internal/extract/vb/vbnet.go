package vb

import (
	"regexp"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

var (
	vbNamespacePattern = regexp.MustCompile(`(?i)^namespace\s+([a-z_][a-z0-9_.]*)`)
	vbTypePattern      = regexp.MustCompile(`(?i)\b(class|module|structure|interface|enum)\s+([a-z_][a-z0-9_]*)`)
	vbDelegatePattern  = regexp.MustCompile(`(?i)\bdelegate\s+(sub|function)\s+([a-z_][a-z0-9_]*)`)
	vbMethodPattern    = regexp.MustCompile(`(?i)\b(sub|function)\s+(new|[a-z_][a-z0-9_]*)`)
	vbPropertyPattern  = regexp.MustCompile(`(?i)\bproperty\s+([a-z_][a-z0-9_]*)`)
	vbEventPattern     = regexp.MustCompile(`(?i)\bevent\s+([a-z_][a-z0-9_]*)`)
	vbOperatorPattern  = regexp.MustCompile(`(?i)\boperator\s+([^\s(]+)`)
)

func parseVBNet(file model.SourceFile) vbParseResult {
	lines := vbLines(file.Content)
	result := vbParseResult{Lines: lines, Owner: vbOwner(file, "compilation_unit", "")}
	stack := make([]*vbDecl, 0, 8)
	for index, line := range lines {
		lineNumber := index + 1
		text := strings.TrimSpace(vbCode(line.Text))
		lower := strings.ToLower(text)
		if text == "" || strings.HasPrefix(text, "'") || strings.HasPrefix(text, "#") {
			continue
		}
		if len(stack) > 0 && vbNetCloses(stack[len(stack)-1].Kind, text) {
			stack[len(stack)-1].EndLine = lineNumber
			stack = stack[:len(stack)-1]
			continue
		}
		decl := parseVBNetDeclaration(text, lineNumber, stack)
		if decl == nil {
			continue
		}
		result.Decls = append(result.Decls, decl)
		if decl.Block {
			stack = append(stack, decl)
		}
		_ = lower
	}
	for _, decl := range stack {
		decl.EndLine = len(lines)
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "vbnet_unclosed_block", Message: "VB.NET block " + decl.Name + " is not closed; span recovered to end of file"})
	}
	for _, decl := range result.Decls {
		if decl.EndLine == 0 {
			decl.EndLine = decl.StartLine
		}
	}
	return result
}

func parseVBNetDeclaration(text string, line int, stack []*vbDecl) *vbDecl {
	parent := vbDeclParent(stack)
	if match := vbNamespacePattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("namespace", match[1], text, line, true, parent)
	}
	if match := vbDelegatePattern.FindStringSubmatch(text); len(match) == 3 {
		return newVBDecl("delegate", match[2], text, line, false, parent)
	}
	if match := vbTypePattern.FindStringSubmatch(text); len(match) == 3 && !strings.HasPrefix(strings.ToLower(text), "end ") {
		return newVBDecl(strings.ToLower(match[1]), match[2], text, line, true, parent)
	}
	if match := vbOperatorPattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("operator", match[1], text, line, true, parent)
	}
	if match := vbMethodPattern.FindStringSubmatch(text); len(match) == 3 {
		name := match[2]
		kind := strings.ToLower(match[1])
		if strings.EqualFold(name, "new") {
			kind = "constructor"
		}
		if strings.Contains(strings.ToLower(text), " handles ") || strings.Contains(name, "_") {
			kind = "event-handler"
		}
		block := true
		if parent != nil && parent.Kind == "interface" {
			block = false
		}
		if strings.Contains(strings.ToLower(text), "declare ") {
			kind = "declare"
			block = false
		}
		if strings.HasPrefix(strings.ToLower(text), "sub ") || strings.HasPrefix(strings.ToLower(text), "function ") {
			// Keep ordinary top-level members as structural declarations.
		}
		if strings.HasPrefix(strings.ToLower(name), "test") || parent != nil && strings.Contains(strings.ToLower(parent.Name), "test") {
			kind = "test"
		}
		return newVBDecl(kind, name, text, line, block, parent)
	}
	if match := vbPropertyPattern.FindStringSubmatch(text); len(match) == 2 {
		block := strings.Contains(strings.ToLower(text), " get") || strings.HasSuffix(strings.ToLower(text), " get")
		return newVBDecl("property", match[1], text, line, block, parent)
	}
	if match := vbEventPattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("event", match[1], text, line, false, parent)
	}
	return nil
}

func vbNetCloses(kind, text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if !strings.HasPrefix(text, "end ") {
		return false
	}
	ending := strings.TrimSpace(strings.TrimPrefix(text, "end "))
	switch kind {
	case "namespace":
		return ending == "namespace"
	case "class":
		return ending == "class"
	case "module":
		return ending == "module"
	case "structure":
		return ending == "structure"
	case "interface":
		return ending == "interface"
	case "enum":
		return ending == "enum"
	case "sub", "event-handler", "test":
		return ending == "sub"
	case "function", "constructor", "declare":
		return ending == "function"
	case "property":
		return ending == "property"
	case "operator":
		return ending == "operator"
	}
	return false
}

func newVBDecl(kind, name, header string, line int, block bool, parent *vbDecl) *vbDecl {
	qualified := name
	if parent != nil && parent.Qualified != "" {
		qualified = parent.Qualified + "." + name
	}
	return &vbDecl{Kind: kind, Name: name, Qualified: qualified, Header: header, StartLine: line, EndLine: line, Parent: parent, Block: block}
}

func vbDeclParent(stack []*vbDecl) *vbDecl {
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}
