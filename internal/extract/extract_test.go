package extract

import (
	"context"
	"testing"

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
