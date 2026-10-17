package python

import "testing"

func TestAuditPythonClosedParentScope(t *testing.T) {
	e := auditPython(t, "class Closed:\n    pass\nif True:\n    def helper():\n        pass\n")
	for _, s := range e.Symbols {
		if s.Name == "helper" {
			if s.ParentHandle != "" || s.Kind != "function" {
				t.Fatalf("function outside class assigned closed parent: %+v", s)
			}
			return
		}
	}
	t.Fatal("helper missing")
}
