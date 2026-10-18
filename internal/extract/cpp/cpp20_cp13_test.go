package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP13(t *testing.T) {
	t.Run("C14_commented_include_prefix", func(t *testing.T) {
		for _, source := range []string{"/* note */ #include \"dep.hpp\"\nvoid real() {}\n", "#include /* note */ \"dep.hpp\"\nvoid real() {}\n"} {
			x := cppAuditExtract(t, source)
			found := false
			for _, r := range x.Relations {
				if r.Kind == "imports" {
					found = true
				}
			}
			if !found {
				t.Error("valid include with comments omitted")
			}
		}
	})
}
