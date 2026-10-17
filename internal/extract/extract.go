package extract

import (
	"bytes"
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
	// Keep parser input BOM-free as it was before raw file bytes were retained
	// by the scanner. Source positions must still refer to the original file.
	offset := 0
	if bytes.HasPrefix(file.Content, []byte{0xef, 0xbb, 0xbf}) {
		offset = 3
		file.Content = file.Content[offset:]
	}
	result, err := extractor.Extract(ctx, file)
	if offset != 0 {
		for index := range result.Symbols {
			symbol := &result.Symbols[index]
			if symbol.EndByte > symbol.StartByte {
				symbol.StartByte += offset
				symbol.EndByte += offset
			}
		}
		for index := range result.Chunks {
			chunk := &result.Chunks[index]
			if chunk.EndByte > chunk.StartByte {
				chunk.StartByte += offset
				chunk.EndByte += offset
			}
		}
	}
	return result, err
}
