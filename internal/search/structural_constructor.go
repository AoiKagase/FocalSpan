package search

import (
	"strings"

	"github.com/focalspan/focalspan/internal/query"
)

var structuralConstructorTerms = map[string]bool{
	"registry": true, "assembled": true, "assembly": true, "wired": true,
	"registered": true, "registration": true,
}

func structuralConstructorHints(plan query.Plan, req SearchRequest) []string {
	if len(req.Paths) > 0 || len(plan.Terms.Paths) > 0 {
		return nil
	}
	for _, word := range plan.Terms.Words {
		if structuralConstructorTerms[strings.ToLower(strings.TrimSpace(word))] {
			return []string{"NewWithConfig"}
		}
	}
	lower := strings.ToLower(plan.RawQuery)
	if strings.Contains(lower, "登録") || strings.Contains(lower, "組み立て") {
		return []string{"NewWithConfig"}
	}
	return nil
}
