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

func TestCS03CallsRespectReceiverAndDeclaringType(t *testing.T) {
	for _, tc := range []struct{ name, ns, call, want string }{
		{"bare", "namespace Demo;", "Save();", "Demo.Service.Save"},
		{"global namespace", "", "Save();", "Service.Save"},
		{"this", "namespace Demo;", "this.Save();", "Demo.Service.Save"},
		{"explicit type", "namespace Demo;", "Other.Save();", "Demo.Other.Save"},
		{"fully qualified type", "namespace Demo;", "Demo.Other.Save();", "Demo.Other.Save"},
		{"unknown receiver", "namespace Demo;", "foreign.Save();", ""},
		{"conditional receiver", "namespace Demo;", "foreign?.Save();", ""},
		{"unknown base", "namespace Demo;", "base.Save();", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.ns + "\nclass Service { void Save() {} void Run() { " + tc.call + " } }\nclass Other { public static void Save() {} }\n"
			got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(source)})
			if err != nil {
				t.Fatal(err)
			}
			prefix := ""
			if tc.ns != "" {
				prefix = "Demo."
			}
			from := findSymbol(got.Symbols, prefix+"Service.Run", "method")
			if from.Handle == "" {
				t.Fatal("Run missing")
			}
			count := 0
			for _, r := range got.Relations {
				if r.FromHandle != from.Handle || r.Kind != "calls" {
					continue
				}
				count++
				if tc.want == "" {
					if r.ToHandle != "" || r.UnresolvedTo == "" {
						t.Errorf("unknown receiver resolved: %+v", r)
					}
				} else {
					target := findSymbol(got.Symbols, tc.want, "method")
					if target.Handle == "" || r.ToHandle != target.Handle {
						t.Errorf("call=%+v, want %s", r, tc.want)
					}
				}
			}
			if count != 1 {
				t.Errorf("calls=%d, want 1", count)
			}
		})
	}
}
func TestCS03AmbiguousOverloadStaysUnresolved(t *testing.T) {
	source := "class Service { void Save(int x) {} void Save(string x) {} void Run() { Save(1); } }"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	from := findSymbol(got.Symbols, "Service.Run", "method")
	found := false
	for _, r := range got.Relations {
		if r.FromHandle == from.Handle && r.Kind == "calls" {
			found = true
			if r.ToHandle != "" || r.UnresolvedTo != "Save" {
				t.Errorf("ambiguous call=%+v", r)
			}
		}
	}
	if !found {
		t.Fatal("missing unresolved Save call")
	}
}

func TestCS04PropertiesKeepCompleteDeclarationSource(t *testing.T) {
	cases := []struct{ name, member, symbol string }{
		{"required", "public required string Name { get; init; }", "Name"},
		{"attributes", "[Label(\"name\")]\n public string Name { get; set; }", "Name"},
		{"multiline type", "public\n System.Collections.Generic.Dictionary<string, int>\n Name { get; }", "Name"},
		{"expression", "public string Name => Format();", "Name"},
		{"indexer", "public string this[int index] { get => Format(); }", "this[]"},
		{"expression indexer", "public string this[int index] => Format();", "this[]"},
		{"explicit interface", "string IName.Name { get; }", "Name"},
		{"initializer", "public string[] Name { get; } = new[] { \"x\", \"y\" };", "Name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "class Service {\n private int prior;\n // unrelated comment\n " + tc.member + "\n void After() {}\n string Format() { return \"\"; }\n}\n"
			got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(source)})
			if err != nil {
				t.Fatal(err)
			}
			property := findSymbol(got.Symbols, "Service."+tc.symbol, "property")
			if property.Handle == "" {
				t.Fatal("property missing")
			}
			if actual := source[property.StartByte:property.EndByte]; actual != tc.member {
				t.Errorf("property source=%q, want %q", actual, tc.member)
			}
			if !strings.HasPrefix(property.Signature, strings.Split(tc.member, " {")[0]) && !strings.HasPrefix(tc.member, property.Signature) {
				t.Errorf("incomplete signature %q", property.Signature)
			}
			found := false
			for _, chunk := range got.Chunks {
				if chunk.SymbolHandle == property.Handle {
					found = true
					if chunk.Content != tc.member {
						t.Errorf("chunk=%q, want %q", chunk.Content, tc.member)
					}
				}
			}
			if !found {
				t.Fatal("property chunk missing")
			}
			if findSymbol(got.Symbols, "Service.After", "method").Handle == "" {
				t.Fatal("following member lost")
			}
		})
	}
}

func TestCS03LocalFunctionsDoNotEscapeTheirMethod(t *testing.T) {
	source := "class Service { void Owner() { void Local() {} Local(); } void Other() { Local(); Service.Local(); } }"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Service.cs", Language: "csharp", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	owner := findSymbol(got.Symbols, "Service.Owner", "method")
	other := findSymbol(got.Symbols, "Service.Other", "method")
	local := findSymbol(got.Symbols, "Service.Local", "method")
	own := false
	outside := 0
	for _, r := range got.Relations {
		if r.Kind != "calls" {
			continue
		}
		if r.FromHandle == owner.Handle && r.ToHandle == local.Handle {
			own = true
		}
		if r.FromHandle == other.Handle {
			outside++
			if r.ToHandle != "" {
				t.Errorf("local function escaped scope: %+v", r)
			}
		}
	}
	if !own || outside != 2 {
		t.Errorf("own local=%v outside calls=%d", own, outside)
	}
}
