package python

import "testing"

func TestPythonMaskRespectsMultilineStringBoundaries(t *testing.T) {
	for _, prefix := range []string{"", "r", "b", "f"} {
		t.Run("continued-"+prefix, func(t *testing.T) {
			source := "text = " + prefix + "\"hello \\\nworld\"\ndef real():\n    return 1\n"
			e := auditPython(t, source)
			for _, s := range e.Symbols {
				if s.Name == "real" {
					return
				}
			}
			t.Fatalf("real declaration masked: %+v", e.Symbols)
		})
	}
	for _, prefix := range []string{"", "r", "b", "f"} {
		t.Run("escaped-triple-"+prefix, func(t *testing.T) {
			source := "doc = " + prefix + "\"\"\"escaped \\\"\"\"\ndef fake():\n    pass\n\"\"\"\ndef real():\n    return 1\n"
			e := auditPython(t, source)
			found := false
			for _, s := range e.Symbols {
				if s.Name == "fake" {
					t.Errorf("string declaration escaped mask: %+v", s)
				}
				if s.Name == "real" {
					found = true
				}
			}
			if !found {
				t.Fatalf("real declaration masked: %+v", e.Symbols)
			}
		})
	}
}

func TestPythonMaskResumesAfterContinuedString(t *testing.T) {
	e := auditPython(t, "def Target():\n    pass\ndef Caller():\n    text = \"hello \\\nworld\" # Target()\n    return text\n")
	for _, r := range e.Relations {
		if r.Kind == "calls" {
			t.Fatalf("comment after continued string produced call: %+v", r)
		}
	}
}

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
