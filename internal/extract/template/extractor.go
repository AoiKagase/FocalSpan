package template

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract/generic"

	"github.com/focalspan/focalspan/internal/model"
)

// Extractor provides best-effort structural extraction for Smarty and static
// templates. It never renders or executes a template.
type Extractor struct{ fallback generic.Extractor }

func NewExtractor() Extractor { return NewExtractorWithChunkOptions(80, 10) }

func NewExtractorWithChunkOptions(lines, overlap int) Extractor {
	return Extractor{generic.NewExtractorWithOptions(lines, overlap)}
}

func (Extractor) Name() string { return "template-structural" }

func (Extractor) Supports(_ string, language string) bool {
	return language == "smarty" || language == "template"
}

func (e Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	regions, scanDiagnostics, err := scan(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed, err := parse(ctx, file.Path, file.Content, regions, scanDiagnostics)
	if err != nil {
		return model.Extraction{}, err
	}
	return build(ctx, file, regions, parsed, e.fallback)
}
