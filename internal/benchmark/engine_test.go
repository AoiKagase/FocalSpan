package benchmark

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/search"
)

func TestAppEngineBuildAndQueryEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos", "authsample")
	engine, err := NewAppEngineFactory().Open(root, search.RetrievalFull)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	measurement, err := engine.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Files == 0 || measurement.Symbols == 0 || measurement.Chunks == 0 {
		t.Fatalf("measurement = %+v", measurement)
	}
	packet, err := engine.QueryEvidence(context.Background(), app.EvidenceQueryRequest{Query: "ValidateToken の呼び出し元はどこですか", TokenBudget: 2048, Mode: evidence.ModeFocused, NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Schema != evidence.SchemaContextV1 || packet.Budget.Used > packet.Budget.Limit || len(packet.Evidence) == 0 {
		t.Fatalf("packet = %+v", packet)
	}
}

func TestAppEnginePropagatesCancellation(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos", "authsample")
	engine, err := NewAppEngineFactory().Open(root, search.RetrievalFull)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Build(ctx); err == nil {
		t.Fatal("expected cancellation")
	}
}
