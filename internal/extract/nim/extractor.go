package nim

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "nim-structural" }
func (Extractor) Supports(path, language string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if language == "nim" {
		return ext == ".nim" || ext == ".nims"
	}
	return ext == ".nim" || ext == ".nims"
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	_, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed := parseNim(file)
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	for index := range parsed.Diagnostics {
		parsed.Diagnostics[index].FilePath = file.Path
	}
	return buildNim(ctx, file, parsed, append(diagnostics, parsed.Diagnostics...)), nil
}
