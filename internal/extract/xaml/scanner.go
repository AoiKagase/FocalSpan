package xaml

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type attribute struct {
	Name      string
	Value     string
	ValueFrom int
	ValueTo   int
}

type element struct {
	Name       string
	StartByte  int
	EndByte    int
	SelfClosed bool
	Attributes []attribute
}

type scanResult struct {
	Elements    []element
	Diagnostics []model.Diagnostic
}

func scan(ctx context.Context, content []byte) (scanResult, error) {
	result := scanResult{}
	stack := make([]int, 0, 8)
	for index := 0; index < len(content); {
		if err := ctx.Err(); err != nil {
			return scanResult{}, err
		}
		if content[index] != '<' {
			index++
			continue
		}
		if strings.HasPrefix(string(content[index:]), "<!--") {
			end := strings.Index(string(content[index+4:]), "-->")
			if end < 0 {
				result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "xaml_unclosed_comment", Message: "XAML comment is not closed"})
				break
			}
			index += 4 + end + 3
			continue
		}
		if strings.HasPrefix(string(content[index:]), "<![CDATA[") {
			end := strings.Index(string(content[index+9:]), "]]>")
			if end < 0 {
				result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "xaml_unclosed_cdata", Message: "XAML CDATA is not closed"})
				break
			}
			index += 9 + end + 3
			continue
		}
		if index+1 < len(content) && content[index+1] == '?' {
			index = advanceToTagEnd(content, index+2)
			continue
		}
		if index+1 < len(content) && content[index+1] == '!' {
			index = advanceToTagEnd(content, index+2)
			continue
		}
		if index+1 < len(content) && content[index+1] == '/' {
			end := advanceToTagEnd(content, index+2)
			name := strings.TrimSpace(string(content[index+2 : minInt(end, len(content))]))
			name = strings.TrimSuffix(name, ">")
			name = strings.TrimSpace(name)
			if close := strings.IndexAny(name, " \t\r\n"); close >= 0 {
				name = name[:close]
			}
			for position := len(stack) - 1; position >= 0; position-- {
				if strings.EqualFold(result.Elements[stack[position]].Name, name) {
					result.Elements[stack[position]].EndByte = minInt(end, len(content))
					stack = stack[:position]
					break
				}
			}
			index = end
			continue
		}
		end := advanceToTagEnd(content, index+1)
		parsed, ok := parseOpeningTag(content, index, end)
		if !ok {
			index = maxInt(index+1, end)
			continue
		}
		result.Elements = append(result.Elements, parsed)
		current := len(result.Elements) - 1
		if !parsed.SelfClosed {
			stack = append(stack, current)
		}
		if end >= len(content) && (len(content) == 0 || content[len(content)-1] != '>') {
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "xaml_unclosed_tag", Message: "XAML tag is not closed; recovered through end of file"})
			break
		}
		index = end
	}
	for _, index := range stack {
		if index >= 0 && index < len(result.Elements) && result.Elements[index].EndByte <= result.Elements[index].StartByte {
			result.Elements[index].EndByte = len(content)
		}
	}
	return result, nil
}

func parseOpeningTag(content []byte, start, end int) (element, bool) {
	if start < 0 || start >= end || content[start] != '<' {
		return element{}, false
	}
	position := start + 1
	for position < end && isTagSpace(content[position]) {
		position++
	}
	nameStart := position
	for position < end && isTagNameByte(content[position]) {
		position++
	}
	if position == nameStart {
		return element{}, false
	}
	parsed := element{Name: string(content[nameStart:position]), StartByte: start, EndByte: end}
	trimEnd := end
	if trimEnd > start && content[trimEnd-1] == '>' {
		trimEnd--
	}
	for trimEnd > position && isTagSpace(content[trimEnd-1]) {
		trimEnd--
	}
	if trimEnd > position && content[trimEnd-1] == '/' {
		parsed.SelfClosed = true
		trimEnd--
	}
	for position < trimEnd {
		for position < trimEnd && isTagSpace(content[position]) {
			position++
		}
		if position >= trimEnd {
			break
		}
		attrStart := position
		for position < trimEnd && !isTagSpace(content[position]) && content[position] != '=' {
			position++
		}
		if position == attrStart {
			position++
			continue
		}
		attr := attribute{Name: string(content[attrStart:position])}
		for position < trimEnd && isTagSpace(content[position]) {
			position++
		}
		if position < trimEnd && content[position] == '=' {
			position++
			for position < trimEnd && isTagSpace(content[position]) {
				position++
			}
			if position < trimEnd && (content[position] == '"' || content[position] == '\'') {
				quote := content[position]
				position++
				attr.ValueFrom = position
				for position < trimEnd && content[position] != quote {
					position++
				}
				attr.ValueTo = position
				attr.Value = string(content[attr.ValueFrom:attr.ValueTo])
				if position < trimEnd {
					position++
				}
			} else {
				attr.ValueFrom = position
				for position < trimEnd && !isTagSpace(content[position]) {
					position++
				}
				attr.ValueTo = position
				attr.Value = string(content[attr.ValueFrom:attr.ValueTo])
			}
		}
		parsed.Attributes = append(parsed.Attributes, attr)
	}
	return parsed, true
}

func advanceToTagEnd(content []byte, start int) int {
	quote := byte(0)
	for index := start; index < len(content); index++ {
		if quote != 0 {
			if content[index] == quote {
				quote = 0
			}
			continue
		}
		if content[index] == '"' || content[index] == '\'' {
			quote = content[index]
			continue
		}
		if content[index] == '>' {
			return index + 1
		}
	}
	return len(content)
}

func isTagSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isTagNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune(":._-", rune(value))
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
