package benchmark

import "testing"

func TestWorkspaceIDRejectsUnsafeComponents(t *testing.T) {
	for _, id := range []string{".", "..", "../outside", "a/b", `a\b`, "C:drive", "CON", "con.txt", "LPT1", "COM9.log", "trailing.", "trailing "} {
		if _, err := safeWorkspaceID(id); err == nil {
			t.Errorf("accepted unsafe ID %q", id)
		}
	}
	for _, id := range []string{"repo.name", "case-1", "a_b", "-prefixed"} {
		if _, err := safeWorkspaceID(id); err != nil {
			t.Errorf("valid ID %q: %v", id, err)
		}
	}
}
func TestSuiteRejectsTraversalIDs(t *testing.T) {
	for _, field := range []string{"case", "repository"} {
		c := Case{ID: "safe", Repository: "repo", BaseRef: "base", TargetRef: "head", Query: "query", Budgets: []int{1000}}
		if field == "case" {
			c.ID = "../../outside"
		} else {
			c.Repository = ".."
		}
		if err := ValidateSuite(Suite{Schema: SuiteSchemaV1, Name: "unsafe", Cases: []Case{c}}); err == nil {
			t.Errorf("accepted unsafe %s ID", field)
		}
	}
}
