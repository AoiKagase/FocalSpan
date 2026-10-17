package python

import "testing"

func TestAuditPythonClassTestClassification(t *testing.T) {
	e := auditPython(t, "class TestSuite:\n    def test_target(self):\n        pass\n")
	for _, s := range e.Symbols {
		if s.Name == "test_target" {
			if s.Kind != "test" {
				t.Fatalf("collected class test lost test classification: %+v", s)
			}
			return
		}
	}
	t.Fatal("test missing")
}
