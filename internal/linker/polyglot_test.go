package linker

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLinkerResolvesScopedModulePathReferencesAcrossTargetProfiles(t *testing.T) {
	s := openLinkerStore(t)
	if !linkerPathMatch("plugin.lua", "lib/auth.lua", "lib/auth") {
		t.Fatal("extensionless module path did not match its source file")
	}
	cases := []struct {
		name, caller, target, targetName, unresolved, kind string
	}{
		{"go", "cmd/main.go", "internal/auth.go", "GoValidate", "GoValidate", "calls"},
		{"rust", "src/lib.rs", "src/rust_auth.rs", "RustAuth", "src/rust_auth", "imports"},
		{"cpp", "src/auth.cpp", "include/auth.hpp", "CppAuth", "include/auth.hpp", "references"},
		{"csharp", "Views/MainWindow.xaml", "Views/MainWindow.xaml.cs", "MainWindow", "Views/MainWindow.xaml.cs", "references"},
		{"php", "src/Http/Middleware.php", "src/Auth/TokenService.php", "TokenService", "src/Auth/TokenService.php", "references"},
		{"javascript", "src/index.ts", "src/js_auth.ts", "JSAuth", "src/js_auth", "calls"},
		{"python", "app/__init__.py", "app/auth.py", "PyAuth", "app/auth", "imports"},
		{"ruby", "lib/auth.rb", "lib/token.rb", "RubyToken", "lib/token.rb", "imports"},
		{"lua", "plugin.lua", "lib/lua_auth.lua", "LuaAuth", "lib/lua_auth", "imports"},
		{"pawn", "addons/amxmodx/scripting/plugin.sma", "addons/amxmodx/scripting/include/auth.inc", "PawnAuth", "addons/amxmodx/scripting/include/auth.inc", "imports"},
		{"vb6", "MainForm.frm", "AuthService.cls", "AuthService", "AuthService.cls", "references"},
		{"vbnet", "Views/MainWindow.xaml.vb", "Forms/Auth.vb", "VbAuth", "Forms/Auth.vb", "references"},
		{"nim", "src/auth.nim", "src/nim_auth_types.nim", "NimToken", "nim_auth_types", "imports"},
		{"zig", "src/auth.zig", "src/auth_types.zig", "ZigToken", "auth_types", "imports"},
	}
	for _, test := range cases {
		seedLinkerFile(t, s, test.target, test.name+"-target", test.targetName, test.targetName, nil)
		seedLinkerFile(t, s, test.caller, test.name+"-caller", test.name+"Caller", test.name+"Caller", []model.Relation{{FromHandle: test.name + "-caller", UnresolvedTo: test.unresolved, Kind: test.kind, Confidence: .4, Source: "test"}})
	}
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		found := false
		for _, relation := range relations {
			if relation.FromHandle == test.name+"-caller" {
				found = true
				if relation.ToHandle != test.name+"-target" || relation.UnresolvedTo != "" {
					t.Fatalf("%s relation=%+v", test.name, relation)
				}
			}
		}
		if !found {
			t.Fatalf("%s relation missing", test.name)
		}
	}
}
