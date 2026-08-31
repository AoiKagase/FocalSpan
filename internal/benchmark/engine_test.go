package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
)

func TestAppEngineBuildAndQueryEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos", "authsample")
	engine, err := NewAppEngineFactory().Open(root)
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
	attributed, err := engine.QueryEvidenceAttributed(context.Background(), app.EvidenceQueryRequest{Query: "ValidateToken の呼び出し元はどこですか", TokenBudget: 2048, Mode: evidence.ModeFocused, NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	packetJSON, _ := json.Marshal(packet)
	attributedJSON, _ := json.Marshal(attributed.Compile.Packet)
	if !bytes.Equal(packetJSON, attributedJSON) || len(attributed.Trace.Retrieved) == 0 || len(attributed.Trace.Candidates) == 0 {
		t.Fatalf("attributed query diverged: packet=%s trace=%+v", attributedJSON, attributed.Trace)
	}
}

func TestAppEnginePropagatesCancellation(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos", "authsample")
	engine, err := NewAppEngineFactory().Open(root)
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
