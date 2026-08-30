package template

import (
	"bytes"
	"context"
	"strings"
)

// Scan returns lexical regions without interpreting Smarty semantics.
func Scan(ctx context.Context, source []byte) ([]Region, error) {
	regions, _, err := scan(ctx, source)
	return regions, err
}

func scan(ctx context.Context, source []byte) ([]Region, []string, error) {
	regions := make([]Region, 0)
	diagnostics := make([]string, 0)
	staticStart := 0
	flushStatic := func(end int) {
		if end > staticStart {
			regions = append(regions, makeRegion(source, KindStatic, staticStart, end))
		}
	}
	check := func(offset int) error {
		if offset&1023 == 0 {
			return ctx.Err()
		}
		return nil
	}

	for i := 0; i < len(source); {
		if err := check(i); err != nil {
			return regions, diagnostics, err
		}
		switch {
		case bytesHasPrefixFold(source, i, "<!--"):
			flushStatic(i)
			end := bytesIndexFold(source, i+4, "-->")
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_unclosed_html_comment")
			} else {
				end += 3
			}
			regions = append(regions, makeRegion(source, KindHTMLComment, i, end))
			i, staticStart = end, end
		case bytesHasPrefix(source, i, "{*"):
			flushStatic(i)
			end := bytes.Index(source[i+2:], []byte("*}"))
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_unclosed_comment")
			} else {
				end += i + 2
				end += 2
			}
			regions = append(regions, makeRegion(source, KindSmartyComment, i, end))
			i, staticStart = end, end
		case isLiteralOpen(source, i):
			flushStatic(i)
			closeTag := "{/literal}"
			if bytesHasPrefixFold(source, i, "{verbatim") {
				closeTag = "{/verbatim}"
			}
			end := bytesIndexFold(source, i+1, closeTag)
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_unclosed_literal")
			} else {
				end += len(closeTag)
			}
			regions = append(regions, makeRegion(source, KindSmartyLiteral, i, end))
			i, staticStart = end, end
		case isPHPOpen(source, i):
			flushStatic(i)
			end := bytes.Index(source[i+2:], []byte("?>"))
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_unclosed_php")
			} else {
				end += i + 2
				end += 2
			}
			regions = append(regions, makeRegion(source, KindPHPBlock, i, end))
			i, staticStart = end, end
		case bytesHasPrefix(source, i, "{{"):
			flushStatic(i)
			end := findTemplateTagEnd(source, i)
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_unclosed_template_tag")
			} else {
				end += 2
			}
			regions = append(regions, makeRegion(source, KindTemplateTag, i, end))
			i, staticStart = end, end
		case isHTMLTag(source, i, "script"):
			flushStatic(i)
			openEnd := findTagEnd(source, i)
			if openEnd < 0 {
				openEnd = len(source)
				diagnostics = append(diagnostics, "template_unterminated_tag")
				regions = append(regions, makeRegion(source, KindScriptOpen, i, openEnd))
				i, staticStart = openEnd, openEnd
				continue
			}
			openEnd++
			regions = append(regions, makeRegion(source, KindScriptOpen, i, openEnd))
			closeStart := findScriptClose(source, openEnd)
			bodyEnd := closeStart
			if closeStart < 0 {
				bodyEnd = len(source)
				diagnostics = append(diagnostics, "template_unclosed_script")
			}
			bodyKind := KindScriptBody
			if isDataScript(source[i:openEnd]) {
				bodyKind = KindDataScript
			}
			if bodyEnd > openEnd {
				regions = append(regions, makeRegion(source, bodyKind, openEnd, bodyEnd))
			}
			if closeStart >= 0 {
				closeEnd := findTagEnd(source, closeStart)
				if closeEnd < 0 {
					closeEnd = len(source)
				} else {
					closeEnd++
				}
				regions = append(regions, makeRegion(source, KindScriptClose, closeStart, closeEnd))
				i, staticStart = closeEnd, closeEnd
			} else {
				i, staticStart = len(source), len(source)
			}
		case isHTMLTag(source, i, "style"):
			flushStatic(i)
			openEnd := findTagEnd(source, i)
			if openEnd < 0 {
				openEnd = len(source)
				diagnostics = append(diagnostics, "template_unterminated_tag")
				regions = append(regions, makeRegion(source, KindStyleOpen, i, openEnd))
				i, staticStart = openEnd, openEnd
				continue
			}
			openEnd++
			regions = append(regions, makeRegion(source, KindStyleOpen, i, openEnd))
			closeStart := bytesIndexFold(source, openEnd, "</style")
			bodyEnd := closeStart
			if closeStart < 0 {
				bodyEnd = len(source)
				diagnostics = append(diagnostics, "template_unclosed_style")
			}
			if bodyEnd > openEnd {
				regions = append(regions, makeRegion(source, KindStyleBody, openEnd, bodyEnd))
			}
			if closeStart >= 0 {
				closeEnd := findTagEnd(source, closeStart)
				if closeEnd < 0 {
					closeEnd = len(source)
				} else {
					closeEnd++
				}
				regions = append(regions, makeRegion(source, KindStyleClose, closeStart, closeEnd))
				i, staticStart = closeEnd, closeEnd
			} else {
				i, staticStart = len(source), len(source)
			}
		case source[i] == '{' && isSmartyTagStart(source, i):
			flushStatic(i)
			end := findSmartyEnd(source, i)
			if end < 0 {
				end = len(source)
				diagnostics = append(diagnostics, "template_malformed_tag")
			} else {
				end++
			}
			kind := KindSmartyTag
			if i+1 < end && source[i+1] == '$' {
				kind = KindSmartyVar
			}
			regions = append(regions, makeRegion(source, kind, i, end))
			i, staticStart = end, end
		default:
			i++
		}
	}
	flushStatic(len(source))
	return regions, diagnostics, nil
}

