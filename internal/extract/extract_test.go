package extract

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/extract/php"
	templateextract "github.com/focalspan/focalspan/internal/extract/template"
	"github.com/focalspan/focalspan/internal/model"
)

type testExtractor struct{}

func (testExtractor) Name() string                        { return "test" }
func (testExtractor) Supports(path, language string) bool { return language == "test" }
func (testExtractor) Extract(context.Context, model.SourceFile) (model.Extraction, error) {
	return model.Extraction{}, nil
}

func TestRegistrySelectsOnlySupportedExtractor(t *testing.T) {
	r := NewRegistry(testExtractor{})
	if _, ok := r.For("x.test", "test"); !ok {
		t.Fatal("expected extractor")
	}
	if _, ok := r.For("x.go", "go"); ok {
		t.Fatal("unexpected extractor")
	}
}

func TestRegistryPrefersPHPExtractorOverGeneric(t *testing.T) {
	r := NewRegistry(goast.NewExtractor(), php.NewExtractor(), generic.NewExtractor())
	got, ok := r.For("view.PHP", "php")
	if !ok || got.Name() != "php-structural" {
		t.Fatalf("extractor=%v ok=%v", got, ok)
	}
}

func TestRegistryPrefersTemplateExtractorOverGeneric(t *testing.T) {
	r := NewRegistry(goast.NewExtractor(), php.NewExtractor(), templateextract.NewExtractor(), generic.NewExtractor())
	got, ok := r.For("views/login.tpl", "smarty")
	if !ok || got.Name() != "template-structural" {
		t.Fatalf("extractor=%v ok=%v", got, ok)
	}
	got, ok = r.For("views/plain.tpl", "template")
	if !ok || got.Name() != "template-structural" {
		t.Fatalf("plain extractor=%v ok=%v", got, ok)
	}
}
