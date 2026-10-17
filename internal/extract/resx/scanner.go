package resx

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
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

// scan uses XML token boundaries while retaining original attribute byte spans.
// Decoder never loads external entities; malformed XML stops extraction at the
// error rather than turning text or partial attributes into resource markup.
func scan(ctx context.Context, content []byte) (scanResult, error) {
	result := scanResult{}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	type frame struct {
		name    string
		fileRef bool
		value   string
	}
	stack := make([]frame, 0)
	for {
		if err := ctx.Err(); err != nil {
			return scanResult{}, err
		}
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Level: "warning", Code: "resx_invalid_xml", Message: "resource XML is malformed"})
			break
		}
		switch current := token.(type) {
		case xml.StartElement:
			end := int(decoder.InputOffset())
			name := strings.ToLower(current.Name.Local)
			next := frame{name: name}
			if name == "data" || name == "metadata" || name == "setting" {
				// InputOffset is the end of this actual start tag. Parse only its raw
				// attributes so entity decoding never changes source coordinates.
				nameEnd := start + 1
				for nameEnd < end && !isSpace(content[nameEnd]) && content[nameEnd] != '/' && content[nameEnd] != '>' {
					nameEnd++
				}
				attrs := parseAttributes(content, nameEnd, end)
				for _, decoded := range current.Attr {
					for index := range attrs {
						local := attrs[index].Name
						if colon := strings.IndexByte(local, ':'); colon >= 0 {
							local = local[colon+1:]
						}
						if local == decoded.Name.Local {
							attrs[index].Value = decoded.Value
						}
					}
				}
				kind := "resx_resource"
				if name == "metadata" {
					kind = "resx_metadata"
				} else if name == "setting" {
					kind = "setting"
				}
				result.Items = append(result.Items, item{Kind: kind, Name: attributeValue(attrs, "name"), StartByte: start, EndByte: end, Attributes: attrs})
				next.fileRef = name == "data" && strings.TrimSpace(strings.Split(attributeValue(attrs, "type"), ",")[0]) == "System.Resources.ResXFileRef"
			}
			stack = append(stack, next)
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			last := len(stack) - 1
			if stack[last].name == "resxfileref" {
				stack[last].value += string(current)
			}
			if last > 0 && stack[last].name == "value" && stack[last-1].fileRef {
				stack[last-1].value += string(current)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if last.fileRef || last.name == "resxfileref" {
				target := strings.TrimSpace(strings.SplitN(last.value, ";", 2)[0])
				target = strings.ReplaceAll(target, `\\`, `\`)
				if target != "" {
					result.Imports = append(result.Imports, target)
				}
			}
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
