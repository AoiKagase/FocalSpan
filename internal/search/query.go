package search

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type QueryTerms struct {
	Words       []string
	Identifiers []string
	Phrases     []string
	Paths       []string
	Symbols     []string
}

var queryTokenPattern = regexp.MustCompile(`[A-Za-z0-9_:.\\/-]+`)
var phrasePattern = regexp.MustCompile(`"([^"]+)"`)

func NormalizeQuery(query string) QueryTerms {
	result := QueryTerms{}
	seen := map[string]bool{}
	addWord := func(word string) {
		word = strings.ToLower(strings.TrimSpace(word))
		if word != "" && !seen[word] {
			seen[word] = true
			result.Words = append(result.Words, word)
		}
	}
	for _, phrase := range phrasePattern.FindAllStringSubmatch(query, -1) {
		if len(phrase) == 2 {
			result.Phrases = append(result.Phrases, phrase[1])
		}
	}
	for _, token := range queryTokenPattern.FindAllString(query, -1) {
		if strings.Contains(token, "/") || strings.Contains(token, "\\") || strings.Contains(token, ".go") || strings.Contains(token, ".cs") || strings.Contains(token, ".py") {
			result.Paths = appendUnique(result.Paths, strings.ReplaceAll(token, "\\", "/"))
		}
		if strings.IndexFunc(token, unicode.IsUpper) >= 0 || strings.ContainsAny(token, "_:.") {
			result.Identifiers = appendUnique(result.Identifiers, token)
			for _, segment := range strings.FieldsFunc(token, func(r rune) bool { return r == '_' || r == ':' || r == '.' || r == '/' || r == '\\' || r == '-' }) {
				if probableSymbol(segment) {
					result.Identifiers = appendUnique(result.Identifiers, segment)
				}
			}
		}
		if strings.ContainsAny(token, "_:. ") || hasUpperBoundary(token) {
			if dot := strings.IndexByte(token, '.'); dot > 0 {
				addWord(token[:dot])
			}
			for _, segment := range strings.FieldsFunc(token, func(r rune) bool { return r == '_' || r == ':' || r == '.' || r == '/' || r == '\\' || r == '-' }) {
				addWord(segment)
			}
			for _, part := range splitIdentifier(token) {
				addWord(part)
			}
		} else {
			addWord(token)
		}
		if probableSymbol(token) {
			result.Symbols = appendUnique(result.Symbols, token)
		}
	}
	for _, phrase := range result.Phrases {
		for _, part := range splitIdentifier(phrase) {
			addWord(part)
		}
	}
	return result
}

func BuildFTSQuery(query string) string {
	terms := NormalizeQuery(query)
	parts := make([]string, 0, len(terms.Words)+len(terms.Phrases)+len(terms.Identifiers))
	for _, word := range terms.Words {
		parts = append(parts, `"`+strings.ReplaceAll(word, `"`, `""`)+`"`)
	}
	for _, identifier := range terms.Identifiers {
		parts = append(parts, `"`+strings.ReplaceAll(identifier, `"`, `""`)+`"`)
	}
	for _, phrase := range terms.Phrases {
		parts = append(parts, `"`+strings.ReplaceAll(phrase, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " OR ")
}

func splitIdentifier(value string) []string {
	value = strings.NewReplacer("::", " ", ".", " ", "_", " ", "/", " ", "\\", " ", "-", " ").Replace(value)
	value = regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(value, `$1 $2`)
	return strings.FieldsFunc(value, unicode.IsSpace)
}

func hasUpperBoundary(value string) bool {
	for i, r := range value {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func probableSymbol(value string) bool {
	return strings.IndexFunc(value, unicode.IsUpper) >= 0 || strings.ContainsAny(value, "_:.")
}

func appendUnique(values []string, value string) []string {
	for _, old := range values {
		if old == value {
			return values
		}
	}
	return append(values, value)
}

func sortedUnique(values []string) []string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
