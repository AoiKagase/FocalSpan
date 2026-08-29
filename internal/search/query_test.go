package search

import "testing"

func TestNormalizeQueryPreservesCodeTermsAndSplitsIdentifiers(t *testing.T) {
	terms := NormalizeQuery(`Where does AuthService.ValidateToken reject expired_token in auth/service.go?`)
	if !contains(terms.Words, "auth") || !contains(terms.Words, "service") || !contains(terms.Words, "validate") || !contains(terms.Words, "token") || !contains(terms.Words, "expired") {
		t.Fatalf("words=%v", terms.Words)
	}
	if !contains(terms.Identifiers, "ValidateToken") || !contains(terms.Paths, "auth/service.go") {
		t.Fatalf("identifiers=%v paths=%v", terms.Identifiers, terms.Paths)
	}
}

func TestBuildFTSQueryQuotesSpecialCharacters(t *testing.T) {
	query := BuildFTSQuery(`name:"expired" OR foo/bar (unsafe)`)
	if query == "" || query == `name:"expired" OR foo/bar (unsafe)` {
		t.Fatalf("query was not sanitized: %q", query)
	}
}

func TestQueryRelationsDoesNotTreatTypeScriptAsTypeRelation(t *testing.T) {
	terms := NormalizeQuery("where is an expired TypeScript token rejected?")
	if got := queryRelations(terms); len(got) != 0 {
		t.Fatalf("relations=%v, want none", got)
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
