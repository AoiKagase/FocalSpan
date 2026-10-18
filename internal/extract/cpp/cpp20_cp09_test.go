package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP09(t *testing.T) {
	t.Run("C10_receiver_method", func(t *testing.T) {
		x := cppAuditExtract(t, "void target() {}\nnamespace n { struct Object { void target(); }; }\nvoid caller(n::Object &object) { object.target(); }\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
}
