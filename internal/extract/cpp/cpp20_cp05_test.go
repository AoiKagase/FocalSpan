package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP05(t *testing.T) {
	t.Run("C05_namespace_alias", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace original {}\nnamespace alias = original;\nvoid later() {}\n")
		cppAuditSymbol(t, x, "later")
		for _, s := range x.Symbols {
			if s.Name == "alias" && s.Kind == "namespace" {
				t.Error("alias incorrectly treated as namespace body")
			}
		}
	})
	t.Run("C05_using_namespace", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace original {}\nusing namespace original;\nvoid later() {}\n")
		cppAuditSymbol(t, x, "later")
	})
}
