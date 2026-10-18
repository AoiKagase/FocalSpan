package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP17(t *testing.T) {
	t.Run("C19_template_call", func(t *testing.T) {
		x := cppAuditExtract(t, "template<class T> void work() {}\nvoid caller() { work<int>(); }\n")
		f := cppAuditSymbol(t, x, "caller")
		target := cppAuditSymbol(t, x, "work")
		found := false
		for _, r := range x.Relations {
			if r.FromHandle == f.Handle && r.Kind == "calls" && (r.ToHandle == target.Handle || r.UnresolvedTo == "work") {
				found = true
			}
		}
		if !found {
			t.Error("template invocation missing even as an unresolved call")
		}
	})
}
