package projectmeta

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestParseMetadataExtractsStaticFactsWithoutExecutingProjectFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want []string
	}{
		{name: "go", path: "go.mod", body: "module example.com/auth\n\nreplace example.com/common => ../common\n", want: []string{"module:example.com/auth", "replace:../common"}},
		{name: "cargo", path: "Cargo.toml", body: "[package]\nname = \"auth\"\n[dependencies.core]\npath = \"../core\"\n", want: []string{"package:auth", "path-dependency:../core"}},
		{name: "node", path: "package.json", body: `{"name":"auth-app","type":"module","main":"src/index.js","workspaces":["packages/*"]}`, want: []string{"package:auth-app", "type:module", "main:src/index.js", "workspace:packages/*"}},
		{name: "tsconfig", path: "tsconfig.json", body: `{"baseUrl":"src","paths":{"@auth/*":["auth/*"]}}`, want: []string{"baseUrl:src", "path-alias:@auth/*", "path-alias:auth/*"}},
		{name: "jsconfig", path: "jsconfig.json", body: `{"baseUrl":"src","paths":{"@app/*":["app/*"]}}`, want: []string{"baseUrl:src", "path-alias:@app/*", "path-alias:app/*"}},
		{name: "dotnet", path: "Auth.csproj", body: `<Project><PropertyGroup><RootNamespace>Demo.Auth</RootNamespace></PropertyGroup><ItemGroup><ProjectReference Include="..\Core\Core.csproj" /><Compile Include="Auth.cs" /></ItemGroup></Project>`, want: []string{"namespace:Demo.Auth", "project-reference:..\\Core\\Core.csproj", "compile:Auth.cs"}},
		{name: "vbproj", path: "Auth.vbproj", body: `<Project><PropertyGroup><RootNamespace>Demo.Auth</RootNamespace></PropertyGroup><ItemGroup><ProjectReference Include="..\Core\Core.vbproj" /></ItemGroup></Project>`, want: []string{"namespace:Demo.Auth", "project-reference:..\\Core\\Core.vbproj"}},
		{name: "composer", path: "composer.json", body: `{"name":"demo/auth","autoload":{"psr-4":{"Demo\\Auth\\":"src/"},"files":["src/helpers.php"]}}`, want: []string{"package:demo/auth", "psr-4:Demo\\Auth\\", "psr-4:src/", "files:src/helpers.php"}},
		{name: "python", path: "pyproject.toml", body: "[project]\nname = \"demo-auth\"\n[tool.setuptools]\nwhere = \"src\"\n", want: []string{"package:demo-auth", "package-dir:src"}},
		{name: "ruby", path: "Gemfile", body: "source \"https://rubygems.org\"\nrequire_relative \"lib/auth\"\n", want: []string{"require-relative:lib/auth"}},
		{name: "gemspec", path: "auth.gemspec", body: "Gem::Specification.new do |s|\n  s.name = \"demo-auth\"\n  s.require_paths = [\"lib\"]\nend\n", want: []string{"gem:demo-auth", "require-path:lib"}},
		{name: "rockspec", path: "auth-1.rockspec", body: "package = \"auth\"\nsource = { url = \"../auth\" }\n", want: []string{"package:auth", "package:../auth"}},
		{name: "vb6", path: "Project.vbp", body: "Form=MainForm.frm\nClass=AuthService.cls\nModule=AuthModule.bas\n", want: []string{"component:MainForm.frm", "component:AuthService.cls", "component:AuthModule.bas"}},
		{name: "nimble", path: "auth.nimble", body: "name = \"auth\"\nsrcDir = \"src\"\n", want: []string{"package:auth", "src-dir:src"}},
		{name: "zig", path: "build.zig.zon", body: ".name = .{ .bytes = \"auth\" },\n.dependencies = .{ core = .{ .path = \"../core\" } },\n", want: []string{"package:auth", "path:../core"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts, diagnostics, err := ParseFile(context.Background(), t.TempDir(), model.SourceFile{Path: test.path, Content: []byte(test.body)})
			if err != nil {
				t.Fatal(err)
			}
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v", diagnostics)
			}
			for _, expected := range test.want {
				if !factContains(facts, expected) {
					t.Errorf("fact %q missing: %+v", expected, facts)
				}
			}
		})
	}
}

func TestMetadataParsingHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := ParseFile(ctx, t.TempDir(), model.SourceFile{Path: "go.mod", Content: []byte("module example.com/auth\n")}); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestMetadataParsingSupportsCommonManifestForms(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want []string
	}{
		{name: "go workspace block", path: "go.work", body: "go 1.26\nuse (\n\t./services/api\n\t../shared\n)\n", want: []string{"workspace-use:./services/api", "workspace-use:../shared"}},
		{name: "setup cfg", path: "setup.cfg", body: "[metadata]\nname = demo-auth\n[options]\npackage_dir =\n    =src\n", want: []string{"package:demo-auth", "package-dir:src"}},
		{name: "local gem", path: "Gemfile", body: "gem \"common-auth\", path: \"../common-auth\"\n", want: []string{"gem-path:../common-auth"}},
		{name: "rockspec directory", path: "auth-1.rockspec", body: "package = \"auth\"\nsource = { url = \"https://example.invalid/auth.tar.gz\", dir = \"src\" }\n", want: []string{"path:src"}},
		{name: "nimble dependency", path: "auth.nimble", body: "name = \"auth\"\nrequires \"../common\"\n", want: []string{"path-dependency:../common"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts, diagnostics, err := ParseFile(context.Background(), t.TempDir(), model.SourceFile{Path: test.path, Content: []byte(test.body)})
			if err != nil {
				t.Fatal(err)
			}
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v", diagnostics)
			}
			for _, expected := range test.want {
				if !factContains(facts, expected) {
					t.Errorf("fact %q missing: %+v", expected, facts)
				}
			}
		})
	}
}

func factContains(facts []Fact, value string) bool {
	for _, fact := range facts {
		if fact.Kind+":"+fact.Target == value || fact.Kind+":"+fact.Name == value {
			return true
		}
	}
	return false
}
