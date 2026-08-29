package template

import (
	"context"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/model"
)

type smartyTag struct {
	Region      Region
	Name        string
	Closing     bool
	Attributes  map[string]string
	Positionals []string
}

type templateNode struct {
	Tag       smartyTag
	Close     *smartyTag
	Parent    int
	EndByte   int
	Recovered bool
}

type parseResult struct {
	Tags        []smartyTag
	Nodes       []templateNode
	Diagnostics []model.Diagnostic
}

func parse(ctx context.Context, filePath string, source []byte, regions []Region, scanDiagnostics []string) (parseResult, error) {
	result := parseResult{Tags: make([]smartyTag, 0), Nodes: make([]templateNode, 0), Diagnostics: make([]model.Diagnostic, 0, len(scanDiagnostics))}
	for _, code := range scanDiagnostics {
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: filePath, Level: "warning", Code: code, Message: "template scanner recovered from malformed input"})
	}
	stack := make([]struct {
		name string
		node int
	}, 0)
	for index, region := range regions {
		if index&127 == 0 {
			if err := ctx.Err(); err != nil {
				return result, err
			}
		}
		if region.Kind != KindSmartyTag {
			continue
		}
		tag, ok := parseTag(region)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: filePath, Level: "warning", Code: "template_malformed_tag", Message: "unable to parse Smarty tag"})
			continue
		}
		result.Tags = append(result.Tags, tag)
		if tag.Closing {
			matching := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].name == tag.Name {
					matching = i
					break
				}
			}
			if matching < 0 {
				result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: filePath, Level: "warning", Code: "template_mismatched_close", Message: "Smarty closing tag has no matching opener"})
				continue
			}
			for i := len(stack) - 1; i > matching; i-- {
				entry := stack[i]
				if entry.node >= 0 && result.Nodes[entry.node].Close == nil {
					result.Nodes[entry.node].EndByte = len(source)
					result.Nodes[entry.node].Recovered = true
				}
				result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: filePath, Level: "warning", Code: "template_mismatched_close", Message: "nested Smarty structure was recovered"})
			}
			entry := stack[matching]
			stack = stack[:matching]
			if entry.node >= 0 {
				result.Nodes[entry.node].Close = &result.Tags[len(result.Tags)-1]
				result.Nodes[entry.node].EndByte = tag.Region.EndByte
			}
			continue
		}
		if isNamedTag(tag.Name) {
			parent := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].node >= 0 {
					parent = stack[i].node
					break
				}
			}
			node := templateNode{Tag: tag, Parent: parent, EndByte: len(source)}
			result.Nodes = append(result.Nodes, node)
			stack = append(stack, struct {
				name string
				node int
			}{name: tag.Name, node: len(result.Nodes) - 1})
		} else if isControlTag(tag.Name) {
			stack = append(stack, struct {
				name string
				node int
			}{name: tag.Name, node: -1})
		}
	}
	for _, entry := range stack {
		if entry.node < 0 || result.Nodes[entry.node].Close != nil {
			continue
		}
		result.Nodes[entry.node].EndByte = len(source)
		result.Nodes[entry.node].Recovered = true
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: filePath, Level: "warning", Code: "template_unclosed_" + entry.name, Message: "Smarty structure has no closing tag"})
	}
	return result, nil
}

func parseTag(region Region) (smartyTag, bool) {
	raw := strings.TrimSpace(string(region.Content))
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return smartyTag{}, false
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" || strings.HasPrefix(inner, "$") {
		return smartyTag{}, false
	}
	closing := strings.HasPrefix(inner, "/")
	if closing {
		inner = strings.TrimSpace(inner[1:])
	}
	words := splitSmartyWords(inner)
	if len(words) == 0 {
		return smartyTag{}, false
	}
	name := strings.ToLower(words[0])
	attributes, positionals := parseSmartyAttributes(words[1:])
	return smartyTag{Region: region, Name: name, Closing: closing, Attributes: attributes, Positionals: positionals}, true
}

func splitSmartyWords(value string) []string {
	words := make([]string, 0)
	start := -1
	quote := rune(0)
	escaped := false
	for index, r := range value {
		if escaped {
			escaped = false
			if start < 0 {
				start = index
			}
			continue
		}
		if quote != 0 {
			if r == '\\' {
				escaped = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			if start < 0 {
				start = index
			}
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			if start >= 0 {
				words = append(words, value[start:index])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = index
		}
	}
	if start >= 0 {
		words = append(words, value[start:])
	}
	return words
}

func parseSmartyAttributes(words []string) (map[string]string, []string) {
	attributes := make(map[string]string)
	positionals := make([]string, 0)
	for index := 0; index < len(words); index++ {
		word := words[index]
		if word == "=" || strings.HasPrefix(word, "=") {
			continue
		}
		if equal := strings.IndexByte(word, '='); equal >= 0 {
			key := strings.ToLower(strings.TrimSpace(word[:equal]))
			if key != "" {
				attributes[key] = unquoteSmartyValue(word[equal+1:])
			}
			continue
		}
		if index+1 < len(words) && words[index+1] == "=" {
			if index+2 < len(words) {
				attributes[strings.ToLower(word)] = unquoteSmartyValue(words[index+2])
				index += 2
			}
			continue
		}
		positionals = append(positionals, unquoteSmartyValue(word))
	}
	return attributes, positionals
}

func unquoteSmartyValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
		value = value[1 : len(value)-1]
	}
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\'`, `'`)
	return value
}

func isNamedTag(name string) bool {
	switch name {
	case "block", "function", "capture":
		return true
	default:
		return false
	}
}

func isControlTag(name string) bool {
	switch name {
	case "if", "foreach", "for", "while", "section":
		return true
	default:
		return false
	}
}
