package language

import "testing"

func TestRelationLanguageFamilies(t *testing.T) {
	for _, tc := range []struct {
		from, to, kind string
		want           bool
	}{{"go", "python", "calls", false}, {"javascript", "typescript", "calls", true}, {"c", "cpp", "calls", true}, {"go", "go", "calls", true}, {"template", "javascript", "references", true}, {"go", "python", "imports", true}} {
		if got := RelationLanguagesCompatible(tc.from, tc.to, tc.kind); got != tc.want {
			t.Fatalf("%+v got=%t", tc, got)
		}
	}
}
