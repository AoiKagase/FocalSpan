package goast

import (
	"context"
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
