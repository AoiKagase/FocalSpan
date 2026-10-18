package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

var _ = strings.Contains
var _ = model.Symbol{}

func TestCPP20_CP04(t *testing.T) {
	t.Run("C04_braced_initializer_span", func(t *testing.T) {
		source := "void consume() {}\nstruct Item { int value; explicit Item(int x) : value{x} { consume(); } };\n"
		x := cppAuditExtract(t, source)
		s := model.Symbol{}
		for _, v := range x.Symbols {
			if v.Name == "Item" && v.Kind == "constructor" {
				s = v
			}
		}
		found := false
		for _, c := range x.Chunks {
			if c.SymbolHandle == s.Handle && strings.Contains(c.Content, "consume();") {
				found = true
			}
		}
		if !found {
			t.Error("constructor body omitted from its chunk")
		}
	})
	t.Run("C04_requires_expression_body", func(t *testing.T) {
		source := "template<class T> int use(T x) requires requires(T y) { y.work(); } { return x.work(); }\n"
		x := cppAuditExtract(t, source)
		s := cppAuditSymbol(t, x, "use")
		found := false
		for _, c := range x.Chunks {
			if c.SymbolHandle == s.Handle && strings.Contains(c.Content, "return x.work();") {
				found = true
			}
		}
		if !found {
			t.Error("requires-expression mistaken for actual function body")
		}
	})
	t.Run("C04_function_try_block", func(t *testing.T) {
		x := cppAuditExtract(t, "void work() try { throw 1; } catch (int) { recover(); }\n")
		s := cppAuditSymbol(t, x, "work")
		found := false
		for _, c := range x.Chunks {
			if c.SymbolHandle == s.Handle && strings.Contains(c.Content, "recover();") {
				found = true
			}
		}
		if !found {
			t.Error("function-try-block handler omitted from source")
		}
	})
}
