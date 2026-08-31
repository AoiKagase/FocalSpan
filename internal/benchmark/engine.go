package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
)

type EngineFactory interface {
	Open(root string) (Engine, error)
}

type Engine interface {
	Build(ctx context.Context) (IndexMeasurement, error)
	QueryLegacy(ctx context.Context, req app.QueryRequest) (model.ContextBundle, error)
	QueryEvidence(ctx context.Context, req app.EvidenceQueryRequest) (evidence.Packet, error)
	ExpandEvidence(ctx context.Context, req app.EvidenceExpandRequest) (evidence.Packet, error)
	Close() error
}

type IndexMeasurement struct {
	Files         int
	Symbols       int
	Chunks        int
	Relations     int
	DatabaseBytes int64
	Duration      time.Duration
}

type appEngineFactory struct{}

func NewAppEngineFactory() EngineFactory { return appEngineFactory{} }

func (appEngineFactory) Open(root string) (Engine, error) {
	service, err := app.New(root)
	if err != nil {
		return nil, err
	}
	return &appEngine{service: service}, nil
}

type appEngine struct {
	service *app.Service
}

func (engine *appEngine) Build(ctx context.Context) (IndexMeasurement, error) {
	started := time.Now()
	if _, err := engine.service.Index(ctx, true); err != nil {
		return IndexMeasurement{}, err
	}
	measurement := IndexMeasurement{Duration: time.Since(started)}
	status, err := engine.service.Status(ctx)
	if err != nil {
		return IndexMeasurement{}, err
	}
	measurement.Files = status.FileCount
	measurement.Symbols = status.SymbolCount
	measurement.Chunks = status.ChunkCount
	measurement.Relations = status.RelationCount
	databasePath := filepath.Join(engine.service.Root, engine.service.Config.IndexDirectory, "index.db")
	if info, statErr := os.Stat(databasePath); statErr == nil {
		measurement.DatabaseBytes = info.Size()
	}
	return measurement, nil
}

func (engine *appEngine) QueryLegacy(ctx context.Context, req app.QueryRequest) (model.ContextBundle, error) {
	return engine.service.Query(ctx, req)
}

func (engine *appEngine) QueryEvidence(ctx context.Context, req app.EvidenceQueryRequest) (evidence.Packet, error) {
	result, err := engine.service.QueryEvidence(ctx, req)
	return result.Packet, err
}

func (engine *appEngine) ExpandEvidence(ctx context.Context, req app.EvidenceExpandRequest) (evidence.Packet, error) {
	result, err := engine.service.ExpandEvidence(ctx, req)
	return result.Packet, err
}

func (engine *appEngine) Close() error { return engine.service.Close() }
