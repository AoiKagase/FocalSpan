package php

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func extractFunctional(t *testing.T, source string) model.Extraction {
	t.Helper()
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/functional.inc", Language: "php", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFunctionalGroupedImports(t *testing.T) {
	source := `<?php namespace App;
use Vendor\{Thing, Sub\Other as Alias, function helpers\run as act, const Settings\VALUE};
class After {}`
	got := extractFunctional(t, source)
	for _, target := range []string{`Vendor\Thing`, `Vendor\Sub\Other`, `Vendor\helpers\run`, `Vendor\Settings\VALUE`} {
		if !hasUnresolvedTarget(got.Relations, target, "imports") {
			t.Errorf("missing grouped import %s", target)
		}
	}
	requireSymbol(t, got.Symbols, "App\\After", "class")
	if len(got.Diagnostics) != 0 {
		t.Errorf("valid group diagnostics: %+v", got.Diagnostics)
	}
}

func TestFunctionalTypedConstants(t *testing.T) {
	declaration := `#[Marker] public const string NAME = 'x', OTHER = 'y';`
	source := "<?php class Example {\n" + declaration + "\npublic const (A&B)|C VALUE = C::VALUE;\npublic const PLAIN = [Foo::BAR, Baz::QUX];\n}"
	got := extractFunctional(t, source)
	for _, name := range []string{"NAME", "OTHER", "VALUE", "PLAIN"} {
		requireSymbol(t, got.Symbols, "Example::"+name, "constant")
	}
	for _, symbol := range got.Symbols {
		if symbol.Kind != "constant" {
			continue
		}
		if symbol.Name == "string" || symbol.Name == "A" || symbol.Name == "Baz" {
			t.Errorf("false constant: %+v", symbol)
		}
		if symbol.Name == "NAME" && string([]byte(source)[symbol.StartByte:symbol.EndByte]) != declaration {
			t.Errorf("incomplete source: %q", source[symbol.StartByte:symbol.EndByte])
		}
	}
}

func TestFunctionalParenthesizedIncludes(t *testing.T) {
	for _, expression := range []string{`'bootstrap.inc'`, `('bootstrap.inc')`, `(('bootstrap.inc'))`, `(__DIR__ . '/bootstrap.inc')`, `(dirname(__FILE__) . '/bootstrap.inc')`} {
		for _, keyword := range []string{"include", "include_once", "require", "require_once"} {
			t.Run(keyword+expression, func(t *testing.T) {
				got := extractFunctional(t, "<?php function loadIt() { "+keyword+" "+expression+"; }")
				owner := requireSymbol(t, got.Symbols, "loadIt", "function")
				if !hasUnresolvedRelation(got.Relations, owner.Handle, "src/bootstrap.inc", "imports") {
					t.Errorf("static include not normalized: %+v", got.Relations)
				}
			})
		}
	}
	for _, expression := range []string{`($dynamic)`, `('/outside.inc')`, `('../../outside.inc')`, `("$dynamic.inc")`} {
		got := extractFunctional(t, "<?php include "+expression+";")
		for _, relation := range got.Relations {
			if relation.Kind == "imports" && relation.Confidence > .25 {
				t.Errorf("unsafe/dynamic include resolved: %+v", relation)
			}
		}
	}
}

func TestFunctionalImportKinds(t *testing.T) {
	source := `<?php namespace Vendor; class Widget {} function helper() {} namespace App;
use Vendor\Widget as Shared;
use function Vendor\helper as Shared;
use const Vendor\VALUE as Shared;
function run() { new Shared(); Shared(); }`
	got := extractFunctional(t, source)
	owner := requireSymbol(t, got.Symbols, "App\\run", "function")
	widget := requireSymbol(t, got.Symbols, "Vendor\\Widget", "class")
	helper := requireSymbol(t, got.Symbols, "Vendor\\helper", "function")
	if !hasRelation(got.Relations, owner.Handle, widget.Handle, "references") {
		t.Error("class alias overwritten by function/constant alias")
	}
	if !hasRelation(got.Relations, owner.Handle, helper.Handle, "calls") {
		t.Error("function alias overwritten by class/constant alias")
	}
}

