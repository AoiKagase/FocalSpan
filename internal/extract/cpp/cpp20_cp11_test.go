package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP11(t *testing.T) {
	t.Run("C12_parameter_names_in_pair", func(t *testing.T) {
		x := cppAuditExtract(t, "void target(int value);\nvoid target(int renamed) {}\n")
		found := false
		for _, r := range x.Relations {
			if r.Kind == "declares" && r.ToHandle != "" {
				found = true
			}
		}
		if !found {
			t.Error("declaration-definition pair lost solely due to parameter name")
		}
	})
}
