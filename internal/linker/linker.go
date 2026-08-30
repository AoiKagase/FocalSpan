package linker

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/projectmeta"
	"github.com/focalspan/focalspan/internal/store"
)

type Linker struct {
	Store *store.Store
}

func (l *Linker) Link(ctx context.Context, facts []projectmeta.Fact) error {
	if l == nil || l.Store == nil {
		return errors.New("linker store is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	symbols, err := l.Store.Symbols(ctx)
	if err != nil {
		return err
	}
	relations, err := l.Store.Relations(ctx)
	if err != nil {
		return err
	}
	byHandle := make(map[string]model.Symbol, len(symbols))
	for _, symbol := range symbols {
		byHandle[symbol.Handle] = symbol
	}
	orderedFacts := append([]projectmeta.Fact(nil), facts...)
	sort.SliceStable(orderedFacts, func(i, j int) bool {
		if orderedFacts[i].SourcePath != orderedFacts[j].SourcePath {
			return orderedFacts[i].SourcePath < orderedFacts[j].SourcePath
		}
		return orderedFacts[i].Target < orderedFacts[j].Target
	})
	for _, relation := range relations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if relation.UnresolvedTo == "" || relation.ToHandle != "" {
			continue
		}
		from := byHandle[relation.FromHandle]
		candidates := resolveCandidates(from, relation.UnresolvedTo, symbols, orderedFacts)
		if len(candidates) != 1 {
			continue
		}
		if err := l.Store.LinkRelation(ctx, relation.FromHandle, relation.UnresolvedTo, relation.Kind, candidates[0].Handle); err != nil {
			return err
		}
	}
	return nil
}

func resolveCandidates(from model.Symbol, target string, symbols []model.Symbol, facts []projectmeta.Fact) []model.Symbol {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	pathCandidates := make([]model.Symbol, 0)
	for _, symbol := range symbols {
		if symbol.Handle == from.Handle {
			continue
		}
		if linkerPathMatch(from.FilePath, symbol.FilePath, target) {
			pathCandidates = append(pathCandidates, symbol)
		}
	}
	if len(pathCandidates) > 0 {
		return uniqueSymbols(pathCandidates)
	}
	pathCandidates = factPathCandidates(target, symbols, facts)
	if len(pathCandidates) > 0 {
		return uniqueSymbols(pathCandidates)
	}
	if len(pathCandidates) > 0 {
		return uniqueSymbols(pathCandidates)
	}
	qualified := make([]model.Symbol, 0)
	for _, symbol := range symbols {
		if symbol.Handle != from.Handle && strings.EqualFold(symbol.QualifiedName, target) {
			qualified = append(qualified, symbol)
		}
	}
	if len(qualified) > 0 {
		return uniqueSymbols(qualified)
	}
	name := make([]model.Symbol, 0)
	for _, symbol := range symbols {
		if symbol.Handle != from.Handle && strings.EqualFold(symbol.Name, target) {
			name = append(name, symbol)
		}
	}
	return uniqueSymbols(name)
}

func factPathCandidates(target string, symbols []model.Symbol, facts []projectmeta.Fact) []model.Symbol {
	candidates := make([]model.Symbol, 0)
	for _, fact := range facts {
		for _, expected := range factCandidatePaths(fact, target) {
			for _, symbol := range symbols {
				if sourcePathMatches(symbol.FilePath, expected) {
					candidates = append(candidates, symbol)
				}
			}
		}
	}
	return uniqueSymbols(candidates)
}

func factCandidatePaths(fact projectmeta.Fact, target string) []string {
	manifestDir := filepath.ToSlash(filepath.Dir(fact.SourcePath))
	if manifestDir == "." {
		manifestDir = ""
	}
	normalizedTarget := strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	switch fact.Kind {
	case "module":
		module := strings.Trim(strings.ReplaceAll(fact.Target, "\\", "/"), "/")
		if module == "" || !sameOrChildImport(normalizedTarget, module) {
			return nil
		}
		suffix := strings.Trim(strings.TrimPrefix(normalizedTarget, module), "/")
		if suffix == "" {
			return []string{manifestDir}
		}
		return []string{filepath.ToSlash(filepath.Join(manifestDir, suffix))}
	case "psr-4", "psr-0":
		prefix := strings.ReplaceAll(fact.Name, "\\", "/")
		prefix = strings.Trim(prefix, "/")
		prefixWithSeparator := prefix
		if prefixWithSeparator != "" {
			prefixWithSeparator += "/"
		}
		if prefixWithSeparator != "" && !strings.HasPrefix(strings.ToLower(normalizedTarget), strings.ToLower(prefixWithSeparator)) {
			return nil
		}
		remainder := normalizedTarget
		if prefixWithSeparator != "" {
			remainder = normalizedTarget[len(prefixWithSeparator):]
		}
		remainder = strings.Trim(remainder, "/")
		remainder = strings.ReplaceAll(remainder, "::", "/")
		remainder = strings.ReplaceAll(remainder, "\\", "/")
		base := strings.ReplaceAll(fact.Target, "\\", "/")
		mapped := filepath.ToSlash(filepath.Join(manifestDir, base, strings.ReplaceAll(remainder, ".", "/")))
		return []string{mapped, mapped + ".php"}
	case "path-alias":
		alias := strings.ReplaceAll(fact.Name, "\\", "/")
		aliasPrefix := strings.TrimSuffix(alias, "*")
		if aliasPrefix != "" && !strings.HasPrefix(normalizedTarget, aliasPrefix) {
			return nil
		}
		remainder := strings.TrimPrefix(normalizedTarget, aliasPrefix)
		base := strings.ReplaceAll(fact.Target, "\\", "/")
		mapped := strings.ReplaceAll(base, "*", remainder)
		return []string{filepath.ToSlash(filepath.Join(manifestDir, mapped))}
	default:
		if fact.Target == "" || (!strings.Contains(fact.Target, "/") && !strings.Contains(fact.Target, "\\") && filepath.Ext(fact.Target) == "") {
			return nil
		}
		return []string{filepath.ToSlash(filepath.Join(manifestDir, strings.ReplaceAll(fact.Target, "\\", "/")))}
	}
}

func sameOrChildImport(target, module string) bool {
	return strings.EqualFold(target, module) || strings.HasPrefix(strings.ToLower(target), strings.ToLower(module)+"/")
}

func sourcePathMatches(candidate, target string) bool {
	candidate = strings.TrimPrefix(strings.ReplaceAll(candidate, "\\", "/"), "./")
	target = strings.TrimPrefix(strings.ReplaceAll(target, "\\", "/"), "./")
	if sameStorePath(candidate, target) {
		return true
	}
	withoutExtension := strings.TrimSuffix(candidate, filepath.Ext(candidate))
	return sameStorePath(withoutExtension, target) || strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(target)+"/")
}

