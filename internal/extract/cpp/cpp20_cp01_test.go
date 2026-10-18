package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP01(t *testing.T) {
	t.Run("C01_digit_separator", func(t *testing.T) {
		x := cppAuditExtract(t, "constexpr int count = 1'000;\nvoid real() {}\n")
		cppAuditSymbol(t, x, "real")
	})
}
