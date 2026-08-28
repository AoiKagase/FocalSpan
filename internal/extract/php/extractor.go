package php

import (
	"context"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor { return Extractor{} }

func (Extractor) Name() string { return "php-structural" }

func (Extractor) Supports(path, language string) bool {
	return language == "php"
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
	parsed, err := parsePHP(ctx, file, tokens, diagnostics)
	if err != nil {
		return model.Extraction{}, err
	}
	result, err := buildExtraction(ctx, file, parsed)
	if err != nil {
		return model.Extraction{}, err
	}
	if len(result.Diagnostics) > 0 && !hasDiagnostic(result.Diagnostics, "php_partial_extraction") && strings.TrimSpace(string(file.Content)) != "" {
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{FilePath: file.Path, Level: "warning", Code: "php_partial_extraction", Message: "PHP extraction recovered from malformed source; some declarations or chunks may be incomplete"})
	}
	return result, nil
}

func hasDiagnostic(diagnostics []model.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
