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
