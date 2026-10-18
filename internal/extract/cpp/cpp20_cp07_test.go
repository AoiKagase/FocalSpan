package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP07(t *testing.T) {
	t.Run("C08_operators", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Item { int operator[](int i) { return i; } int operator+(int i) { return i; } explicit operator bool() const { return true; } int operator()(int i) { return i; } };\n")
		for _, name := range []string{"operator[]", "operator+", "operator bool", "operator()"} {
			found := false
			for _, s := range x.Symbols {
				if s.Name == name && s.Kind == "operator" {
					found = true
				}
			}
			if !found {
				t.Errorf("operator %s missing or misnamed", name)
			}
		}
	})
}
