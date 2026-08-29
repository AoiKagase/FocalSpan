package query

import "strings"

// BuildFTS creates an FTS5 expression made exclusively of quoted terms.
// User-provided operators can therefore never become part of the expression.
func BuildFTS(terms Terms) string {
	parts := make([]string, 0, maxFTSTerms)
	seen := make(map[string]bool, maxFTSTerms)
	appendTerm := func(value string) bool {
		value = bounded(strings.TrimSpace(value))
		if value == "" {
			return true
		}
		key := strings.ToLower(value)
		if seen[key] {
			return true
		}
		if len(parts) >= maxFTSTerms {
			return false
		}
		seen[key] = true
		parts = append(parts, `"`+strings.ReplaceAll(value, `"`, `""`)+`"`)
		return true
	}
	for _, phrase := range terms.Phrases {
		if !appendTerm(phrase) {
			return strings.Join(parts, " OR ")
		}
	}
	for _, value := range terms.Identifiers {
		if !appendTerm(value) {
			return strings.Join(parts, " OR ")
		}
	}
	for _, value := range terms.Symbols {
		if !appendTerm(value) {
			return strings.Join(parts, " OR ")
		}
	}
	for _, value := range terms.Words {
		if !appendTerm(value) {
			return strings.Join(parts, " OR ")
		}
	}
	for _, value := range terms.UnicodeRuns {
		if !appendTerm(value) {
			return strings.Join(parts, " OR ")
		}
	}
	return strings.Join(parts, " OR ")
}
