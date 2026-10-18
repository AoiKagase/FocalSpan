package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP14(t *testing.T) {
	t.Run("C15_template_type_parameter", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n { template<class T> struct Box { void run() {} }; }\n")
		cppAuditSymbol(t, x, "n::Box")
		cppAuditSymbol(t, x, "n::Box::run")
		for _, s := range x.Symbols {
			if s.Name == "T" && s.Kind == "class" {
				t.Error("type parameter swallowed class declaration")
			}
		}
	})
}
