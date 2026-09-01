package search

import (
	"strings"

	"github.com/focalspan/focalspan/internal/query"
)

const identityBridgeHintLimit = 4

var structuralMarkers = map[string]bool{
	"package":    true,
	"packages":   true,
	"module":     true,
	"modules":    true,
	"namespace":  true,
	"namespaces": true,
	"crate":      true,
	"crates":     true,
	"パッケージ":      true,
	"モジュール":      true,
	"名前空間":       true,
	"クレート":       true,
}

var bridgeNoiseWords = map[string]bool{
	"a": true, "an": true, "and": true, "by": true, "find": true,
	"for": true, "from": true, "in": true, "is": true, "locate": true,
	"of": true, "show": true, "the": true, "to": true, "where": true,
	"with": true, "within": true, "の": true, "を": true, "は": true,
	"が": true, "に": true, "へ": true, "と": true, "で": true, "や": true,
	"も": true,
}

// identityBridgeHints extracts only values adjacent to an explicit structural
// marker. It deliberately does not turn ordinary natural-language words into
// path hints; explicit path queries remain owned by SearchPaths.
func identityBridgeHints(plan query.Plan) (packageHints, symbolHints []string) {
	if len(plan.Terms.Paths) > 0 {
		return nil, nil
	}
	words := plan.Terms.Words
	if len(words) == 0 {
		return nil, nil
	}
	original := make(map[string]string, len(plan.Terms.Identifiers)+len(plan.Terms.Symbols)+len(plan.Terms.Paths))
	for _, value := range append(append(append([]string{}, plan.Terms.Identifiers...), plan.Terms.Symbols...), plan.Terms.Paths...) {
		key := strings.ToLower(strings.TrimSpace(value))
		if key != "" {
			if _, exists := original[key]; !exists {
				original[key] = value
			}
		}
	}
	seen := make(map[string]bool, identityBridgeHintLimit)
	addHint := func(value string) {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || structuralMarkers[key] || bridgeNoiseWords[key] || seen[key] || len(packageHints) >= identityBridgeHintLimit {
			return
		}
		seen[key] = true
		if preserved, ok := original[key]; ok {
			value = preserved
		}
		packageHints = append(packageHints, value)
	}
	for index, word := range words {
		marker := strings.ToLower(strings.TrimSpace(word))
		if !structuralMarkers[marker] {
			continue
		}
		for distance := 1; distance < len(words); distance++ {
			added := false
			for _, candidateIndex := range []int{index - distance, index + distance} {
				if candidateIndex < 0 || candidateIndex >= len(words) {
					continue
				}
				candidate := strings.TrimSpace(words[candidateIndex])
				candidateKey := strings.ToLower(candidate)
				if candidate == "" || structuralMarkers[candidateKey] || bridgeNoiseWords[candidateKey] {
					continue
				}
				addHint(candidate)
				added = true
				break
			}
			if added {
				break
			}
		}
	}
	if len(packageHints) == 0 {
		// When no explicit marker is present, use a small lexical seed set as
		// structural probes. The store still scopes these probes to files with
		// structural entry symbols, so ordinary documentation cannot become an
		// anchor and no value is sent to SearchPaths.
		lexical := make([]string, 0, identityBridgeHintLimit)
		for _, word := range words {
			candidate := strings.TrimSpace(word)
			key := strings.ToLower(candidate)
			if candidate == "" || len([]rune(candidate)) < 3 || structuralMarkers[key] || bridgeNoiseWords[key] || seenSymbols(plan, key) {
				continue
			}
			lexical = append(lexical, candidate)
			if len(lexical) >= identityBridgeHintLimit {
				break
			}
		}
		// A single generic noun is too weak to justify a structural probe.
		// Requiring two lexical seeds keeps ordinary questions out while still
		// allowing natural-language package/module-to-symbol bridging.
		if len(lexical) >= 2 {
			for _, candidate := range lexical {
				addHint(candidate)
			}
		}
	}
	if len(packageHints) == 0 {
		return nil, nil
	}
	seenSymbols := make(map[string]bool, len(plan.Terms.Symbols))
	for _, value := range append(append([]string{}, plan.Terms.Symbols...), plan.Terms.Identifiers...) {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seenSymbols[key] || seen[key] || structuralMarkers[key] || bridgeNoiseWords[key] {
			continue
		}
		if strings.ContainsAny(value, "/\\") {
			continue
		}
		seenSymbols[key] = true
		symbolHints = append(symbolHints, value)
	}
	return packageHints, symbolHints
}

func seenSymbols(plan query.Plan, key string) bool {
	for _, value := range append(append([]string{}, plan.Terms.Symbols...), plan.Terms.Identifiers...) {
		if strings.ToLower(strings.TrimSpace(value)) == key {
			return true
		}
	}
	return false
}
