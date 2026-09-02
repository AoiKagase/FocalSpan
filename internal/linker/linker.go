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
	return l.LinkWithScope(ctx, facts, store.LinkScope{Full: true}, nil)
}

func (l *Linker) LinkWithScope(ctx context.Context, facts []projectmeta.Fact, scope store.LinkScope, progress func(completed, total int)) error {
	if l == nil || l.Store == nil {
		return errors.New("linker store is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !scope.Full && len(scope.ChangedPaths) == 0 && len(scope.LookupKeys) == 0 {
		return nil
	}
	if scope.Full && len(facts) == 0 {
		scope.UseProjection = true
	}
	symbols, err := l.Store.Symbols(ctx)
	if err != nil {
		return err
	}
	relations, err := l.Store.RelationsForLink(ctx, scope)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(0, len(relations))
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
	index := newSymbolIndex(symbols)
	links := make([]store.RelationLink, 0)
	for indexRelation, relation := range relations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if relation.UnresolvedTo == "" || relation.ToHandle != "" {
			continue
		}
		from := byHandle[relation.FromHandle]
		candidatePool := index.candidates(from, relation.UnresolvedTo, orderedFacts)
		if len(candidatePool) == 0 {
			if progress != nil {
				progress(indexRelation+1, len(relations))
			}
			continue
		}
		candidates := resolveCandidates(from, relation.UnresolvedTo, candidatePool, orderedFacts)
		if len(candidates) != 1 {
			if progress != nil {
				progress(indexRelation+1, len(relations))
			}
			continue
		}
		links = append(links, store.RelationLink{FromHandle: relation.FromHandle, UnresolvedTo: relation.UnresolvedTo, Kind: relation.Kind, ToHandle: candidates[0].Handle})
		if progress != nil {
			progress(indexRelation+1, len(relations))
		}
	}
	return l.Store.LinkRelations(ctx, links)
}

type symbolIndex struct {
	all       []model.Symbol
	byLookup  map[string][]model.Symbol
	byFileKey map[string][]model.Symbol
}

func newSymbolIndex(symbols []model.Symbol) symbolIndex {
	index := symbolIndex{all: symbols, byLookup: make(map[string][]model.Symbol), byFileKey: make(map[string][]model.Symbol)}
	for _, symbol := range symbols {
		for kind, key := range linkerSymbolKeys(symbol) {
			index.byLookup[kind+"\x00"+key] = append(index.byLookup[kind+"\x00"+key], symbol)
		}
		for _, key := range linkerFileKeys(symbol.FilePath) {
			index.byFileKey[key] = append(index.byFileKey[key], symbol)
		}
	}
	return index
}

