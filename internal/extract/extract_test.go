package extract

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/php"
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

func TestRegistryPrefersFirstClassLanguageExtractorsOverGeneric(t *testing.T) {
	r := NewRegistry(goast.NewExtractor(), php.NewExtractor(), cpp.NewExtractor(), csharp.NewExtractor(), jsts.NewExtractor(), generic.NewExtractor())
	tests := []struct{ path, language, want string }{{"x.cpp", "cpp", "cpp-structural"}, {"x.cs", "csharp", "csharp-structural"}, {"x.tsx", "typescript", "jsts-structural"}}
	for _, tt := range tests {
		got, ok := r.For(tt.path, tt.language)
		if !ok || got.Name() != tt.want {
			t.Fatalf("For(%q,%q)=%v ok=%v, want %s", tt.path, tt.language, got, ok, tt.want)
		}
	}
}
