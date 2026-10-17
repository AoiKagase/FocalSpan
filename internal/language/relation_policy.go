package language

import "strings"

// SymbolNamesEqual is for semantic relations, not fuzzy search retrieval.
func SymbolNamesEqual(language, kind, left, right string) bool {
	switch language {
	case "vb6", "vbnet", "nim":
		return strings.EqualFold(left, right)
	case "php":
		switch kind {
		case "function", "method", "class", "interface", "trait", "enum", "namespace":
			return strings.EqualFold(left, right)
		}
	}
	return left == right
}

func RelationLanguagesCompatible(from, to, relation string) bool {
	if from == to {
		return true
	}
	if relation == "imports" || relation == "exports" || relation == "references" {
		return true
	}
	return (from == "c" || from == "cpp") && (to == "c" || to == "cpp") || (from == "javascript" || from == "typescript") && (to == "javascript" || to == "typescript")
}
