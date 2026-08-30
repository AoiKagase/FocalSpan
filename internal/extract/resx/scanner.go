package resx

import (
	"context"
	"regexp"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type attribute struct {
	Name      string
	Value     string
	ValueFrom int
	ValueTo   int
}

type item struct {
	Kind       string
	Name       string
	StartByte  int
	EndByte    int
	Attributes []attribute
}

type scanResult struct {
	Items       []item
	Imports     []string
	Diagnostics []model.Diagnostic
}

var tagStartPattern = regexp.MustCompile(`(?is)<(data|metadata|setting)\b`)
var fileRefPattern = regexp.MustCompile(`(?is)<ResXFileRef\b[^>]*>\s*([^<]+?)\s*</ResXFileRef\s*>`)

func scan(ctx context.Context, content []byte) (scanResult, error) {
	result := scanResult{}
	for _, match := range tagStartPattern.FindAllIndex(content, -1) {
		if err := ctx.Err(); err != nil {
			return scanResult{}, err
		}
		start := match[0]
		end := advanceToTagEnd(content, start+1)
		nameStart := match[0] + 1
		nameEnd := nameStart
		for nameEnd < end && content[nameEnd] != ' ' && content[nameEnd] != '\t' && content[nameEnd] != '\r' && content[nameEnd] != '\n' && content[nameEnd] != '>' {
			nameEnd++
		}
		name := strings.ToLower(string(content[nameStart:nameEnd]))
		kind := "resx_resource"
		if name == "metadata" {
			kind = "resx_metadata"
		} else if name == "setting" {
			kind = "setting"
		}
		attrs := parseAttributes(content, nameEnd, end)
		result.Items = append(result.Items, item{Kind: kind, Name: attributeValue(attrs, "name"), StartByte: start, EndByte: end, Attributes: attrs})
	}
	for _, match := range fileRefPattern.FindAllSubmatchIndex(content, -1) {
		if len(match) < 4 {
			continue
		}
		value := strings.TrimSpace(string(content[match[2]:match[3]]))
		if semicolon := strings.IndexByte(value, ';'); semicolon >= 0 {
			value = value[:semicolon]
		}
		value = strings.ReplaceAll(value, `\\`, `\`)
		if value != "" {
			result.Imports = append(result.Imports, value)
		}
	}
	return result, nil
}

func parseAttributes(content []byte, start, end int) []attribute {
	result := make([]attribute, 0, 4)
	for position := start; position < end; {
		for position < end && isSpace(content[position]) {
			position++
		}
		if position >= end || content[position] == '/' || content[position] == '>' {
			break
		}
		nameStart := position
		for position < end && !isSpace(content[position]) && content[position] != '=' && content[position] != '>' {
			position++
		}
		if nameStart == position {
			position++
			continue
		}
		attr := attribute{Name: string(content[nameStart:position])}
		for position < end && isSpace(content[position]) {
			position++
		}
		if position < end && content[position] == '=' {
			position++
			for position < end && isSpace(content[position]) {
				position++
			}
			if position < end && (content[position] == '"' || content[position] == '\'') {
				quote := content[position]
				position++
				attr.ValueFrom = position
				for position < end && content[position] != quote {
					position++
				}
				attr.ValueTo = position
				attr.Value = string(content[attr.ValueFrom:attr.ValueTo])
				if position < end {
					position++
				}
			}
		}
		result = append(result, attr)
	}
	return result
}

func attributeValue(attrs []attribute, wanted string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, wanted) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func advanceToTagEnd(content []byte, start int) int {
	quote := byte(0)
	for position := start; position < len(content); position++ {
		if quote != 0 {
			if content[position] == quote {
				quote = 0
			}
			continue
		}
		if content[position] == '"' || content[position] == '\'' {
			quote = content[position]
		} else if content[position] == '>' {
			return position + 1
		}
	}
	return len(content)
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}
