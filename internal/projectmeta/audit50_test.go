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
