package app

import (
	"context"
	"fmt"

	"github.com/focalspan/focalspan/internal/gitx"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/repository"
)

type impactSourceCheck struct {
	hashes      map[string]string
	diagnostics map[string]model.Diagnostic
	checked     map[string]bool
}

func (s *Service) validateImpactSources(ctx context.Context, affected map[string][]gitx.LineRange) (*impactSourceCheck, error) {
	check := &impactSourceCheck{hashes: map[string]string{}, diagnostics: map[string]model.Diagnostic{}, checked: map[string]bool{}}
	if len(affected) == 0 {
		return check, nil
	}
	files, diagnostics, err := repository.NewScanner(s.Root, s.Config).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("verify impact source: %w", err)
	}
	for _, diagnostic := range diagnostics {
		check.diagnostics[diagnostic.FilePath] = diagnostic
	}
	for _, file := range files {
		check.hashes[file.Path] = file.SHA256
	}
	// Explicit refs can contain current-source ranges for files that have
	// since disappeared from this worktree. Never retrieve their old rows.
	for path := range affected {
		valid, err := check.verify(ctx, s, path)
		if err != nil {
			return nil, err
		}
		if !valid {
			delete(affected, path)
		}
	}
	return check, nil
}

func (check *impactSourceCheck) verify(ctx context.Context, s *Service, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if valid, ok := check.checked[path]; ok {
		return valid, nil
	}
	if diagnostic, ok := check.diagnostics[path]; ok {
		return false, fmt.Errorf("cannot verify impact source %q (%s); update the index before retrying", path, diagnostic.Code)
	}
	current, exists := check.hashes[path]
	if !exists {
		check.checked[path] = false
		return false, nil
	}
	hash, found, err := s.Store.FileHash(ctx, path)
	if err != nil {
		return false, fmt.Errorf("verify impact hash: %w", err)
	}
	if !found || hash != current {
		return false, fmt.Errorf("index is stale for impact source %q; update the index before retrying", path)
	}
	check.checked[path] = true
	return true, nil
}
