package cpp_test

import (
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestCPP20_CallableEdges(t *testing.T) {
	t.Run("CP06_ImplicitUsing", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace{void hidden(){}} void caller(){hidden();}\n")
		if !cpp20HasCall(x, "caller", "(anonymous@src/main.cpp)::hidden") {
			t.Fatal("same-file anonymous member not found")
		}
	})
	t.Run("CP08_RvalueParameter", func(t *testing.T) {
		x := cppAuditExtract(t, "using Callback=void(*)();void target(){} void caller(Callback&& target){target();}\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	for _, test := range []struct{ name, src string }{
		{"FunctionPointer", "void f(void (*cb)(int value)); void f(void (*renamed)(int different)){}"},
		{"NestedTemplate", "namespace n{template<class T>struct Box{};}void f(n::Box<n::Box<int>> value);void f(n::Box<n::Box<int>> renamed){}"},
		{"DefaultArgument", "void f(int value=1);void f(int renamed){}"},
	} {
		t.Run("CP11_"+test.name, func(t *testing.T) {
			x := cppAuditExtract(t, test.src)
			ok := false
			for _, r := range x.Relations {
				if r.Kind == "declares" && r.ToHandle != "" {
					ok = true
				}
			}
			if !ok {
				t.Fatal("declaration/definition not paired")
			}
		})
	}
	_ = model.Symbol{}
}
