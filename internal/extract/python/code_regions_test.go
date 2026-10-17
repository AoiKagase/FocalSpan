package python

import "testing"

func TestAuditPythonStringIsNotCode(t *testing.T) {
	e := auditPython(t, "text = '''\ndef Phantom():\n    pass\n'''\n")
	for _, s := range e.Symbols {
		if s.Name == "Phantom" {
			t.Fatalf("string content indexed as declaration: %+v", s)
		}
	}
}
func TestAuditPythonCallInsideCommentOrString(t *testing.T) {
	e := auditPython(t, "def Target():\n    pass\ndef Caller():\n    text = 'Target()'\n    # Target()\n    return text\n")
	for _, r := range e.Relations {
		if r.Kind == "calls" {
			t.Fatalf("non-code text produced call: %+v", r)
		}
	}
}
