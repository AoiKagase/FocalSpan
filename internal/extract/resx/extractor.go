package resx

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "resx-structural" }
func (Extractor) Supports(path, language string) bool {
	if language == "dotnet-resource" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".resx" || ext == ".settings"
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".resx" || ext == ".settings"
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	parsed, err := scan(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	return build(ctx, file, parsed), nil
}
