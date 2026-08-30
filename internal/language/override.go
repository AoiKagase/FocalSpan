package language

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func selectOverride(filePath string, overrides map[string]string) (string, bool) {
	normalized := strings.TrimPrefix(strings.ReplaceAll(filepath.ToSlash(filePath), "\\", "/"), "./")
	keys := make([]string, 0, len(overrides))
	for pattern := range overrides {
		keys = append(keys, pattern)
	}
	sort.Strings(keys)
	bestPattern := ""
	bestSpecificity := -1
	for _, pattern := range keys {
		language := overrides[pattern]
		if !IsKnown(language) || !globMatch(pattern, normalized) {
			continue
		}
		specificity := globSpecificity(pattern)
		if specificity > bestSpecificity || specificity == bestSpecificity && (bestPattern == "" || pattern < bestPattern) {
			bestPattern = pattern
			bestSpecificity = specificity
		}
	}
	if bestPattern == "" {
		return "", false
	}
	return overrides[bestPattern], true
}

func ValidateOverride(pattern, language string) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("language override pattern must not be empty")
	}
	clean := strings.ReplaceAll(pattern, "\\", "/")
	if filepath.IsAbs(filepath.FromSlash(clean)) || strings.HasPrefix(clean, "/") || len(clean) > 1 && clean[1] == ':' {
		return fmt.Errorf("language override pattern must be relative: %q", pattern)
	}
	for _, part := range strings.Split(strings.TrimPrefix(clean, "./"), "/") {
		if part == ".." {
			return fmt.Errorf("language override pattern must stay inside the repository: %q", pattern)
		}
		if part == "" {
			continue
		}
		if _, err := path.Match(part, ""); err != nil {
			return fmt.Errorf("invalid language override pattern %q: %w", pattern, err)
		}
	}
	if !IsKnown(language) {
		return fmt.Errorf("unknown language override %q", language)
	}
	return nil
}

func globMatch(pattern, value string) bool {
	patternParts := splitPath(pattern)
	valueParts := splitPath(value)
	var match func(int, int) bool
	match = func(patternAt, valueAt int) bool {
		if patternAt == len(patternParts) {
			return valueAt == len(valueParts)
		}
		if patternParts[patternAt] == "**" {
			return match(patternAt+1, valueAt) || valueAt < len(valueParts) && match(patternAt, valueAt+1)
		}
		return valueAt < len(valueParts) && segmentMatch(patternParts[patternAt], valueParts[valueAt]) && match(patternAt+1, valueAt+1)
	}
	return match(0, 0)
}

func segmentMatch(pattern, value string) bool {
	ok, err := path.Match(pattern, value)
	return err == nil && ok
}

func splitPath(value string) []string {
	value = strings.Trim(strings.ReplaceAll(filepath.ToSlash(value), "\\", "/"), "/")
	value = strings.TrimPrefix(value, "./")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func globSpecificity(pattern string) int {
	score := 0
	for _, char := range pattern {
		switch char {
		case '*', '?', '[', ']':
		default:
			score++
		}
	}
	return score
}
