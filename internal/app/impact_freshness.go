package app

import (
	"context"
	"fmt"

	"github.com/focalspan/focalspan/internal/gitx"
	"github.com/focalspan/focalspan/internal/repository"
)

func (s *Service) validateImpactSources(ctx context.Context, affected map[string][]gitx.LineRange) error {
	if len(affected) == 0 {
		return nil
	}
	files, diagnostics, err := repository.NewScanner(s.Root, s.Config).Scan(ctx)
	if err != nil {
		return fmt.Errorf("verify impact source: %w", err)
	}
	for _, diagnostic := range diagnostics {
		if len(affected[diagnostic.FilePath]) > 0 {
			return fmt.Errorf("cannot verify impact source %q (%s); update the index before retrying", diagnostic.FilePath, diagnostic.Code)
		}
	}
	for _, file := range files {
		if len(affected[file.Path]) == 0 {
			continue
		}
		hash, found, err := s.Store.FileHash(ctx, file.Path)
		if err != nil {
			return fmt.Errorf("verify impact hash: %w", err)
		}
		if !found || hash != file.SHA256 {
			return fmt.Errorf("index is stale for impact source %q; update the index before retrying", file.Path)
		}
	}
	return nil
}
