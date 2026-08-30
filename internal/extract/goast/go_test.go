package goast

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractsGoSymbolsImportsCallsAndTests(t *testing.T) {
	content := []byte("package auth\n\nimport \"time\"\n\ntype Service struct{}\n\n// ValidateToken rejects expired tokens.\nfunc (s *Service) ValidateToken(token string) error {\n\tif time.Now().After(time.Now()) {\n\t\treturn ErrExpired\n\t}\n\treturn s.validate(token)\n}\n\nfunc TestValidateExpiredToken(t *testing.T) {\n\t_ = Service{}.ValidateToken(\"expired\")\n}\n")
	extraction, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth/service.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if findSymbol(extraction.Symbols, "ValidateToken") == nil || findSymbol(extraction.Symbols, "Service") == nil || findSymbol(extraction.Symbols, "TestValidateExpiredToken") == nil {
		t.Fatalf("symbols=%+v", extraction.Symbols)
	}
	if len(extraction.Relations) < 2 {
		t.Fatalf("relations=%+v", extraction.Relations)
	}
	if len(extraction.Chunks) < 3 {
		t.Fatalf("chunks=%+v", extraction.Chunks)
	}
	validate := findSymbol(extraction.Symbols, "ValidateToken")
	if validate.StartLine != 8 || validate.EndLine < validate.StartLine || validate.Handle == "" || validate.Signature == "" {
		t.Fatalf("validate=%+v", validate)
	}
}

func findSymbol(symbols []model.Symbol, name string) *model.Symbol {
	for i := range symbols {
		if symbols[i].Name == name {
			return &symbols[i]
		}
	}
	return nil
}

func TestGoExtractionIsDeterministicAndHandlesCRLF(t *testing.T) {
	content := []byte("package p\r\n\r\nfunc Run() {}\r\n")
	first, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "run.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "run.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Symbols) != len(second.Symbols) || first.Symbols[1].Handle != second.Symbols[1].Handle {
		t.Fatalf("not deterministic: first=%+v second=%+v", first.Symbols, second.Symbols)
	}
}

func TestGoTestRelationResolvesCamelCaseTargetWithInsertedQualifier(t *testing.T) {
	content := []byte("package auth\n\nfunc ValidateToken() {}\n\nfunc TestValidateExpiredToken(t *testing.T) { ValidateToken() }\n")
	extraction, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth/service_test.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	target := findSymbol(extraction.Symbols, "ValidateToken")
	testSymbol := findSymbol(extraction.Symbols, "TestValidateExpiredToken")
	if target == nil || testSymbol == nil {
		t.Fatalf("symbols=%+v", extraction.Symbols)
	}
	for _, relation := range extraction.Relations {
		if relation.FromHandle == testSymbol.Handle && relation.Kind == "tests" {
			if relation.ToHandle != target.Handle || relation.UnresolvedTo != "" {
				t.Fatalf("test relation=%+v target=%+v", relation, *target)
			}
			return
		}
	}
	t.Fatalf("resolved test relation missing: %+v", extraction.Relations)
}

func TestGoPartialParseRetainsEarlierSymbols(t *testing.T) {
	content := []byte("package auth\n\nfunc Valid() error { return nil }\n\nfunc Broken(\n")
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "broken.go", Language: "go", Content: content})
	if err != nil {
		t.Fatalf("partial parse should be returned without a fatal error: %v", err)
	}
	if findSymbol(got.Symbols, "Valid") == nil {
		t.Fatalf("partial AST symbol was discarded: %+v", got.Symbols)
	}
	foundDiagnostic := false
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Code == "go_parse_partial" {
			foundDiagnostic = true
		}
	}
	if !foundDiagnostic {
		t.Fatalf("partial parse diagnostic missing: %+v", got.Diagnostics)
	}
}

func TestGoExtractsMembersAliasesGenericsAndMultipleNames(t *testing.T) {
	content := []byte("package auth\n\ntype Alias = string\n\ntype Box[T any] struct {\n\tValue T\n\t*Embedded\n}\n\ntype Embedded interface {\n\tEmbeddedMethod() error\n}\n\ntype Validator[T any] interface {\n\tEmbedded\n\tValidate(value T) error\n}\n\nconst A, B = 1, 2\nvar C, D = A, B\n\nfunc Identity[T any](value T) T { return value }\nfunc (b *Box[T]) Validate(value T) error { return nil }\n")
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth/members.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"Alias", "type_alias"}, {"Value", "field"}, {"Embedded", "embedded_field"},
		{"EmbeddedMethod", "interface_method"}, {"A", "const"}, {"B", "const"},
		{"C", "var"}, {"D", "var"},
	} {
		found := false
		for _, symbol := range got.Symbols {
			if symbol.Name == want.name && symbol.Kind == want.kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s %s: %+v", want.kind, want.name, got.Symbols)
		}
	}
	identity := findSymbol(got.Symbols, "Identity")
	if identity == nil || !strings.Contains(identity.Signature, "[T any]") {
		t.Fatalf("generic function signature=%+v", identity)
	}
	method := findGoSymbol(got.Symbols, "Validate", "method")
	if method == nil || method.QualifiedName != "Box.Validate" || !strings.Contains(method.Signature, "[T]") {
		t.Fatalf("generic pointer receiver method=%+v", method)
	}
	validator := findGoSymbol(got.Symbols, "Validator", "interface")
	embedded := findGoSymbol(got.Symbols, "Embedded", "interface")
	if validator == nil || embedded == nil || !hasGoRelation(got.Relations, validator.Handle, embedded.Handle, "references") {
		t.Fatalf("interface embedding relation missing: %+v", got.Relations)
	}
}

func TestGoRecognizesFuzzBenchmarkAndExampleAndResolvesPlainCalls(t *testing.T) {
	content := []byte("package auth\n\nfunc Validate(value string) error { return nil }\nfunc FuzzValidate(f *testing.F) { Validate(\"ok\") }\nfunc BenchmarkValidate(b *testing.B) { Validate(\"ok\") }\nfunc ExampleValidate() { Validate(\"ok\") }\n")
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth_test.go", Language: "go", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FuzzValidate", "BenchmarkValidate", "ExampleValidate"} {
		symbol := findSymbol(got.Symbols, name)
		if symbol == nil || symbol.Kind != "test" {
			t.Errorf("%s was not recognized as test: %+v", name, symbol)
		}
	}
	validate := findSymbol(got.Symbols, "Validate")
	fuzz := findSymbol(got.Symbols, "FuzzValidate")
	if validate == nil || fuzz == nil || !hasGoRelation(got.Relations, fuzz.Handle, validate.Handle, "calls") {
		t.Fatalf("plain call relation was not resolved: %+v", got.Relations)
	}
	for _, relation := range got.Relations {
		if relation.Kind == "calls" && relation.UnresolvedTo == "Validate" && relation.FromHandle != fuzz.Handle {
			t.Fatalf("unexpected selector resolution: %+v", relation)
		}
	}
}

func hasGoRelation(relations []model.Relation, from, to, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.ToHandle == to && relation.Kind == kind {
			return true
		}
	}
	return false
}

func findGoSymbol(symbols []model.Symbol, name, kind string) *model.Symbol {
	for i := range symbols {
		if symbols[i].Name == name && symbols[i].Kind == kind {
			return &symbols[i]
		}
	}
	return nil
}
