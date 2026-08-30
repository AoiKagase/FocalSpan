package lua

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "lua-structural" }

func (Extractor) Supports(path, language string) bool {
	return language == "lua" || strings.EqualFold(filepath.Ext(path), ".lua") || strings.EqualFold(filepath.Ext(path), ".rockspec")
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	tokens, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	return buildLua(ctx, file, tokens, diagnostics)
}
