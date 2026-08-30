package vb

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type VB6Extractor struct{}
type VBNetExtractor struct{}

func NewVB6Extractor() VB6Extractor     { return VB6Extractor{} }
func NewVBNetExtractor() VBNetExtractor { return VBNetExtractor{} }
func (VB6Extractor) Name() string       { return "vb6-structural" }
func (VBNetExtractor) Name() string     { return "vbnet-structural" }

func (VB6Extractor) Supports(path, language string) bool {
	if language == "vb6" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".frm", ".bas", ".cls", ".ctl":
		return language == "" || language == "vb6"
	}
	return false
}

func (VBNetExtractor) Supports(path, language string) bool {
	if language == "vbnet" {
		return true
	}
	return strings.EqualFold(filepath.Ext(path), ".vb") && (language == "" || language == "vbnet")
}

func (e VB6Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	return extractVB(ctx, file, false)
}

func (e VBNetExtractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	return extractVB(ctx, file, true)
}
