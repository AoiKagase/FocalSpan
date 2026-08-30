package zig

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor  { return Extractor{} }
func (Extractor) Name() string { return "zig-structural" }
func (Extractor) Supports(path, language string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zig") || language == "zig" && strings.EqualFold(filepath.Ext(path), ".zig")
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	_, diagnostics, err := Lex(ctx, file.Content)
	if err != nil {
		return model.Extraction{}, err
	}
	parsed := parseZig(file)
	for index := range diagnostics {
		diagnostics[index].FilePath = file.Path
	}
	for index := range parsed.Diagnostics {
		parsed.Diagnostics[index].FilePath = file.Path
	}
	return buildZig(ctx, file, parsed, append(diagnostics, parsed.Diagnostics...)), nil
}
