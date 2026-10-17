package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
)

func TestAppEngineBuildAndQueryEvidence(t *testing.T) {
	root := isolatedFixture(t, "authsample")
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
	root := isolatedFixture(t, "authsample")
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

func isolatedFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "repos", name)
	target := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".focalspan") {
			continue
		}
		from, to := filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := os.CopyFS(to, os.DirFS(from)); err != nil {
				t.Fatal(err)
			}
		} else {
			data, err := os.ReadFile(from)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(to, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return target
}
