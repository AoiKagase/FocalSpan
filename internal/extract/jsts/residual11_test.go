package jsts

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestR01ShadowedCalls(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"parameter", "function caller(target: () => void) { target(); }"},
		{"default", "function caller(target = () => {}) { target(); }"},
		{"destructure", "function caller({target}) { target(); }"},
		{"local", "function caller() { const target = getCallback(); target(); }"},
		{"var", "function caller() { if (true) { var target = getCallback(); } target(); }"},
		{"catch", "function caller() { try {} catch (target) { target(); } }"},
		{"receiver", "function caller(obj) { obj.target(); }"},
		{"nested", "function caller(target) { function inner() { target(); } inner(); }"},
		{"loop", "function caller() { for (let target of values) { target(); } }"},
		{"loop-branch", "function caller() { for (let target of values) if (condition) target(); else target(); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte("function target() {}\n" + tc.body + "\n")})
			if e != nil {
				t.Fatal(e)
			}
			names := map[string]string{}
			for _, s := range x.Symbols {
				names[s.Handle] = s.Name
			}
			for _, r := range x.Relations {
				if names[r.FromHandle] != "target" && r.Kind != "contains" && (names[r.ToHandle] == "target" || r.UnresolvedTo == "target") {
					t.Errorf("dynamic binding leaked global relation: %+v", r)
				}
			}
		})
	}
}

func TestR01LexicalControls(t *testing.T) {
	for _, source := range []string{
		"function target() {} function caller() { target(); }",
		"function target() {} function caller() { { let target = external; } target(); }",
		"function target() {} function caller() { function target() {} target(); }",
		"function caller() { const target = () => {}; target(); }",
		"function target() {} function caller() { values.forEach((target) => { target(); }); target(); }",
	} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte(source)})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		names := map[string]string{}
		for _, s := range x.Symbols {
			names[s.Handle] = s.Name
		}
		for _, r := range x.Relations {
			if names[r.FromHandle] == "caller" && r.Kind == "calls" && names[r.ToHandle] == "target" {
				found = true
			}
		}
		if !found {
			t.Errorf("known target not resolved: %s; %+v", source, x.Relations)
		}
	}
}

func TestR01LoopBindingDoesNotEscape(t *testing.T) {
	for _, loop := range []string{
		"for (let target of values) { target(); }",
		"for (const target of values) { target(); }",
		"for (let target = callback; condition; next()) target();",
	} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte("function target() {} function caller() { " + loop + " target(); }")})
		if e != nil {
			t.Fatal(e)
		}
		names := map[string]string{}
		for _, s := range x.Symbols {
			names[s.Handle] = s.Name
		}
		found := false
		for _, r := range x.Relations {
			if names[r.FromHandle] == "caller" && r.Kind == "calls" && names[r.ToHandle] == "target" {
				found = true
			}
		}
		if !found {
			t.Errorf("loop binding escaped: %s; %+v", loop, x.Relations)
		}
	}
}

func TestR01AnonymousCallbackBindings(t *testing.T) {
	for _, callback := range []string{
		"(target) => { target(); }",
		"target => { target(); }",
		"function(target) { target(); }",
		"({target}) => { target(); }",
	} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte("function target() {} function caller() { values.forEach(" + callback + "); }")})
		if e != nil {
			t.Fatal(e)
		}
		names := map[string]string{}
		for _, s := range x.Symbols {
			names[s.Handle] = s.Name
		}
		for _, r := range x.Relations {
			if r.Kind != "contains" && names[r.FromHandle] != "target" && (names[r.ToHandle] == "target" || r.UnresolvedTo == "target") {
				t.Errorf("callback parameter leaked global relation: %s; %+v", callback, r)
			}
		}
	}
}

