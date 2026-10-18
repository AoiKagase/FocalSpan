// Package cppscope implements conservative lexical C++ callable lookup.
package cppscope

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
)

func LookupLevels(namespace, target string, absolute bool) []string {
	target = strings.TrimPrefix(target, "::")
	if absolute {
		return []string{target}
	}
	var out []string
	for {
		if namespace == "" {
			out = append(out, target)
			break
		}
		out = append(out, namespace+"::"+target)
		if p := strings.LastIndex(namespace, "::"); p >= 0 {
			namespace = namespace[:p]
		} else {
			namespace = ""
		}
	}
	return out
}
func Resolve(from model.Symbol, target string, symbols []model.Symbol) []model.Symbol {
	if strings.Contains(target, "->") || strings.Contains(target, ".") {
		return nil
	}
	scope := ""
	if i := strings.LastIndex(from.QualifiedName, "::"); i >= 0 {
		scope = from.QualifiedName[:i]
	}
	for _, qualified := range LookupLevels(scope, target, strings.HasPrefix(target, "::")) {
		var found []model.Symbol
		hidden := false
		for _, s := range symbols {
			if s.Language != "cpp" && s.Language != "c" {
				continue
			}
			if s.QualifiedName != qualified {
				continue
			}
			if strings.Contains(s.QualifiedName, "(anonymous@") && s.FilePath != from.FilePath {
				continue
			}
			switch s.Kind {
			case "function", "method", "operator", "constructor", "destructor", "test":
				found = append(found, s)
			case "namespace", "translation_unit", "module":
			default:
				hidden = true
			}
		}
		if len(found) > 0 {
			return found
		}
		if hidden {
			return nil
		}
	}
	return nil
}
func Matches(from model.Symbol, target string, anchor model.Symbol, symbols []model.Symbol) bool {
	for _, s := range Resolve(from, target, symbols) {
		if s.Handle == anchor.Handle {
			return true
		}
	}
	return false
}