func linkerPathMatch(importer, candidate, target string) bool {
	target = strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	if target == "" {
		return false
	}
	if sameStorePath(candidate, target) {
		return true
	}
	if filepath.Ext(target) == "" {
		base := strings.TrimSuffix(filepath.ToSlash(candidate), filepath.Ext(candidate))
		if sameStorePath(base, target) || strings.HasSuffix(strings.ToLower(base), "/"+strings.ToLower(target)) || strings.EqualFold(filepath.Base(base), target) {
			return true
		}
	}
	if importer != "" {
		dir := filepath.ToSlash(filepath.Dir(importer))
		joined := filepath.ToSlash(filepath.Join(dir, target))
		if sameStorePath(candidate, joined) || strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(joined)+"/") {
			return true
		}
	}
	return false
}

func sameFactTarget(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(strings.ReplaceAll(left, "\\", "/")), strings.TrimSpace(strings.ReplaceAll(right, "\\", "/")))
}

func uniqueSymbols(symbols []model.Symbol) []model.Symbol {
	seen := make(map[string]bool)
	result := make([]model.Symbol, 0, len(symbols))
	for _, symbol := range symbols {
		if seen[symbol.Handle] {
			continue
		}
		seen[symbol.Handle] = true
		result = append(result, symbol)
	}
	return result
}

func sameStorePath(left, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(strings.ReplaceAll(left, "\\", "/"), "./"), strings.TrimPrefix(strings.ReplaceAll(right, "\\", "/"), "./"))
}
