package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP18(t *testing.T) {
	t.Run("C20_typedef_custom_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Existing {};\ntypedef Existing Alias;\n")
		s := cppAuditSymbol(t, x, "Alias")
		if s.Handle != "" && s.Kind != "alias" {
			t.Error(s)
		}
	})
}
