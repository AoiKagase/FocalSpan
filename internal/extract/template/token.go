package template

import "bytes"

// Kind identifies a bounded lexical region in a composite template.
type Kind string

const (
	KindStatic        Kind = "static"
	KindHTMLComment   Kind = "html-comment"
	KindTemplateTag   Kind = "template-tag"
	KindSmartyTag     Kind = "smarty-tag"
	KindSmartyVar     Kind = "smarty-variable"
	KindSmartyComment Kind = "smarty-comment"
	KindSmartyLiteral Kind = "smarty-literal"
	KindPHPBlock      Kind = "php-block"
	KindScriptOpen    Kind = "script-open"
	KindScriptBody    Kind = "script-body"
	KindScriptClose   Kind = "script-close"
	KindDataScript    Kind = "data-script"
	KindStyleOpen     Kind = "style-open"
	KindStyleBody     Kind = "style-body"
	KindStyleClose    Kind = "style-close"
)

// Region is always expressed in offsets into the original source bytes.
type Region struct {
	Kind      Kind
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
	Content   []byte
}

func makeRegion(source []byte, kind Kind, start, end int) Region {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	startLine, endLine := lineRange(source, start, end)
	return Region{Kind: kind, StartByte: start, EndByte: end, StartLine: startLine, EndLine: endLine, Content: source[start:end]}
}

func lineRange(source []byte, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if start > len(source) {
		start = len(source)
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	startLine := 1 + bytes.Count(source[:start], []byte{'\n'})
	endLine := startLine
	if end > start {
		endLine = 1 + bytes.Count(source[:end], []byte{'\n'})
		if source[end-1] == '\n' && endLine > startLine {
			endLine--
		}
	}
	return startLine, endLine
}

func lineCount(source []byte) int {
	if len(source) == 0 {
		return 1
	}
	count := 1 + bytes.Count(source, []byte{'\n'})
	if source[len(source)-1] == '\n' && count > 1 {
		count--
	}
	return count
}
