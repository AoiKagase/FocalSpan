// Package phpscope resolves lexical PHP calls without fuzzy short-name lookup.
package phpscope

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
)

// Candidates returns ordered PHP names; the extractor expands import aliases.
func Candidates(from model.Symbol, target string) []string {
	if target == "" || strings.Contains(target, "->") || strings.ContainsAny(target, "$()[]{}<>?") {
		return nil
	}
	owner := from.QualifiedName
	if at := strings.Index(owner, "::"); at >= 0 {
		owner = owner[:at]
	}
	namespace := ""
	if at := strings.LastIndex(owner, "\\"); at >= 0 {
		namespace = owner[:at]
	}
	if strings.HasPrefix(target, "\\") {
		return []string{strings.TrimPrefix(target, "\\")}
	}
	if strings.HasPrefix(strings.ToLower(target), "namespace\\") {
		target = target[len("namespace\\"):]
		if namespace != "" {
			target = namespace + "\\" + target
		}
		return []string{target}
	}
	primary := target
	if namespace != "" {
		primary = namespace + "\\" + target
	}
	if namespace != "" && !strings.Contains(target, "\\") && !strings.Contains(target, "::") {
		return []string{primary, target}
	}
	return []string{primary}
}

// Resolve retains ambiguity at the first populated lookup level.
func Resolve(from model.Symbol, target string, symbols []model.Symbol) []model.Symbol {
	for _, name := range Candidates(from, target) {
		var matches []model.Symbol
		method := strings.Contains(name, "::")
		for _, symbol := range symbols {
			if symbol.Language != "php" {
				continue
			}
			if method {
				if symbol.Kind != "method" && symbol.Kind != "test" {
					continue
				}
			} else if symbol.Kind != "function" {
				continue
			}
			if strings.EqualFold(symbol.QualifiedName, name) {
				matches = append(matches, symbol)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}
	return nil
}

// ReferenceKind identifies the extractor's typed lexical relations. Other
// languages and legacy path references continue through generic linking.
func ReferenceKind(source string) string {
	if strings.HasPrefix(source, "php-import-") {
		return strings.TrimPrefix(source, "php-import-")
	}
	switch source {
	case "new", "class-reference", "type", "property-type", "signature-type", "inheritance", "trait-use", "phpdoc-type":
		return "type"
	}
	return ""
}

func ResolveReference(target, kind string, symbols []model.Symbol) []model.Symbol {
	target = strings.TrimPrefix(target, "\\")
	var matches []model.Symbol
	for _, symbol := range symbols {
		if symbol.Language != "php" {
			continue
		}
		class := symbol.Kind == "class" || symbol.Kind == "interface" || symbol.Kind == "trait" || symbol.Kind == "enum"
		allowed := kind == "type" && class || kind == "class" && (class || symbol.Kind == "namespace") || kind == "function" && symbol.Kind == "function" || kind == "const" && symbol.Kind == "constant"
		equal := strings.EqualFold(symbol.QualifiedName, target)
		if kind == "const" {
			equal = symbol.QualifiedName == target
		}
		if allowed && equal {
			matches = append(matches, symbol)
		}
	}
	return matches
}
