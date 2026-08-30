package projectmeta

import (
	"context"

	"github.com/focalspan/focalspan/internal/model"
)

type Fact struct {
	SourcePath string
	Kind       string
	Name       string
	Target     string
	Confidence float64
}

type Provider interface {
	Supports(path string) bool
	Parse(ctx context.Context, root string, file model.SourceFile) ([]Fact, []model.Diagnostic, error)
}
