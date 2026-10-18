package csharp

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCS01GenericMethodNamesAndCalls(t *testing.T) {
	source := "namespace Demo;\nclass Service {\n public T Echo<T>(T value) { return value; }\n public T Map<T,U>(T value, U other) { return value; }\n public int Value() { return 1; }\n public int Value<T>() { return 1; }\n public int Plain() { return 1; }\n public static Service operator >>(Service value, int count) => value;\n public void Run() { Echo<int>(1); Map<System.Collections.Generic.Dictionary<string,System.Collections.Generic.List<int>>,int>(null, 1); Plain(); }\n}\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{{"Echo", "method"}, {"Map", "method"}, {"Plain", "method"}, {"operator >>", "operator"}} {
		if s := findSymbol(got.Symbols, "Demo.Service."+want.name, want.kind); s.Handle == "" {
			t.Errorf("missing %s %s", want.kind, want.name)
		}
	}
	run := findSymbol(got.Symbols, "Demo.Service.Run", "method")
	for _, name := range []string{"Echo", "Map", "Plain"} {
		target := findSymbol(got.Symbols, "Demo.Service."+name, "method")
		found := false
		for _, rel := range got.Relations {
			if rel.FromHandle == run.Handle && rel.ToHandle == target.Handle && target.Handle != "" && rel.Kind == "calls" {
				found = true
			}
		}
		if !found {
			t.Errorf("Run has no call to %s", name)
		}
	}
	count := 0
	for _, s := range got.Symbols {
		if s.QualifiedName == "Demo.Service.Value" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Value overload count=%d, want 2", count)
	}
	for _, s := range got.Symbols {
		if s.Kind == "method" && strings.ContainsAny(s.Name, "<>") {
			t.Errorf("invalid generic method name %q", s.Name)
		}
	}
}

func TestCS02ExpressionBodiesProduceCallAndTestRelations(t *testing.T) {
	source := "namespace Demo;\nclass Service {\n void Save() {}\n void Accept(System.Func<int> value) {}\n public void Run() => Save();\n [Fact] public void Checks() => Save();\n public void Nested() => Accept(() => { var n = 1; return n; });\n public void After() => Save();\n}\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "ServiceTests.cs", Language: "csharp", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ from, kind, to, relation string }{
		{"Run", "method", "Save", "calls"}, {"Checks", "test", "Save", "tests"},
		{"Nested", "method", "Accept", "calls"}, {"After", "method", "Save", "calls"},
	} {
		from := findSymbol(got.Symbols, "Demo.Service."+tc.from, tc.kind)
		to := findSymbol(got.Symbols, "Demo.Service."+tc.to, "method")
		found := false
		for _, r := range got.Relations {
			if from.Handle != "" && to.Handle != "" && r.FromHandle == from.Handle && r.ToHandle == to.Handle && r.Kind == tc.relation {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s relation %s -> %s", tc.relation, tc.from, tc.to)
		}
		for _, chunk := range got.Chunks {
			if chunk.SymbolHandle == from.Handle && chunk.Content != source[chunk.StartByte:chunk.EndByte] {
				t.Errorf("%s source mismatch", tc.from)
			}
		}
	}
	nested := findSymbol(got.Symbols, "Demo.Service.Nested", "method")
	if nested.Handle == "" || !strings.HasSuffix(source[nested.StartByte:nested.EndByte], "return n; });") {
		t.Errorf("Nested expression was truncated: %+v", nested)
	}
}