func TestR01TypedCallbackBoundaries(t *testing.T) {
	for _, source := range []string{
		`export function validateToken(): boolean { return true; }`,
		`const value = x => ({x})`,
		`const value = x => [x]`,
	} {
		if _, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte(source)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, callback := range []string{
		`function(target): void { target(); }`,
		`(target): void => { target(); }`,
	} {
		source := `function target() {} function caller(){ values.forEach(` + callback + `); }`
		x, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte(source)})
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]string{}
		for _, symbol := range x.Symbols {
			names[symbol.Handle] = symbol.Name
		}
		for _, relation := range x.Relations {
			if relation.Kind == "calls" && (names[relation.ToHandle] == "target" || relation.UnresolvedTo == "target") {
				t.Errorf("typed callback inferred a call: %s: %+v", callback, relation)
			}
		}
	}
	source := `function target(){} function scoped(target): void {} function caller(){target();}`
	x, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.ts", Language: "typescript", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, symbol := range x.Symbols {
		names[symbol.Handle] = symbol.Name
	}
	found := false
	for _, relation := range x.Relations {
		if relation.Kind == "calls" && names[relation.FromHandle] == "caller" && names[relation.ToHandle] == "target" {
			found = true
		}
	}
	if !found {
		t.Errorf("typed function binding escaped into caller: %+v", x.Relations)
	}
}

func TestR02StaticModuleArguments(t *testing.T) {
	for _, call := range []string{"import", "require"} {
		for _, tc := range []struct{ arg, want string }{
			{"name + './dep'", ""}, {"resolve('./dep')", ""}, {"name, {type: './wrong'}", ""}, {"`${name}/dep`", ""},
			{"'./dep'", "./dep"}, {" /* before */ './dep' /* after */ ", "./dep"}, {"'./dep', {with: {type: 'json'}}", "./dep"}, {"'./dep',", "./dep"},
		} {
			t.Run(call+tc.arg, func(t *testing.T) {
				tokens, _, e := Lex(context.Background(), []byte(call+"("+tc.arg+");"))
				if e != nil {
					t.Fatal(e)
				}
				p, e := parseJSTS(context.Background(), tokens)
				if e != nil {
					t.Fatal(e)
				}
				got := ""
				for _, m := range p.Modules {
					if m.Source == "jsts:dynamic-import" || m.Source == "jsts:require" {
						got = m.Target
					}
				}
				if got != tc.want {
					t.Fatalf("module=%q want %q", got, tc.want)
				}
			})
		}
	}
}

func TestR03ExportValueIsNotModule(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"export const label = './dep';", ""},
		{"export default './dep';", ""},
		{"export type Choice = './a' | './b';", ""},
		{"export function value() { return './dep'; }", ""},
		{"const value = 1; export {value};", ""},
		{"export * from './dep';", "./dep"},
		{"export {value as alias} from './dep';", "./dep"},
	} {
		tokens, _, e := Lex(context.Background(), []byte(tc.source))
		if e != nil {
			t.Fatal(e)
		}
		p, e := parseJSTS(context.Background(), tokens)
		if e != nil {
			t.Fatal(e)
		}
		got := ""
		for _, m := range p.Modules {
			if m.Source == "jsts:export" || m.Source == "jsts:reexport" {
				got = m.Target
			}
		}
		if got != tc.want {
			t.Errorf("%s: module=%q want %q", tc.source, got, tc.want)
		}
	}
	tokens, _, _ := Lex(context.Background(), []byte("export async function load() { return import('./real'); }"))
	p, e := parseJSTS(context.Background(), tokens)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, m := range p.Modules {
		if m.Source == "jsts:dynamic-import" && m.Target == "./real" {
			found = true
		}
	}
	if !found {
		t.Error("dynamic import inside exported function lost")
	}
}

func TestR03ExportSymbolsPreserved(t *testing.T) {
	for _, tc := range []struct{ code, name string }{
		{"export const value = './not-module';", "value"},
		{"export function work() {}", "work"},
		{"export async function work() {}", "work"},
		{"export class Example {}", "Example"},
		{"export type Choice = 'value';", "Choice"},
		{"export default 1;", "default"},
	} {
		tokens, _, _ := Lex(context.Background(), []byte(tc.code))
		p, e := parseJSTS(context.Background(), tokens)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, m := range p.Modules {
			if m.Source == "jsts:export-symbol" && m.Target == tc.name {
				found = true
			}
		}
		if !found {
			t.Errorf("export symbol %s missing from %s", tc.name, tc.code)
		}
	}
}
