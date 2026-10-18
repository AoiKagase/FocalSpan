package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP06(t *testing.T) {
	t.Run("C06_anonymous_namespace", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace { void hidden() {} }\nvoid visible() {}\n")
		h := cppAuditSymbol(t, x, "(anonymous@src/main.cpp)::hidden")
		for _, p := range x.Symbols {
			if p.Handle == h.ParentHandle && p.Kind != "namespace" {
				t.Error("anonymous-namespace function attached to", p.Kind)
			}
		}
		if h.ParentHandle == "" {
			t.Error("anonymous namespace ownership lost")
		}
	})
}