func TestFunctionalCallsPreserveScope(t *testing.T) {
	source := `<?php namespace Other; function helper() {} class Service { public function ping() {} }
namespace App; function run() { helper(); ping(); $unknown->ping(); Missing\helper(); }`
	got := extractFunctional(t, source)
	owner := requireSymbol(t, got.Symbols, "App\\run", "function")
	for _, relation := range got.Relations {
		if relation.FromHandle != owner.Handle || relation.Kind != "calls" {
			continue
		}
		if relation.ToHandle != "" {
			t.Errorf("out-of-scope call resolved: %+v", relation)
		}
	}
	found := false
	for _, relation := range got.Relations {
		if relation.FromHandle == owner.Handle && strings.Contains(relation.UnresolvedTo, "$unknown->ping") {
			found = true
		}
	}
	if !found {
		t.Error("unknown receiver was discarded")
	}
}

func TestFunctionalGroupedImportRecovery(t *testing.T) {
	for _, broken := range []string{`use Vendor\{Thing;`, `use Vendor\{Thing`} {
		got := extractFunctional(t, "<?php namespace App; "+broken+"\nclass After { public function ok() {} }")
		requireSymbol(t, got.Symbols, `App\After`, "class")
		requireSymbol(t, got.Symbols, `App\After::ok`, "method")
		if !hasDiagnosticCode(got.Diagnostics, "php_malformed_declaration") {
			t.Error("missing malformed import diagnostic")
		}
	}
}

func TestFunctionalGroupedImportRecoversFollowingFunction(t *testing.T) {
	got := extractFunctional(t, "<?php namespace App; use Vendor\\{Thing\nfunction after(): void {}")
	requireSymbol(t, got.Symbols, `App\after`, "function")
	if !hasDiagnosticCode(got.Diagnostics, "php_malformed_declaration") {
		t.Error("missing malformed import diagnostic")
	}
}

func TestFunctionalQualifiedAndOpaqueCalls(t *testing.T) {
	source := `<?php namespace Vendor; function act() {} class Service { function ping() {} }
 namespace App; use Vendor as V; use function Vendor\act as action;
 function ping() {} function helper() {}
 function run() { namespace\helper(); factory()->ping(); $object?->ping(); }
 function qualified() { V\act(); } function imported() { action(); } function absolute() { \Vendor\act(); }`
	got := extractFunctional(t, source)
	run := requireSymbol(t, got.Symbols, `App\run`, "function")
	helper := requireSymbol(t, got.Symbols, `App\helper`, "function")
	act := requireSymbol(t, got.Symbols, `Vendor\act`, "function")
	if !hasRelation(got.Relations, run.Handle, helper.Handle, "calls") {
		t.Error("namespace-relative function not resolved")
	}
	count := 0
	for _, rel := range got.Relations {
		if rel.Kind != "calls" {
			continue
		}
		if rel.ToHandle == act.Handle {
			count++
		}
		if rel.FromHandle == run.Handle && rel.Source == "ping()" && rel.ToHandle != "" {
			t.Errorf("opaque receiver became a bare call: %+v", rel)
		}
	}
	if count != 3 {
		t.Errorf("qualified/imported calls count=%d want 3", count)
	}
}

func TestFunctionalConstantAliasesAreCaseSensitive(t *testing.T) {
	file := model.SourceFile{Path: "aliases.php", Language: "php", Content: []byte(`<?php namespace App; use const Vendor\ONE as Value; use const Vendor\TWO as value;`)}
	tokens, diagnostics, err := Lex(context.Background(), file.Content)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parsePHP(context.Background(), file, tokens, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	builder := phpBuilder{ctx: context.Background(), file: file, parsed: parsed}
	aliases := builder.indexSymbols().aliases["App"]["const"]
	if aliases["Value"] != `Vendor\ONE` || aliases["value"] != `Vendor\TWO` {
		t.Errorf("constant alias collision: %+v", aliases)
	}
}

func TestFunctionalAbsoluteTypeNames(t *testing.T) {
	got := extractFunctional(t, `<?php namespace Vendor; class Entity {} function Entity() {}
namespace App;
class Child extends \Vendor\Entity { public \Vendor\Entity $field; function typed(\Vendor\Entity $x): \Vendor\Entity {} }
function create() { new \Vendor\Entity(); }
function reflectType() { \Vendor\Entity::class; }`)
	entity := requireSymbol(t, got.Symbols, `Vendor\Entity`, "class")
	for _, test := range []struct{ name, kind string }{{`App\Child`, "class"}, {`App\Child::field`, "property"}, {`App\Child::typed`, "method"}, {`App\create`, "function"}, {`App\reflectType`, "function"}} {
		owner := requireSymbol(t, got.Symbols, test.name, test.kind)
		if !hasRelation(got.Relations, owner.Handle, entity.Handle, "references") {
			t.Errorf("absolute type lost for %s", test.name)
		}
	}
}
