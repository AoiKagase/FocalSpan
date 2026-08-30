package vb

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

var (
	vb6BeginPattern    = regexp.MustCompile(`(?i)^begin\s+vb\.([a-z][a-z0-9_]*)\s+([a-z_][a-z0-9_]*)`)
	vb6MethodPattern   = regexp.MustCompile(`(?i)\b(sub|function)\s+([a-z_][a-z0-9_]*)`)
	vb6PropertyPattern = regexp.MustCompile(`(?i)\bproperty\s+(get|let|set)\s+([a-z_][a-z0-9_]*)`)
	vb6DeclarePattern  = regexp.MustCompile(`(?i)\bdeclare\s+(function|sub)\s+([a-z_][a-z0-9_]*)`)
	vb6EventPattern    = regexp.MustCompile(`(?i)\bevent\s+([a-z_][a-z0-9_]*)`)
	vb6TypePattern     = regexp.MustCompile(`(?i)^(?:(?:public|private|friend|static)\s+)*type\s+([a-z_][a-z0-9_]*)`)
	vb6EnumPattern     = regexp.MustCompile(`(?i)^(?:(?:public|private|friend)\s+)*enum\s+([a-z_][a-z0-9_]*)`)
	vb6ConstPattern    = regexp.MustCompile(`(?i)^(?:(?:public|private|friend|global|public)\s+)*const\s+([a-z_][a-z0-9_]*)`)
)

func parseVB6(file model.SourceFile) vbParseResult {
	lines := vbLines(file.Content)
	result := vbParseResult{Lines: lines}
	ownerKind := "module"
	switch strings.ToLower(filepath.Ext(file.Path)) {
	case ".frm":
		ownerKind = "form"
	case ".cls":
		ownerKind = "class"
	case ".ctl":
		ownerKind = "usercontrol"
	}
	owner := vbOwner(file, ownerKind, vbNameFromAttribute(lines))
	for index, line := range lines {
		text := strings.TrimSpace(vbCode(line.Text))
		match := vb6BeginPattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		owner.Kind = strings.ToLower(match[1])
		owner.Name = match[2]
		owner.Qualified = owner.Name
		owner.Header = strings.TrimSpace(line.Text)
		if owner.Kind == "mdiform" {
			owner.Kind = "form"
		}
		for end := index + 1; end < len(lines); end++ {
			if strings.EqualFold(strings.TrimSpace(vbCode(lines[end].Text)), "end") {
				result.LayoutEnd = lines[end].End
				break
			}
		}
		break
	}
	result.Owner = owner
	stack := make([]*vbDecl, 0, 8)
	for index, line := range lines {
		lineNumber := index + 1
		text := strings.TrimSpace(vbCode(line.Text))
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(strings.ToLower(text), "attribute ") || strings.HasPrefix(strings.ToLower(text), "begin vb.") || strings.EqualFold(text, "end") {
			continue
		}
		if len(stack) > 0 && vb6Closes(stack[len(stack)-1].Kind, text) {
			stack[len(stack)-1].EndLine = lineNumber
			stack = stack[:len(stack)-1]
			continue
		}
		decl := parseVB6Declaration(text, lineNumber, stack)
		if decl == nil {
			continue
		}
		result.Decls = append(result.Decls, decl)
		if decl.Block {
			stack = append(stack, decl)
		}
	}
	for _, decl := range stack {
		decl.EndLine = len(lines)
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "vb6_unclosed_block", Message: "VB6 block " + decl.Name + " is not closed; span recovered to end of file"})
	}
	for _, decl := range result.Decls {
		if decl.EndLine == 0 {
			decl.EndLine = decl.StartLine
		}
	}
	return result
}

func parseVB6Declaration(text string, line int, stack []*vbDecl) *vbDecl {
	parent := vbDeclParent(stack)
	if match := vb6DeclarePattern.FindStringSubmatch(text); len(match) == 3 {
		return newVBDecl("declare", match[2], text, line, false, parent)
	}
	if match := vb6PropertyPattern.FindStringSubmatch(text); len(match) == 3 {
		return newVBDecl("property-"+strings.ToLower(match[1]), match[2], text, line, true, parent)
	}
	if match := vb6MethodPattern.FindStringSubmatch(text); len(match) == 3 {
		kind := strings.ToLower(match[1])
		if strings.Contains(match[2], "_") {
			kind = "event-handler"
		}
		return newVBDecl(kind, match[2], text, line, true, parent)
	}
	if match := vb6EventPattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("event", match[1], text, line, false, parent)
	}
	if match := vb6TypePattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("type", match[1], text, line, true, parent)
	}
	if match := vb6EnumPattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("enum", match[1], text, line, true, parent)
	}
	if match := vb6ConstPattern.FindStringSubmatch(text); len(match) == 2 {
		return newVBDecl("constant", match[1], text, line, false, parent)
	}
	return nil
}

func vb6Closes(kind, text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.HasPrefix(text, "end sub"):
		return kind == "sub" || kind == "event-handler"
	case strings.HasPrefix(text, "end function"):
		return kind == "function" || kind == "event-handler"
	case strings.HasPrefix(text, "end property"):
		return strings.HasPrefix(kind, "property-")
	case strings.HasPrefix(text, "end type"):
		return kind == "type"
	case strings.HasPrefix(text, "end enum"):
		return kind == "enum"
	}
	return false
}
