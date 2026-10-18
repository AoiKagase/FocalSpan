package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP15(t *testing.T) {
	t.Run("C16_global_class_qualification", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Item { void run() {} };\n")
		cppAuditSymbol(t, x, "Item::run")
		control := cppAuditExtract(t, "namespace n { struct Item { void run() {} }; }\n")
		cppAuditSymbol(t, control, "n::Item::run")
	})
}
