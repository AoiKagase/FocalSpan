package cpp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cppAuditExtract(t *testing.T, source string) model.Extraction {
	t.Helper()
	x, err := cpp.NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/main.cpp", Language: "cpp", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	if root := os.Getenv("CPP_AUDIT_FIXTURE_DIR"); root != "" {
		name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(t.Name())
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("%s-%x.cpp", name, sha256.Sum256([]byte(source)))), []byte(source), 0600); e != nil {
			t.Fatal(e)
		}
	}
	b, _ := json.Marshal(x)
	t.Log("EXTRACTION " + string(b))
	for _, c := range x.Chunks {
		if c.EndByte > c.StartByte && c.Content != source[c.StartByte:c.EndByte] {
			t.Errorf("source fidelity violated: %+v", c)
		}
	}
	return x
}
func cppAuditSymbol(t *testing.T, x model.Extraction, name string) model.Symbol {
	t.Helper()
	for _, s := range x.Symbols {
		if s.QualifiedName == name {
			return s
		}
	}
	t.Errorf("symbol %s missing", name)
	return model.Symbol{}
}
func cppAuditNoCall(t *testing.T, x model.Extraction, from, to string) {
	t.Helper()
	f := cppAuditSymbol(t, x, from)
	target := cppAuditSymbol(t, x, to)
	for _, r := range x.Relations {
		if r.FromHandle == f.Handle && r.ToHandle == target.Handle && r.Kind == "calls" {
			t.Errorf("false definite call: %+v", r)
		}
	}
}
func TestCppFocusedAudit(t *testing.T) {
	t.Run("C01_digit_separator", func(t *testing.T) {
		x := cppAuditExtract(t, "constexpr int count = 1'000;\nvoid real() {}\n")
		cppAuditSymbol(t, x, "real")
	})
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
	t.Run("C03_custom_return_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Item {};\nItem make() { return {}; }\n")
		cppAuditSymbol(t, x, "make")
	})
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
	t.Run("C05_namespace_alias", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace original {}\nnamespace alias = original;\nvoid later() {}\n")
		cppAuditSymbol(t, x, "later")
		for _, s := range x.Symbols {
			if s.Name == "alias" && s.Kind == "namespace" {
				t.Error("alias incorrectly treated as namespace body")
			}
		}
	})
	t.Run("C06_anonymous_namespace", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace { void hidden() {} }\nvoid visible() {}\n")
		h := cppAuditSymbol(t, x, "hidden")
		for _, p := range x.Symbols {
			if p.Handle == h.ParentHandle && p.Kind != "namespace" {
				t.Error("anonymous-namespace function attached to", p.Kind)
			}
		}
		if h.ParentHandle == "" {
			t.Error("anonymous namespace ownership lost")
		}
	})
	t.Run("C07_nested_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Outer { struct Inner { void run() {} }; };\n")
		cppAuditSymbol(t, x, "Outer::Inner")
		cppAuditSymbol(t, x, "Outer::Inner::run")
	})
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
	t.Run("C09_parameter_shadow", func(t *testing.T) {
		x := cppAuditExtract(t, "void target() {}\nvoid caller(void (*target)()) { target(); }\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("C10_receiver_method", func(t *testing.T) {
		x := cppAuditExtract(t, "void target() {}\nnamespace n { struct Object { void target(); }; }\nvoid caller(n::Object &object) { object.target(); }\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("C09_callback_argument_shadow", func(t *testing.T) {
		x := cppAuditExtract(t, "using Callback = void (*)(int);\nvoid handler(int x) {}\nvoid register_callback(Callback cb) {}\nvoid install(Callback handler) { register_callback(handler); }\n")
		from := cppAuditSymbol(t, x, "install")
		target := cppAuditSymbol(t, x, "handler")
		for _, r := range x.Relations {
			if r.FromHandle == from.Handle && r.ToHandle == target.Handle && r.Kind == "references" {
				t.Errorf("dynamic callback bound to global handler: %+v", r)
			}
		}
	})
	t.Run("C11_external_qualified_call", func(t *testing.T) {
		x := cppAuditExtract(t, "#include \"external.hpp\"\nnamespace other { void work() {} }\nvoid caller() { external::work(); }\n")
		cppAuditNoCall(t, x, "caller", "other::work")
	})
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
	t.Run("C13_spliced_line_comment", func(t *testing.T) {
		x := cppAuditExtract(t, "// note \\\nvoid phantom() {}\nvoid real() {}\n")
		cppAuditSymbol(t, x, "real")
		for _, s := range x.Symbols {
			if s.Name == "phantom" {
				t.Error("continued comment created a declaration")
			}
		}
	})
	t.Run("C14_commented_include_prefix", func(t *testing.T) {
		for _, source := range []string{"/* note */ #include \"dep.hpp\"\nvoid real() {}\n", "#include /* note */ \"dep.hpp\"\nvoid real() {}\n"} {
			x := cppAuditExtract(t, source)
			found := false
			for _, r := range x.Relations {
				if r.Kind == "imports" {
					found = true
				}
			}
			if !found {
				t.Error("valid include with comments omitted")
			}
		}
	})
	t.Run("C15_template_type_parameter", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n { template<class T> struct Box { void run() {} }; }\n")
		cppAuditSymbol(t, x, "n::Box")
		cppAuditSymbol(t, x, "n::Box::run")
		for _, s := range x.Symbols {
			if s.Name == "T" && s.Kind == "class" {
				t.Error("type parameter swallowed class declaration")
			}
		}
	})
	t.Run("C16_global_class_qualification", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Item { void run() {} };\n")
		cppAuditSymbol(t, x, "Item::run")
		control := cppAuditExtract(t, "namespace n { struct Item { void run() {} }; }\n")
		cppAuditSymbol(t, control, "n::Item::run")
	})
	t.Run("C17_module_target", func(t *testing.T) {
		x := cppAuditExtract(t, "export module tools;\nimport utilities;\nvoid real() {}\n")
		found := false
		for _, r := range x.Relations {
			if r.Kind == "imports" && r.Source == "cpp:module-import" && r.UnresolvedTo == "utilities" {
				found = true
			}
		}
		if !found {
			t.Error("module import target is not normalized to utilities")
		}
	})
	t.Run("C19_template_call", func(t *testing.T) {
		x := cppAuditExtract(t, "template<class T> void work() {}\nvoid caller() { work<int>(); }\n")
		f := cppAuditSymbol(t, x, "caller")
		target := cppAuditSymbol(t, x, "work")
		found := false
		for _, r := range x.Relations {
			if r.FromHandle == f.Handle && r.Kind == "calls" && (r.ToHandle == target.Handle || r.UnresolvedTo == "work") {
				found = true
			}
		}
		if !found {
			t.Error("template invocation missing even as an unresolved call")
		}
	})
	t.Run("C20_typedef_custom_type", func(t *testing.T) {
		x := cppAuditExtract(t, "struct Existing {};\ntypedef Existing Alias;\n")
		s := cppAuditSymbol(t, x, "Alias")
		if s.Handle != "" && s.Kind != "alias" {
			t.Error(s)
		}
	})
	t.Run("C05_using_namespace", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace original {}\nusing namespace original;\nvoid later() {}\n")
		cppAuditSymbol(t, x, "later")
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
	t.Run("control_assignment_operator", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n { struct Item { Item &operator=(const Item &other) { return *this; } }; }\n")
		s := cppAuditSymbol(t, x, "n::Item::operator=")
		if s.Kind != "operator" {
			t.Error(s)
		}
	})
	t.Run("C22_utf8_bom", func(t *testing.T) {
		x := cppAuditExtract(t, "\ufeffvoid first() {}\r\nvoid second() {}\r\n")
		cppAuditSymbol(t, x, "first")
		cppAuditSymbol(t, x, "second")
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
