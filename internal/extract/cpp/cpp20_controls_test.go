package cpp_test

import (
	"testing"
)

func TestCPP20_Controls(t *testing.T) {
	t.Run("C07_nested_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Outer { struct Inner { void run() {} }; };\n")
		cppAuditSymbol(t, x, "Outer::Inner")
		cppAuditSymbol(t, x, "Outer::Inner::run")
	})
	t.Run("control_assignment_operator", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n { struct Item { Item &operator=(const Item &other) { return *this; } }; }\n")
		s := cppAuditSymbol(t, x, "n::Item::operator=")
		if s.Kind != "operator" {
			t.Error(s)
		}
	})
	t.Run("control_unicode_crlf", func(t *testing.T) {
		x := cppAuditExtract(t, "void 操作() {}\r\nvoid caller() { 操作(); }\r\n")
		cppAuditSymbol(t, x, "操作")
		cppAuditSymbol(t, x, "caller")
	})
	t.Run("control_primitive_function", func(t *testing.T) {
		x := cppAuditExtract(t, "int add(int x) { return x+1; }\n")
		cppAuditSymbol(t, x, "add")
	})
	t.Run("control_raw_string", func(t *testing.T) {
		x := cppAuditExtract(t, "auto text = u8R\"tag(void phantom(){})tag\";\nvoid real() {}\n")
		cppAuditSymbol(t, x, "real")
		for _, s := range x.Symbols {
			if s.Name == "phantom" {
				t.Error("raw string declaration")
			}
		}
	})
}
