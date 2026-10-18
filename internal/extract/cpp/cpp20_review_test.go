package cpp_test

import (
	"strings"
	"testing"
)

func TestCPP20_ReviewProbe(t *testing.T) {
	t.Run("variable_initializer", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Widget{}; Widget item(42); void later(){}")
		for _, s := range x.Symbols {
			if s.Name == "item" && s.Kind == "function" {
				t.Errorf("variable misclassified: %+v", s)
			}
		}
	})
	for name, source := range map[string]string{
		"absolute":   "void target(){} void caller(int target){::target();}",
		"namespace":  "namespace n{void target(){}} void caller(int target){n::target();}",
		"this":       "struct X{void target(){} void caller(int target){this->target();}};",
		"for_end":    "void target(){} void caller(){for(int target=0;target<1;++target){} target();}",
		"catch_end":  "void target(){} void caller(){try{}catch(int target){} target();}",
		"lambda_end": "void target(){} void caller(){auto f=[](int target){target();}; target();}",
	} {
		t.Run(name, func(t *testing.T) {
			x := cppAuditExtract(t, source)
			from, to := "caller", "target"
			if name == "namespace" {
				to = "n::target"
			}
			if name == "this" {
				from = "X::caller"
				to = "X::target"
			}
			if !cpp20HasCall(x, from, to) {
				t.Errorf("expected call missing %s -> %s; relations=%+v", from, to, x.Relations)
			}
		})
	}
}
func TestCPP20_ReviewBoundaryProbe(t *testing.T) {
	t.Run("requires_conjunction", func(t *testing.T) {
		source := "template<class T> void f(T x) requires C<T> && requires(T y){y.work();} { x.run(); } void later(){}"
		x := cppAuditExtract(t, source)
		s := cppAuditSymbol(t, x, "f")
		if s.EndByte <= 0 || !strings.Contains(source[s.StartByte:s.EndByte], "x.run();") {
			t.Errorf("truncated function: %+v", s)
		}
	})
	for name, source := range map[string]string{
		"mutable_lambda": "void target(){} void caller(){auto f=[](auto target) mutable {target();};}",
		"if_init":        "void target(){} void caller(){if(auto target=callback; target){target();}}",
	} {
		t.Run(name, func(t *testing.T) { x := cppAuditExtract(t, source); cppAuditNoCall(t, x, "caller", "target") })
	}
}
func TestCPP20_ReviewInitializerMatrix(t *testing.T) {
	for _, value := range []string{"42", "'x'", "\"text\"", "Widget{1}", "callback+1"} {
		x := cppAuditExtract(t, "struct Widget{Widget(int);}; Widget item("+value+"); void later(){}\n")
		cppAuditSymbol(t, x, "later")
		for _, s := range x.Symbols {
			if s.Name == "item" && s.Kind == "function" {
				t.Fatal("initializer classified as function", value)
			}
		}
	}
}
func TestCPP20_ReviewQualifiedCallback(t *testing.T) {
	x := cppAuditExtract(t, "using Callback=void(*)();namespace n{void handler(){}}void register_callback(Callback);void caller(Callback handler){register_callback(n::handler);}\n")
	var from, to string
	for _, s := range x.Symbols {
		if s.Name == "caller" {
			from = s.Handle
		}
		if s.QualifiedName == "n::handler" {
			to = s.Handle
		}
	}
	found := false
	for _, r := range x.Relations {
		if r.FromHandle == from && r.ToHandle == to && r.Kind == "references" {
			found = true
		}
	}
	if !found {
		t.Fatal("explicit callback incorrectly shadowed")
	}
}

func TestCPP20_ReviewComplexReceiver(t *testing.T) {
	for _, expression := range []string{"objects[0].target()", "make().target()"} {
		x := cppAuditExtract(t, "void target(){} void caller(){"+expression+";} ")
		cppAuditNoCall(t, x, "caller", "target")
	}
}

func TestCPP20_ReviewNestedIfScope(t *testing.T) {
	x := cppAuditExtract(t, "void target(){} void caller(){if(auto target=callback; target) if(condition){target();}else{target();} target();}")
	if !cpp20HasCall(x, "caller", "target") {
		t.Fatal("if initializer binding escaped the complete nested statement")
	}
}
