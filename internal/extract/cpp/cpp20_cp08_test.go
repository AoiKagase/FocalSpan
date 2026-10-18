package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP08(t *testing.T) {
	t.Run("C09_parameter_shadow", func(t *testing.T) {
		x := cppAuditExtract(t, "void target() {}\nvoid caller(void (*target)()) { target(); }\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("C09_callback_argument_shadow", func(t *testing.T) {
		x := cppAuditExtract(t, "using Callback = void (*)(int);\nvoid handler(int x) {}\nvoid register_callback(Callback cb) {}\nvoid install(Callback handler) { register_callback(handler); }\n")
		from := cppAuditSymbol(t, x, "install")
		target := cppAuditSymbol(t, x, "handler")
		for _, r := range x.Relations {
			if r.FromHandle == from.Handle && r.ToHandle == target.Handle && r.Kind == "references" {
				t.Errorf("dynamic callback bound to global handler: %+v", r)
			}
		}
	})
}
