package cpp_test

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

func cpp20HasCall(x model.Extraction, from, to string) bool {
	var a, b string
	for _, s := range x.Symbols {
		if s.QualifiedName == from {
			a = s.Handle
		}
		if s.QualifiedName == to {
			b = s.Handle
		}
	}
	for _, r := range x.Relations {
		if a != "" && b != "" && r.FromHandle == a && r.ToHandle == b && r.Kind == "calls" {
			return true
		}
	}
	return false
}
func TestCPP20_Extended(t *testing.T) {
	t.Run("CP01_SourceOffsets", func(t *testing.T) {
		for _, n := range []string{"1'000", "0xAB'CD", "0b10'01", "1.2e+3", "0x1p-2"} {
			source := "constexpr auto n=" + n + "; char c='x';\r\nvoid real(){}\r\n"
			x := cppAuditExtract(t, source)
			s := cppAuditSymbol(t, x, "real")
			if s.StartByte != strings.Index(source, "void real") || s.StartLine != 2 {
				t.Fatal(s)
			}
		}
	})
	t.Run("CP20_SourceOffsets", func(t *testing.T) {
		s := cppAuditSymbol(t, cppAuditExtract(t, "\ufeffvoid 操作(){}\r\nvoid second(){}\r\n"), "操作")
		if s.StartByte != 3 || s.StartLine != 1 {
			t.Fatal(s)
		}
	})
	t.Run("CP12_SourceOffsets", func(t *testing.T) {
		x := cppAuditExtract(t, "// note \\\r\nvoid phantom(){}\r\nvoid real(){}\r\n")
		s := cppAuditSymbol(t, x, "real")
		if s.StartLine != 3 {
			t.Fatal(s)
		}
		for _, v := range x.Symbols {
			if v.Name == "phantom" {
				t.Fatal(v)
			}
		}
	})
	t.Run("CP13_SourceOffsets", func(t *testing.T) {
		x := cppAuditExtract(t, "/*before\r\n*/ #include /* inside */ \\\r\n\"dep.hpp\"\r\nauto url=R\"(https://example.invalid/*value*/)\";\r\nvoid real(){}\r\n")
		cppAuditSymbol(t, x, "real")
		ok := false
		for _, r := range x.Relations {
			if r.Kind == "imports" && r.UnresolvedTo == "dep.hpp" {
				ok = true
			}
		}
		if !ok {
			t.Fatal("include omitted")
		}
	})
	t.Run("CP06_ReopenOwnership", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace {void first(){}} namespace {void second(){}}\n")
		a := cppAuditSymbol(t, x, "(anonymous@src/main.cpp)::first")
		b := cppAuditSymbol(t, x, "(anonymous@src/main.cpp)::second")
		if a.ParentHandle == "" || a.ParentHandle != b.ParentHandle {
			t.Fatalf("reopened owner: %s %s", a.ParentHandle, b.ParentHandle)
		}
		other, e := cpp.NewExtractor().Extract(context.Background(), model.SourceFile{Path: "other.cpp", Language: "cpp", Content: []byte("namespace{void first(){}}")})
		if e != nil {
			t.Fatal(e)
		}
		for _, s := range other.Symbols {
			if s.Name == "first" && s.QualifiedName == a.QualifiedName {
				t.Fatal("anonymous scopes collide")
			}
		}
	})
	t.Run("CP14_DeclarationBoundary", func(t *testing.T) {
		for _, h := range []string{"template<typename T>", "template<class T, int N>", "template<template<class> class C, class T>"} {
			x := cppAuditExtract(t, "namespace n{"+h+" struct Box{void run(){}};}")
			cppAuditSymbol(t, x, "n::Box")
			cppAuditSymbol(t, x, "n::Box::run")
		}
	})
	t.Run("CP03_DeclarationBoundary", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n{struct Item{}; template<class T>struct Box{}; Item* ptr(){return 0;} const Item& ref(){throw 1;} Box<Item> boxed(){return{};} }\n")
		for _, s := range []string{"n::ptr", "n::ref", "n::boxed"} {
			cppAuditSymbol(t, x, s)
		}
	})
	t.Run("CP04_DeclarationBoundary", func(t *testing.T) {
		source := "struct X{int a,b; X() try:a{1},b(2){first();}catch(int){second();}catch(...){third();} void later(){}};"
		x := cppAuditExtract(t, source)
		s := cppAuditSymbol(t, x, "X::X")
		if !strings.HasSuffix(source[s.StartByte:s.EndByte], "third();}") {
			t.Fatal("handler omitted", source[s.StartByte:s.EndByte])
		}
		cppAuditSymbol(t, x, "X::later")
	})
	t.Run("CP07_DeclarationBoundary", func(t *testing.T) {
		x := cppAuditExtract(t, "struct X {void* operator new(unsigned long); void operator delete(void*); int operator[](int)const; explicit operator bool()const;};")
		for _, n := range []string{"operator new", "operator delete", "operator[]", "operator bool"} {
			ok := false
			for _, s := range x.Symbols {
				if s.Name == n && s.Kind == "operator" {
					ok = true
				}
			}
			if !ok {
				t.Fatal(n)
			}
		}
	})
	t.Run("CP18_MultipleDeclarators", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n{struct Existing{};} typedef n::Existing Alias, *Pointer; typedef int Array[3]; typedef void (*Callback)(int named); using Other=int;\n")
		for _, name := range []string{"Alias", "Pointer", "Array", "Callback", "Other"} {
			s := cppAuditSymbol(t, x, name)
			if s.Kind != "alias" {
				t.Fatal(s)
			}
		}
	})
	t.Run("CP11_QualifiedTypes", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n{struct Item{};} void f(n::Item); void f(n::Item renamed){}\n")
		ok := false
		for _, r := range x.Relations {
			if r.Kind == "declares" && r.ToHandle != "" {
				ok = true
			}
		}
		if !ok {
			t.Fatal("qualified type pair omitted")
		}
	})
	t.Run("CP11_OverloadSeparation", func(t *testing.T) {
		x := cppAuditExtract(t, "void f(int named); void f(double named){}\nstruct X{void g() const;void g() &{}};\n")
		for _, r := range x.Relations {
			if r.Kind == "declares" && r.ToHandle != "" {
				t.Fatal("different types/qualifiers paired", r)
			}
		}
	})
	t.Run("CP08_BlockScope", func(t *testing.T) {
		x := cppAuditExtract(t, "void target(){} void caller(){ {auto target=[](){}; target();} target();}\n")
		if !cpp20HasCall(x, "caller", "target") {
			t.Fatal("binding escaped its block")
		}
	})
	t.Run("CP08_ForScope", func(t *testing.T) {
		x := cppAuditExtract(t, "void target(){} void caller(){for(int target=0;target<1;++target){target();}}\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("CP08_LambdaScope", func(t *testing.T) {
		x := cppAuditExtract(t, "void target(){} void caller(){auto f=[](auto target){target();};}\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("CP08_CaptureScope", func(t *testing.T) {
		x := cppAuditExtract(t, "void target(){} void caller(){auto f=[target=callback]{target();};}\n")
		cppAuditNoCall(t, x, "caller", "target")
	})
	t.Run("CP09_This", func(t *testing.T) {
		x := cppAuditExtract(t, "struct X{void target(){} void caller(){this->target();}};")
		if !cpp20HasCall(x, "X::caller", "X::target") {
			t.Fatal("this call lost")
		}
	})
	t.Run("CP10_LookupOrder", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace a{void f(){}} namespace n{namespace a{void f(){}} void caller(){a::f(); ::a::f();}}\n")
		if !cpp20HasCall(x, "n::caller", "n::a::f") || !cpp20HasCall(x, "n::caller", "a::f") {
			t.Fatal("qualified lookup order")
		}
	})
	t.Run("CP17_NestedTemplate", func(t *testing.T) {
		x := cppAuditExtract(t, "namespace n{template<class T>void work(){}} void caller(){n::work<Box<int>>();}\n")
		if !cpp20HasCall(x, "caller", "n::work") {
			t.Fatal("nested template call omitted")
		}
	})
	t.Run("CP16_Directives", func(t *testing.T) {
		x := cppAuditExtract(t, "module; export module logical.name; import :part; import <vector>; module :private; void real(){int module=1;}\n")
		cppAuditSymbol(t, x, "real")
		found := false
		for _, r := range x.Relations {
			if r.Kind == "imports" && r.UnresolvedTo == "logical.name:part" {
				found = true
			}
			if r.Kind == "imports" && r.Source == "cpp:module-declaration" {
				t.Fatal("declaration became import")
			}
		}
		if !found {
			t.Fatal("partition omitted")
		}
	})
}
