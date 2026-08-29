package query

import (
	"strings"
	"unicode"
)

const (
	maxTokenRunes = 128
	maxFTSTerms   = 32
)

// Normalize separates natural-language words from code-shaped terms without
// interpreting the input as a query language. It deliberately keeps the
// original spelling of identifiers and paths for exact retrieval.
func Normalize(raw string) Terms {
	var result Terms
	seenWords := make(map[string]bool)
	seenIdentifiers := make(map[string]bool)
	seenPhrases := make(map[string]bool)
	seenPaths := make(map[string]bool)
	seenSymbols := make(map[string]bool)
	seenUnicode := make(map[string]bool)

	addWord := func(value string) {
		value = bounded(strings.ToLower(strings.TrimSpace(value)))
		if value == "" || seenWords[value] {
			return
		}
		seenWords[value] = true
		result.Words = append(result.Words, value)
	}
	addIdentifier := func(value string) {
		value = bounded(strings.TrimSpace(value))
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seenIdentifiers[key] {
			return
		}
		seenIdentifiers[key] = true
		result.Identifiers = append(result.Identifiers, value)
	}
	addPhrase := func(value string) {
		value = bounded(strings.TrimSpace(value))
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seenPhrases[key] {
			return
		}
		seenPhrases[key] = true
		result.Phrases = append(result.Phrases, value)
	}
	addPath := func(value string) {
		value = bounded(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
		if value == "" || seenPaths[strings.ToLower(value)] {
			return
		}
		seenPaths[strings.ToLower(value)] = true
		result.Paths = append(result.Paths, value)
	}
	addSymbol := func(value string) {
		value = bounded(strings.TrimSpace(value))
		if value == "" || seenSymbols[strings.ToLower(value)] {
			return
		}
		seenSymbols[strings.ToLower(value)] = true
		result.Symbols = append(result.Symbols, value)
	}
	addUnicodeRun := func(value string) {
		value = bounded(strings.TrimSpace(value))
		if value == "" || seenUnicode[value] {
			return
		}
		seenUnicode[value] = true
		result.UnicodeRuns = append(result.UnicodeRuns, value)
		addWord(value)
	}

	runes := []rune(raw)
	for index := 0; index < len(runes); {
		if runes[index] == '\x00' {
			index++
			continue
		}
		if runes[index] == '"' {
			end := index + 1
			for end < len(runes) && runes[end] != '"' && runes[end] != '\x00' {
				end++
			}
			if end < len(runes) && runes[end] == '"' {
				phrase := strings.TrimSpace(string(runes[index+1 : end]))
				addPhrase(phrase)
				for _, part := range scanTerms([]rune(phrase)) {
					consumeToken(part, addWord, addIdentifier, addPath, addSymbol)
				}
				index = end + 1
				continue
			}
			// An unmatched quote is punctuation, so continue scanning the
			// remaining input as ordinary lexical terms.
			index++
			continue
		}
		if isUnicodeWordRune(runes[index]) {
			start := index
			for index < len(runes) && isUnicodeWordRune(runes[index]) {
				index++
			}
			for _, part := range splitJapaneseRun(string(runes[start:index])) {
				addUnicodeRun(part)
			}
			continue
		}
		if isTokenRune(runes[index]) {
			start := index
			for index < len(runes) && isTokenRune(runes[index]) {
				index++
			}
			consumeToken(string(runes[start:index]), addWord, addIdentifier, addPath, addSymbol)
			continue
		}
		index++
	}

	return result
}

func consumeToken(token string, addWord func(string), addIdentifier func(string), addPath func(string), addSymbol func(string)) {
	token = bounded(strings.TrimSpace(token))
	if token == "" || strings.Trim(token, "_:./\\-") == "" {
		return
	}
	if looksLikePath(token) {
		addPath(token)
	}
	identifier := probableIdentifier(token)
	if identifier {
		addIdentifier(token)
	}
	if probableSymbol(token) && !looksLikePath(token) {
		addSymbol(token)
	}
	addWord(token)
	if identifier {
		for _, segment := range splitDelimitedIdentifier(token) {
			if probableIdentifier(segment) {
				addIdentifier(segment)
			}
			addWord(segment)
			for _, part := range splitIdentifier(segment) {
				addWord(part)
			}
		}
	}
}

func scanTerms(runes []rune) []string {
	terms := make([]string, 0)
	for index := 0; index < len(runes); {
		if isUnicodeWordRune(runes[index]) {
			start := index
			for index < len(runes) && isUnicodeWordRune(runes[index]) {
				index++
			}
			terms = append(terms, string(runes[start:index]))
			continue
		}
		if isTokenRune(runes[index]) {
			start := index
			for index < len(runes) && isTokenRune(runes[index]) {
				index++
			}
			terms = append(terms, string(runes[start:index]))
			continue
		}
		index++
	}
	return terms
}

func isUnicodeWordRune(r rune) bool {
	return r >= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) && r < unicode.MaxASCII || unicode.IsDigit(r) && r < unicode.MaxASCII || strings.ContainsRune("_:/\\.-", r)
}

func splitJapaneseRun(value string) []string {
	const particles = "のをはがにへとでやも"
	runes := []rune(value)
	parts := make([]string, 0, 2)
	start := 0
	for index := 0; index < len(runes); index++ {
		if !strings.ContainsRune(particles, runes[index]) {
			continue
		}
		if index > start {
			parts = append(parts, string(runes[start:index]))
		}
		parts = append(parts, string(runes[index]))
		start = index + 1
	}
	if start < len(runes) {
		parts = append(parts, string(runes[start:]))
	}
	if len(parts) == 0 {
		return []string{value}
	}
	return parts
}

func looksLikePath(value string) bool {
	value = strings.ReplaceAll(value, "\\", "/")
	if !strings.Contains(value, "/") {
		return false
	}
	if strings.Contains(value, ":/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return true
	}
	base := value[strings.LastIndexByte(value, '/')+1:]
	return strings.Contains(base, ".")
}

func probableIdentifier(value string) bool {
	return hasUpperBoundary(value) || strings.ContainsAny(value, "_:.")
}

func probableSymbol(value string) bool {
	return hasUpperBoundary(value) || strings.ContainsAny(value, "_:.\\")
}

func hasUpperBoundary(value string) bool {
	seen := false
	for _, r := range value {
		if seen && unicode.IsUpper(r) {
			return true
		}
		seen = true
	}
	return false
}

func splitIdentifier(value string) []string {
	var parts []string
	var current []rune
	runes := []rune(value)
	flush := func() {
		if len(current) == 0 {
			return
		}
		parts = append(parts, string(current))
		current = current[:0]
	}
	for index, r := range runes {
		if strings.ContainsRune("_:./\\-", r) {
			flush()
			continue
		}
		if index > 0 && unicode.IsUpper(r) {
			previous := runes[index-1]
			var next rune
			if index+1 < len(runes) {
				next = runes[index+1]
			}
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsLower(next) && unicode.IsUpper(previous) {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()
	return parts
}

func splitDelimitedIdentifier(value string) []string {
	parts := make([]string, 0, 2)
	start := 0
	runes := []rune(value)
	for index, r := range runes {
		if !strings.ContainsRune("_:./\\-", r) {
			continue
		}
		if index > start {
			parts = append(parts, string(runes[start:index]))
		}
		start = index + 1
	}
	if start < len(runes) {
		parts = append(parts, string(runes[start:]))
	}
	return parts
}

func bounded(value string) string {
	runes := []rune(value)
	if len(runes) > maxTokenRunes {
		runes = runes[:maxTokenRunes]
	}
	return string(runes)
}
