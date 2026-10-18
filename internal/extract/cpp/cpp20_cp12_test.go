package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP12(t *testing.T) {
	t.Run("C13_spliced_line_comment", func(t *testing.T) {
		x := cppAuditExtract(t, "// note \\\nvoid phantom() {}\nvoid real() {}\n")
		cppAuditSymbol(t, x, "real")
		for _, s := range x.Symbols {
			if s.Name == "phantom" {
				t.Error("continued comment created a declaration")
			}
		}
	})
}
