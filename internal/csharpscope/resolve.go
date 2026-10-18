// Package csharpscope resolves only lexically known C# call targets.
package csharpscope

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
)

type Resolver struct {
	byHandle    map[string]model.Symbol
	byQualified map[string][]model.Symbol
	children    map[string]map[string][]model.Symbol
	types       map[string][]model.Symbol
}

func New(symbols []model.Symbol) *Resolver {
	r := &Resolver{byHandle: map[string]model.Symbol{}, byQualified: map[string][]model.Symbol{}, children: map[string]map[string][]model.Symbol{}, types: map[string][]model.Symbol{}}
	for _, s := range symbols {
		if s.Language != "csharp" {
			continue
		}
		r.byHandle[s.Handle] = s
		if isType(s.Kind) {
			r.types[s.QualifiedName] = append(r.types[s.QualifiedName], s)
		}
		if !callable(s.Kind) {
			continue
		}
		if r.children[s.ParentHandle] == nil {
			r.children[s.ParentHandle] = map[string][]model.Symbol{}
		}
		r.children[s.ParentHandle][s.Name] = append(r.children[s.ParentHandle][s.Name], s)
	}
	for _, s := range symbols {
		parent := r.byHandle[s.ParentHandle]
		if s.Language == "csharp" && callable(s.Kind) && isType(parent.Kind) {
			r.byQualified[s.QualifiedName] = append(r.byQualified[s.QualifiedName], s)
		}
	}
	return r
}

func (r *Resolver) Resolve(from model.Symbol, target string) []model.Symbol {
	target = strings.TrimPrefix(strings.TrimSpace(target), "global::")
	if target == "" {
		return nil
	}
	owner, hasType := r.declaringType(from)
	if dot := strings.LastIndexByte(target, '.'); dot >= 0 {
		receiver, name := target[:dot], target[dot+1:]
		if receiver == "this" {
			if hasType {
				return r.byQualified[owner.QualifiedName+"."+name]
			}
			return nil
		}
		if receiver == "base" {
			return nil
		}
		// Only an actual type name can qualify a call. Unknown object receivers
		// never fall back to a same-named member in the declaring type.
		for _, prefix := range prefixes(owner.QualifiedName) {
			typeName := prefix + receiver
			if len(r.types[typeName]) > 0 {
				return r.byQualified[typeName+"."+name]
			}
		}
		return nil
	}
	// Local functions belong to the current or an enclosing callable scope.
	current := from
	seen := map[string]bool{}
	for current.Handle != "" && !seen[current.Handle] {
		seen[current.Handle] = true
		if callable(current.Kind) {
			if candidates := r.children[current.Handle][target]; len(candidates) > 0 {
				return candidates
			}
		}
		parent, ok := r.byHandle[current.ParentHandle]
		if !ok {
			break
		}
		if isType(parent.Kind) {
			break
		}
		current = parent
	}
	if hasType {
		// Qualified lookup also includes members from another partial declaration.
		if candidates := r.byQualified[owner.QualifiedName+"."+target]; len(candidates) > 0 {
			return candidates
		}
		return nil
	}
	// Top-level script functions may resolve only within their source file.
	result := []model.Symbol{}
	for _, s := range r.children[from.ParentHandle][target] {
		if s.FilePath == from.FilePath {
			result = append(result, s)
		}
	}
	return result
}

func (r *Resolver) declaringType(from model.Symbol) (model.Symbol, bool) {
	seen := map[string]bool{}
	for from.Handle != "" && !seen[from.Handle] {
		seen[from.Handle] = true
		if isType(from.Kind) {
			return from, true
		}
		parent, ok := r.byHandle[from.ParentHandle]
		if !ok {
			break
		}
		from = parent
	}
	return model.Symbol{}, false
}

// Prefer the nearest type/namespace qualification; exact global names remain
// available after all enclosing qualifications have been considered.
func prefixes(qualified string) []string {
	result := []string{}
	for qualified != "" {
		result = append(result, qualified+".")
		dot := strings.LastIndexByte(qualified, '.')
		if dot < 0 {
			break
		}
		qualified = qualified[:dot]
	}
	return append(result, "")
}

func isType(kind string) bool {
	return kind == "class" || kind == "struct" || kind == "interface" || kind == "record" || kind == "enum"
}
func callable(kind string) bool {
	return kind == "method" || kind == "function" || kind == "constructor" || kind == "destructor" || kind == "operator" || kind == "test"
}
