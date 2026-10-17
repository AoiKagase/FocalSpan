package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/indexlock"
	"github.com/focalspan/focalspan/internal/model"
)

type indexReadKey struct{}

func readResult[T any](s *Service, ctx context.Context, update bool, read func(context.Context) (T, error)) (T, error) {
	return readIndex(s, ctx, update, true, read)
}

func readIndex[T any](s *Service, ctx context.Context, update, requireReady bool, read func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx.Value(indexReadKey{}) == s {
		return read(ctx)
	}
	if update {
		if _, err := s.Index(ctx, false); err != nil {
			return zero, fmt.Errorf("auto-update index: %w", err)
		}
	}
	lease, err := indexlock.Acquire(ctx, filepath.Dir(s.Store.DBPath()), indexlock.Writer, indexlock.Shared)
	if err != nil {
		return zero, err
	}
	defer lease.Close()
	if requireReady {
		status, err := s.statusRead(ctx)
		if err != nil {
			return zero, err
		}
		if status.Stale {
			return zero, errors.New("index is stale or incomplete; update the index before reading evidence")
		}
	}
	return read(context.WithValue(ctx, indexReadKey{}, s))
}

func (s *Service) Status(ctx context.Context) (model.Status, error) {
	return readIndex(s, ctx, false, false, s.statusRead)
}
func (s *Service) Health(ctx context.Context) (model.HealthStatus, error) {
	return readIndex(s, ctx, false, false, s.healthRead)
}

func (s *Service) Query(ctx context.Context, req QueryRequest) (model.ContextBundle, error) {
	return readResult(s, ctx, !req.NoUpdate && s.Config.AutoUpdateBeforeQuery, func(ctx context.Context) (model.ContextBundle, error) { return s.queryRead(ctx, req) })
}
func (s *Service) Expand(ctx context.Context, h []string, r string, n int) (model.ContextBundle, error) {
	return readResult(s, ctx, false, func(ctx context.Context) (model.ContextBundle, error) { return s.expandRead(ctx, h, r, n) })
}
func (s *Service) Impact(ctx context.Context, b, h string, n int) (model.ContextBundle, error) {
	return readResult(s, ctx, s.Config.AutoUpdateBeforeQuery, func(ctx context.Context) (model.ContextBundle, error) { return s.impactRead(ctx, b, h, n) })
}
func (s *Service) queryEvidence(ctx context.Context, req EvidenceQueryRequest, trace bool) (AttributedEvidenceResult, error) {
	return readResult(s, ctx, !req.NoUpdate && s.Config.AutoUpdateBeforeQuery, func(ctx context.Context) (AttributedEvidenceResult, error) {
		return s.queryEvidenceRead(ctx, req, trace)
	})
}
func (s *Service) ExpandEvidence(ctx context.Context, req EvidenceExpandRequest) (evidence.CompileResult, error) {
	return readResult(s, ctx, false, func(ctx context.Context) (evidence.CompileResult, error) { return s.expandEvidenceRead(ctx, req) })
}
func (s *Service) ImpactEvidence(ctx context.Context, req EvidenceImpactRequest) (evidence.CompileResult, error) {
	return readResult(s, ctx, s.Config.AutoUpdateBeforeQuery, func(ctx context.Context) (evidence.CompileResult, error) { return s.impactEvidenceRead(ctx, req) })
}
