package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP03(t *testing.T) {
	t.Run("C03_custom_return_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Item {};\nItem make() { return {}; }\n")
		cppAuditSymbol(t, x, "make")
	})
}