func (i symbolIndex) candidates(from model.Symbol, target string, facts []projectmeta.Fact) []model.Symbol {
	set := make(map[string]model.Symbol)
	add := func(symbols []model.Symbol) {
		for _, symbol := range symbols {
			if symbol.Handle != from.Handle {
				set[symbol.Handle] = symbol
			}
		}
	}
	normalized := normalizeLinkerKey(target)
	add(i.byLookup["name\x00"+normalized])
	add(i.byLookup["qualified\x00"+normalized])
	for _, key := range linkerFileKeys(target) {
		add(i.byFileKey[key])
	}
	for _, key := range linkerModuleKeys(target) {
		add(i.byFileKey[key])
	}
	if from.FilePath != "" {
		dir := filepath.ToSlash(filepath.Dir(from.FilePath))
		for _, key := range linkerFileKeys(filepath.ToSlash(filepath.Join(dir, strings.ReplaceAll(target, "\\", "/")))) {
			add(i.byFileKey[key])
		}
		for _, path := range linkerPythonPaths(from.FilePath, target) {
			for _, key := range linkerFileKeys(path) {
				add(i.byFileKey[key])
			}
		}
	}
	for _, fact := range facts {
		for _, path := range factCandidatePaths(fact, target) {
			for _, key := range linkerFileKeys(path) {
				add(i.byFileKey[key])
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	result := make([]model.Symbol, 0, len(set))
	for _, symbol := range i.all {
		if _, ok := set[symbol.Handle]; ok {
			result = append(result, symbol)
		}
	}
	return result
}

func normalizeLinkerKey(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
}

func linkerSymbolKeys(symbol model.Symbol) map[string]string {
	result := make(map[string]string, 2)
	if key := normalizeLinkerKey(symbol.Name); key != "" {
		result["name"] = key
	}
	if key := normalizeLinkerKey(symbol.QualifiedName); key != "" {
		result["qualified"] = key
	}
	return result
}

func linkerFileKeys(path string) []string {
	normalized := normalizeLinkerKey(path)
	if normalized == "" {
		return nil
	}
	result := []string{normalized}
	if extension := filepath.Ext(normalized); extension != "" {
		result = append(result, strings.TrimSuffix(normalized, extension))
	}
	if extensionless := strings.TrimSuffix(normalized, filepath.Ext(normalized)); extensionless != "" {
		result = append(result, strings.ReplaceAll(extensionless, "/", "."))
	}
	if base := filepath.Base(normalized); base != "." && base != "" {
		result = append(result, base)
	}
	parts := strings.Split(strings.Trim(normalized, "/"), "/")
	for start := 1; start < len(parts); start++ {
		if suffix := strings.Join(parts[start:], "/"); suffix != "" {
			result = append(result, suffix)
		}
	}
	if extensionless := strings.TrimSuffix(normalized, filepath.Ext(normalized)); extensionless != normalized {
		extensionlessParts := strings.Split(strings.Trim(extensionless, "/"), "/")
		for start := 1; start < len(extensionlessParts); start++ {
			if suffix := strings.Join(extensionlessParts[start:], "/"); suffix != "" {
				result = append(result, suffix)
			}
		}
	}
	for start := 0; start < len(parts)-1; start++ {
		if directory := strings.Join(parts[start:len(parts)-1], "/"); directory != "" {
			result = append(result, directory)
		}
	}
	return result
}

func linkerModuleKeys(target string) []string {
	normalized := normalizeLinkerKey(target)
	result := make([]string, 0, 2)
	if strings.Contains(normalized, "::") {
		parts := strings.Split(normalized, "::")
		if len(parts) > 1 {
			parts = parts[:len(parts)-1]
			if len(parts) > 0 && (parts[0] == "crate" || parts[0] == "self" || parts[0] == "super") {
				parts = parts[1:]
			}
			if len(parts) > 0 {
				result = append(result, strings.Join(parts, "/"))
			}
		}
	}
	return result
}

func linkerPythonPaths(importer, target string) []string {
	if !strings.HasSuffix(strings.ToLower(importer), ".py") && !strings.HasSuffix(strings.ToLower(importer), ".pyi") {
		return nil
	}
	leadingDots := len(target) - len(strings.TrimLeft(target, "."))
	module := strings.TrimLeft(target, ".")
	if module == "" {
		return nil
	}
	parts := strings.Split(module, ".")
	if len(parts) > 1 && parts[len(parts)-1] != "" && parts[len(parts)-1][0] >= 'A' && parts[len(parts)-1][0] <= 'Z' {
		parts = parts[:len(parts)-1]
	}
	module = strings.Join(parts, "/")
	base := ""
	if leadingDots > 0 {
		base = filepath.ToSlash(filepath.Dir(importer))
		for level := 1; level < leadingDots; level++ {
			base = filepath.ToSlash(filepath.Dir(base))
		}
	}
	return []string{filepath.ToSlash(filepath.Join(base, module))}
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
		if narrowed := narrowPathCandidates(target, pathCandidates); len(narrowed) > 0 {
			return uniqueSymbols(narrowed)
		}
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

func narrowPathCandidates(target string, candidates []model.Symbol) []model.Symbol {
	leaf := ""
	if strings.Contains(target, "::") {
		leaf = target[strings.LastIndex(target, "::")+2:]
	} else if dot := strings.LastIndexByte(target, '.'); dot >= 0 {
		leaf = target[dot+1:]
		if leaf == "" || leaf[0] < 'A' || leaf[0] > 'Z' {
			return nil
		}
	} else {
		return nil
	}
	leaf = strings.TrimSpace(leaf)
	if leaf == "" {
		return nil
	}
	result := make([]model.Symbol, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.Name, leaf) || strings.HasSuffix(strings.ToLower(candidate.QualifiedName), "::"+strings.ToLower(leaf)) {
			result = append(result, candidate)
		}
	}
	return result
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
	if rustModulePathMatch(candidate, target) || pythonModulePathMatch(importer, candidate, target) {
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

func rustModulePathMatch(candidate, target string) bool {
	if !strings.Contains(target, "::") {
		return false
	}
	parts := strings.Split(target, "::")
	if len(parts) < 2 {
		return false
	}
	parts = parts[:len(parts)-1]
	if len(parts) > 0 && (parts[0] == "crate" || parts[0] == "self" || parts[0] == "super") {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return false
	}
	expected := strings.ToLower(strings.Join(parts, "/"))
	path := strings.ToLower(strings.ReplaceAll(candidate, "\\", "/"))
	withoutExtension := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.HasSuffix(withoutExtension, "/"+expected) || strings.HasSuffix(withoutExtension, "/"+expected+"/mod")
}

func pythonModulePathMatch(importer, candidate, target string) bool {
	if !strings.HasSuffix(strings.ToLower(importer), ".py") && !strings.HasSuffix(strings.ToLower(importer), ".pyi") {
		return false
	}
	leadingDots := len(target) - len(strings.TrimLeft(target, "."))
	module := strings.TrimLeft(target, ".")
	if module == "" {
		return false
	}
	module = strings.ReplaceAll(module, ".", "/")
	base := ""
	if leadingDots > 0 {
		base = filepath.ToSlash(filepath.Dir(importer))
		for level := 1; level < leadingDots; level++ {
			base = filepath.ToSlash(filepath.Dir(base))
		}
	}
	expected := filepath.ToSlash(filepath.Join(base, module))
	if sourcePathMatches(candidate, expected) {
		return true
	}
	if separator := strings.LastIndexByte(expected, '/'); separator > 0 {
		return sourcePathMatches(candidate, expected[:separator])
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