func isLiteralOpen(source []byte, at int) bool {
	for _, name := range []string{"literal", "verbatim"} {
		if !bytesHasPrefixFold(source, at, "{"+name) {
			continue
		}
		end := at + 1 + len(name)
		return end < len(source) && (source[end] == '}' || source[end] == ' ' || source[end] == '\t' || source[end] == '\r' || source[end] == '\n')
	}
	return false
}

func isPHPOpen(source []byte, at int) bool {
	if !bytesHasPrefix(source, at, "<?") {
		return false
	}
	if bytesHasPrefixFold(source, at, "<?xml") {
		return false
	}
	if bytesHasPrefixFold(source, at, "<?php") && at+5 < len(source) && isIdentifierByte(source[at+5]) {
		return false
	}
	if at+2 >= len(source) {
		return true
	}
	if bytesHasPrefixFold(source, at, "<?php") || source[at+2] == '=' {
		return true
	}
	// Short PHP tags are accepted by the existing content detector too. XML
	// declarations were excluded above, so a remaining "<?" is PHP-like.
	return true
}

func isIdentifierByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func isSmartyTagStart(source []byte, at int) bool {
	if at+1 >= len(source) {
		return false
	}
	next := source[at+1]
	if next == '$' || next == '/' {
		return true
	}
	return (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '_'
}

func findSmartyEnd(source []byte, at int) int {
	quote := byte(0)
	escaped := false
	for i := at + 1; i < len(source); i++ {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if source[i] == '\\' {
				escaped = true
			} else if source[i] == quote {
				quote = 0
			}
			continue
		}
		if source[i] == '\'' || source[i] == '"' {
			quote = source[i]
			continue
		}
		if source[i] == '}' {
			return i
		}
	}
	return -1
}

