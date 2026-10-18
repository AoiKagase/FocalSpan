package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP20(t *testing.T) {
	t.Run("C22_utf8_bom", func(t *testing.T) {
		x := cppAuditExtract(t, "\ufeffvoid first() {}\r\nvoid second() {}\r\n")
		cppAuditSymbol(t, x, "first")
		cppAuditSymbol(t, x, "second")
	})
}
