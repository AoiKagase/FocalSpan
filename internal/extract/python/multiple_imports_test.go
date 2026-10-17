package python

import "testing"

func TestPythonImportTargetsPreserveOriginalNames(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   []string
	}{
		{"import first as a, second as b\n", []string{"first", "second"}},
		{"from ..pkg import (\n    First,\n    Second as Alias,\n)\n", []string{"..pkg.First", "..pkg.Second"}},
		{"from pkg import *\n", []string{"pkg"}},
	} {
		e := auditPython(t, tc.source)
		found := map[string]bool{}
		for _, r := range e.Relations {
			if r.Kind == "imports" {
				found[r.UnresolvedTo] = true
			}
		}
		for _, want := range tc.want {
			if !found[want] {
				t.Errorf("%q missing %s: %+v", tc.source, want, e.Relations)
			}
		}
	}
}

func TestAuditPythonMultipleImports(t *testing.T) {
	e := auditPython(t, "import first, second\n")
	found := map[string]bool{}
	for _, r := range e.Relations {
		if r.Kind == "imports" {
			found[r.UnresolvedTo] = true
		}
	}
	if !found["first"] || !found["second"] {
		t.Fatalf("multiple import targets not preserved: %+v", e.Relations)
	}
}