func findTemplateTagEnd(source []byte, at int) int {
	quote := byte(0)
	escaped := false
	for i := at + 2; i+1 < len(source); i++ {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if source[i] == '\\' {
				escaped = true
			} else if source[i] == quote {
				quote = 0
			}
			continue
		}
		if source[i] == '\'' || source[i] == '"' {
			quote = source[i]
			continue
		}
		if source[i] == '}' && source[i+1] == '}' {
			return i
		}
	}
	return -1
}

func isHTMLTag(source []byte, at int, name string) bool {
	if at >= len(source) || source[at] != '<' || !bytesHasPrefixFold(source, at+1, name) {
		return false
	}
	end := at + 1 + len(name)
	return end < len(source) && (source[end] == '>' || source[end] == '/' || source[end] == ' ' || source[end] == '\t' || source[end] == '\r' || source[end] == '\n')
}

func findTagEnd(source []byte, at int) int {
	quote := byte(0)
	for i := at; i < len(source); i++ {
		if quote != 0 {
			if source[i] == quote {
				quote = 0
			}
			continue
		}
		if source[i] == '\'' || source[i] == '"' {
			quote = source[i]
		} else if source[i] == '>' {
			return i
		}
	}
	return -1
}

func findScriptClose(source []byte, at int) int {
	quote := byte(0)
	escaped := false
	lineComment := false
	blockComment := false
	for i := at; i < len(source); i++ {
		c := source[i]
		if lineComment {
			if c == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < len(source) && source[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '/' {
			lineComment = true
			i++
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '*' {
			blockComment = true
			i++
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			continue
		}
		if c == '<' && bytesHasPrefixFold(source, i, "</script") && tagBoundary(source, i+len("</script")) {
			return i
		}
	}
	return -1
}

func tagBoundary(source []byte, at int) bool {
	return at >= len(source) || source[at] == '>' || source[at] == ' ' || source[at] == '\t' || source[at] == '\r' || source[at] == '\n'
}

func isDataScript(open []byte) bool {
	attrs := htmlAttributes(open)
	typeValue := strings.ToLower(attrs["type"])
	return typeValue == "application/ld+json" || typeValue == "application/json" || typeValue == "importmap"
}

func htmlAttributes(tag []byte) map[string]string {
	result := make(map[string]string)
	if len(tag) == 0 {
		return result
	}
	for i := 1; i < len(tag); {
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\r' || tag[i] == '\n' || tag[i] == '/' || tag[i] == '>') {
			i++
		}
		start := i
		for i < len(tag) && isAttrByte(tag[i]) {
			i++
		}
		if start == i {
			i++
			continue
		}
		name := strings.ToLower(string(tag[start:i]))
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\r' || tag[i] == '\n') {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			result[name] = ""
			continue
		}
		i++
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\r' || tag[i] == '\n') {
			i++
		}
		start = i
		if i < len(tag) && (tag[i] == '\'' || tag[i] == '"') {
			quote := tag[i]
			i++
			start = i
			for i < len(tag) && tag[i] != quote {
				i++
			}
			result[name] = string(tag[start:i])
			if i < len(tag) {
				i++
			}
		} else {
			for i < len(tag) && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '\r' && tag[i] != '\n' && tag[i] != '>' {
				i++
			}
			result[name] = string(tag[start:i])
		}
	}
	return result
}

func isAttrByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == ':' || value == '-' || value == '_'
}

func bytesHasPrefix(source []byte, at int, value string) bool {
	return at >= 0 && at+len(value) <= len(source) && string(source[at:at+len(value)]) == value
}

func bytesHasPrefixFold(source []byte, at int, value string) bool {
	return at >= 0 && at+len(value) <= len(source) && strings.EqualFold(string(source[at:at+len(value)]), value)
}

func bytesIndexFold(source []byte, at int, value string) int {
	if at < 0 {
		at = 0
	}
	for i := at; i+len(value) <= len(source); i++ {
		if bytesHasPrefixFold(source, i, value) {
			return i
		}
	}
	return -1
}
