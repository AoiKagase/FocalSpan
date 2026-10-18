package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP16(t *testing.T) {
	t.Run("C17_module_target", func(t *testing.T) {
		x := cppAuditExtract(t, "export module tools;\nimport utilities;\nvoid real() {}\n")
		found := false
		for _, r := range x.Relations {
			if r.Kind == "imports" && r.Source == "cpp:module-import" && r.UnresolvedTo == "utilities" {
				found = true
			}
		}
		if !found {
			t.Error("module import target is not normalized to utilities")
		}
	})
}
