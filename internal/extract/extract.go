package extract

import (
	"context"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor interface {
	Name() string
	Supports(path string, language string) bool
	Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error)
}

type Registry struct{ extractors []Extractor }

func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: append([]Extractor(nil), extractors...)}
}

func (r *Registry) For(path, language string) (Extractor, bool) {
	for _, extractor := range r.extractors {
		if extractor.Supports(path, language) {
			return extractor, true
		}
	}
	return nil, false
}

func (r *Registry) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	extractor, ok := r.For(file.Path, file.Language)
	if !ok {
		return model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "info", Code: "no_extractor", Message: "no extractor supports this file"}}}, nil
	}
	return extractor.Extract(ctx, file)
}
