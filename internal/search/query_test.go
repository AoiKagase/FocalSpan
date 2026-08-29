package search

import (
	"testing"

	"github.com/focalspan/focalspan/internal/query"
)

func TestNormalizeQueryPreservesCodeTermsAndSplitsIdentifiers(t *testing.T) {
	terms := query.Normalize(`Where does AuthService.ValidateToken reject expired_token in auth/service.go?`)
	if !contains(terms.Words, "auth") || !contains(terms.Words, "service") || !contains(terms.Words, "validate") || !contains(terms.Words, "token") || !contains(terms.Words, "expired") {
		t.Fatalf("words=%v", terms.Words)
	}
	if !contains(terms.Identifiers, "ValidateToken") || !contains(terms.Paths, "auth/service.go") {
		t.Fatalf("identifiers=%v paths=%v", terms.Identifiers, terms.Paths)
	}
}

func TestBuildFTSQueryQuotesSpecialCharacters(t *testing.T) {
	ftsQuery := query.BuildFTS(query.Normalize(`name:"expired" OR foo/bar (unsafe)`))
	if ftsQuery == "" || ftsQuery == `name:"expired" OR foo/bar (unsafe)` {
		t.Fatalf("query was not sanitized: %q", ftsQuery)
	}
}

func TestQueryPlannerDoesNotTreatTypeScriptAsTypeRelation(t *testing.T) {
	plan := query.PlanQuery("where is an expired TypeScript token rejected?")
	if len(plan.Relations) != 0 {
		t.Fatalf("relations=%v, want none", plan.Relations)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
