package language

import (
	"reflect"
	"sort"
	"testing"
)

func TestDetectKnownProfilesAndAmbiguousContent(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    string
	}{
		{"go", "main.go", "", "go"},
		{"rust", "main.rs", "", "rust"},
		{"python", "tool.py", "", "python"},
		{"python pyw", "tool.pyw", "", "python"},
		{"python stub", "types.pyi", "", "python"},
		{"ruby", "script.rb", "", "ruby"},
		{"ruby rakefile", "Rakefile", "", "ruby"},
		{"nim", "tool.nim", "", "nim"},
		{"nim script", "build.nims", "", "nim"},
		{"nim package", "package.nimble", "", "nim"},
		{"zig", "main.zig", "", "zig"},
		{"zig build", "build.zig", "", "zig"},
		{"vb6 form", "Form1.frm", "", "vb6"},
		{"vb6 module", "Module1.bas", "", "vb6"},
		{"vb6 class", "Class1.cls", "", "vb6"},
		{"vb6 project", "App.vbp", "", "vb6-project"},
		{"vbnet", "Module1.vb", "", "vbnet"},
		{"xaml", "View.xaml", "", "xaml"},
		{"csharp code behind", "View.xaml.cs", "", "csharp"},
		{"vbnet code behind", "View.xaml.vb", "", "vbnet"},
		{"resx", "Form1.resx", "", "dotnet-resource"},
		{"settings", "Settings.settings", "", "dotnet-resource"},
		{"lua", "main.lua", "", "lua"},
		{"pawn sma", "plugin.sma", "", "pawn"},
		{"pawn pwn", "plugin.pwn", "", "pawn"},
		{"typescript mts", "main.mts", "", "typescript"},
		{"typescript cts", "main.cts", "", "typescript"},
		{"typescript declaration", "types.d.ts", "", "typescript"},
		{"smarty", "page.tpl", `{block name="content"}{/block}`, "smarty"},
		{"template", "page.tpl", `<p>plain</p>`, "template"},
		{"php inc", "common.inc", `<?php echo $value;`, "php"},
		{"pawn inc", "common.inc", `#include <amxmodx>
public plugin_init() { register_plugin("x", "1", "x"); }`, "pawn"},
		{"plain inc", "plain.inc", `plain text`, "text"},
		{"php wins over pawn markers", "common.inc", `<?php
// public plugin_init and #include are only comments
echo "native register_plugin";`, "php"},
		{"pawn words in comments are not enough", "common.inc", `// public plugin_init
/* #include native forward */
"register_plugin"`, "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.path, []byte(tt.content), nil)
			if got.Language != tt.want {
				t.Fatalf("Detect(%q)=%+v, want language %q", tt.path, got, tt.want)
			}
			if got.Confidence <= 0 {
				t.Fatalf("Detect(%q) returned non-positive confidence: %+v", tt.path, got)
			}
		})
	}
}

func TestDetectRecognizesRemainingProfiles(t *testing.T) {
	for _, tt := range []struct {
		path string
		want string
	}{
		{"x.c", "c"}, {"x.cpp", "cpp"}, {"x.hpp", "cpp"},
		{"x.csx", "csharp"}, {"x.js", "javascript"}, {"x.tsx", "typescript"},
		{"x.php8", "php"}, {"x.phtml", "php"}, {"x.md", "markdown"},
		{"x.json", "config"}, {"x.toml", "config"}, {"x.rockspec", "lua"},
		{"x.rake", "ruby"}, {"x.gemspec", "ruby"}, {"x.shell", "text"},
	} {
		if got := Detect(tt.path, nil, nil).Language; got != tt.want {
			t.Errorf("Detect(%q)=%q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestKnownLanguagesIsSortedAndStable(t *testing.T) {
	first := KnownLanguages()
	second := KnownLanguages()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("KnownLanguages is not stable: %v then %v", first, second)
	}
	if !sort.StringsAreSorted(first) {
		t.Fatalf("KnownLanguages is not sorted: %v", first)
	}
	for _, name := range []string{"go", "rust", "python", "ruby", "nim", "zig", "vb6", "vbnet", "lua", "pawn", "xaml", "dotnet-resource", "text"} {
		if !IsKnown(name) {
			t.Errorf("IsKnown(%q)=false", name)
		}
	}
	if IsKnown("not-a-language") {
		t.Fatal("unknown language was reported as known")
	}
}
