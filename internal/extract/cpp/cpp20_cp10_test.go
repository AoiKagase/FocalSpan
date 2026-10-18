package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP10(t *testing.T) {
	t.Run("C11_external_qualified_call", func(t *testing.T) {
		x := cppAuditExtract(t, "#include \"external.hpp\"\nnamespace other { void work() {} }\nvoid caller() { external::work(); }\n")
		cppAuditNoCall(t, x, "caller", "other::work")
	})
}
