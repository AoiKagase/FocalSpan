package xaml

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "xaml-structural" }
func (Extractor) Supports(path, language string) bool {
	return language == "xaml" || strings.EqualFold(filepath.Ext(path), ".xaml")
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
