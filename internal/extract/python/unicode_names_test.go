package python

import "testing"

func TestPythonUnicodeCallRelation(t *testing.T) {
	e := auditPython(t, "def 認証():\n    return True\ndef 呼出し():\n    return 認証()\n")
	var target string
	for _, s := range e.Symbols {
		if s.Name == "認証" {
			target = s.Handle
		}
	}
	for _, r := range e.Relations {
		if r.Kind == "calls" && target != "" && r.ToHandle == target {
			return
		}
	}
	t.Fatalf("Unicode call missing: %+v", e.Relations)
}

func TestAuditPythonUnicodeName(t *testing.T) {
	e := auditPython(t, "def 認証():\n    return True\n")
	for _, s := range e.Symbols {
		if s.Name == "認証" {
			return
		}
	}
	t.Fatalf("valid Unicode identifier missing: %+v", e.Symbols)
}
