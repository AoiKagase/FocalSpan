package projectmeta

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestF19CargoAssignmentScope(t *testing.T) {
	file := model.SourceFile{Path: "Cargo.toml", Content: []byte("[package]\nname = \"actual\"\nversion = \"1.0.0\"\n")}
	facts, diags, err := ParseFile(context.Background(), "", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("diagnostics=%+v", diags)
	}
	if len(facts) != 1 || facts[0].Kind != "package" || facts[0].Target != "actual" {
		t.Fatalf("unrelated quoted lines created package/path facts: %+v", facts)
	}
}

func TestF20ComposerArrayMappings(t *testing.T) {
	file := model.SourceFile{Path: "composer.json", Content: []byte(`{"autoload":{"psr-4":{"Demo\\":["src/","generated/"]}}}`)}
	facts, diags, err := ParseFile(context.Background(), "", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("diagnostics=%+v", diags)
	}
	got := map[string]bool{}
	for _, f := range facts {
		if f.Kind == "psr-4" && f.Name == "Demo\\" {
			got[f.Target] = true
		}
	}
	if len(got) != 2 || !got["src/"] || !got["generated/"] {
		t.Fatalf("array-valued namespace mapping discarded: %+v", facts)
	}
}

func TestF19CargoStructuredTables(t *testing.T) {
	text := "[package]\nname = 'actual' # real package\nversion='1'\n[lib]\npath='src/lib.rs'\n[dependencies]\na={path='../a',version='1'}\nb='1'\n[dev-dependencies.c]\npath='../c'\n[workspace]\nmembers=['member']\n"
	facts, diags, err := ParseFile(context.Background(), "", model.SourceFile{Path: "Cargo.toml", Content: []byte(text)})
	if err != nil || len(diags) != 0 {
		t.Fatalf("%+v %v", diags, err)
	}
	want := map[string]bool{"package:actual": false, "lib-path:src/lib.rs": false, "path-dependency:../a": false, "path-dependency:../c": false, "workspace-member:member": false}
	for _, f := range facts {
		key := f.Kind + ":" + f.Target
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected fact %+v", f)
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Errorf("missing %s", key)
		}
	}
}
func TestF20ComposerAutoloadDevAndInvalidElement(t *testing.T) {
	facts, diags, err := ParseFile(context.Background(), "", model.SourceFile{Path: "composer.json", Content: []byte(`{"autoload-dev":{"psr-4":{"Demo\\":["tests/",12]}}}`)})
	if err != nil || len(diags) == 0 {
		t.Fatalf("invalid entry must be diagnosed: %+v %v", diags, err)
	}
	if len(facts) != 1 || facts[0].Target != "tests/" {
		t.Fatalf("valid entry lost or invalid fact emitted: %+v", facts)
	}
}
