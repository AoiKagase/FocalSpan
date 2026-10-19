package linker

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/php"
	"github.com/focalspan/focalspan/internal/model"
)

func TestPHPFunctionalScopedLinking(t *testing.T) {
	ctx := context.Background()
	s := openLinkerStore(t)
	sources := map[string]string{
		"caller.php":    `<?php namespace App; use Vendor\Widget as Shared; use function Vendor\work as Shared; use function Vendor\work as act; function local() {} function run() { new Shared(); helper(); ping(); $unknown->ping(); act(); Missing\helper(); local(); }`,
		"duplicate.inc": `<?php namespace App; function local() {}`,
		"other.inc":     `<?php namespace Other; function helper() {} class Service { function ping() {} }`,
		"vendor.php":    `<?php namespace Vendor; function work() {} class Widget {} function Widget() {}`,
	}
	for path, source := range sources {
		file := model.SourceFile{Path: path, Language: "php", Content: []byte(source), SHA256: path}
		extraction, err := php.NewExtractor().Extract(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.ReplaceFile(ctx, file, extraction); err != nil {
			t.Fatal(err)
		}
	}
	if err := (&Linker{Store: s}).Link(ctx, nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	symbols, _ := s.Symbols(ctx)
	byHandle := map[string]model.Symbol{}
	for _, symbol := range symbols {
		byHandle[symbol.Handle] = symbol
	}
	found := false
	for _, rel := range relations {
		if rel.Kind != "calls" {
			continue
		}
		if rel.Source == "act()" {
			found = rel.ToHandle != "" && byHandle[rel.ToHandle].QualifiedName == `Vendor\work`
			continue
		}
		if rel.ToHandle != "" {
			t.Errorf("out-of-scope link: %+v => %+v", rel, byHandle[rel.ToHandle])
		}
	}
	if !found {
		t.Error("imported cross-file call not resolved")
	}
	typeFound := false
	for _, rel := range relations {
		if rel.Source == "new" {
			typeFound = rel.ToHandle != "" && byHandle[rel.ToHandle].QualifiedName == `Vendor\Widget` && byHandle[rel.ToHandle].Kind == "class"
		}
	}
	if !typeFound {
		t.Error("cross-file class alias was lost or confused with function alias")
	}
	// The short lexical name must remain a dependency when a matching function
	// appears in a differently named file, including after prior linking.
	paths, err := s.DependentSourcePaths(ctx, []string{"helper"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "caller.php" {
		t.Errorf("missing incremental dependency: %v", paths)
	}
	paths, err = s.DependentSourcePaths(ctx, []string{"work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "caller.php" {
		t.Errorf("missing imported dependency: %v", paths)
	}
}
