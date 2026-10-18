package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP02(t *testing.T) {
	t.Run("C02_first_constructor", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n { struct Item { Item() {} void later() {} }; }\n")
		found := false
		for _, s := range x.Symbols {
			if s.Kind == "constructor" {
				found = true
				if !strings.HasPrefix(s.Signature, "Item(") {
					t.Errorf("constructor name missing from source signature: %q", s.Signature)
				}
			}
		}
		if !found {
			t.Error("constructor missing")
		}
	})
}
