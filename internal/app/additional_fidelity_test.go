package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonFixesPreserveEvidenceSourceAndWireBudget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := "\ufefftext = '''\r\ndef Phantom():\r\n    pass\r\n'''\r\ndef 認証():\r\n    text = 'Phantom()'\r\n    # Phantom()\r\n    return True\r\n"
	writeAppFile(t, filepath.Join(root, "sample.py"), source)
	cfg := config.Default()
	cfg.AutoUpdateBeforeQuery = false
	s, err := NewWithConfig(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	phantom, err := s.Store.SearchExactSymbols(ctx, []string{"Phantom"}, 10)
	if err != nil || len(phantom) != 0 {
		t.Fatalf("phantom stored: %+v %v", phantom, err)
	}
	lines := strings.SplitAfter(source, "\n")
	foundSource := false
	for _, limit := range []int{256, 512, 2048, 4000} {
		for _, mode := range []evidence.Mode{evidence.ModeFocused, evidence.ModeSource, evidence.ModeOutline} {
			result, err := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "認証", Paths: []string{"sample.py"}, TokenBudget: limit, Mode: mode, NoUpdate: true})
			if err != nil {
				t.Fatal(err)
			}
			packet := result.Packet
			if err := evidence.Validate(packet); err != nil {
				t.Fatal(err)
			}
			if measured := evidence.MeasureModelVisible(packet, budget.NewEstimator()); measured > packet.Budget.Limit || measured != packet.Budget.Used {
				t.Fatalf("wire usage=%d packet=%+v", measured, packet.Budget)
			}
			for _, item := range packet.Evidence {
				start, end := item.Location.Lines[0], item.Location.Lines[1]
				if start < 1 || end > len(lines) {
					t.Fatalf("invalid location %+v", item.Location)
				}
				original := strings.Join(lines[start-1:end], "")
				if item.Source != "" {
					if !strings.Contains(original, item.Source) {
						t.Fatalf("source changed at %+v: %q", item.Location, item.Source)
					}
					if item.Symbol == "認証" {
						foundSource = true
					}
				}
				for _, seg := range item.Segments {
					if seg.Kind == evidence.SegmentSource {
						span := strings.Join(lines[seg.Lines[0]-1:seg.Lines[1]], "")
						if !strings.Contains(span, seg.Text) {
							t.Fatalf("segment changed: %+v", seg)
						}
					}
				}
			}
			if _, err := evidence.EncodeContextV2(packet, budget.NewEstimator()); err != nil {
				t.Fatal(err)
			}
		}
	}
	targets, err := s.Store.SearchExactSymbols(ctx, []string{"認証"}, 10)
	if err != nil || len(targets) == 0 {
		t.Fatalf("Unicode symbol not stored: %v %+v", err, targets)
	}
	expanded, err := s.ExpandEvidence(ctx, EvidenceExpandRequest{Handles: []string{targets[0].Handle}, Relation: "self", TokenBudget: 4000})
	if err != nil {
		t.Fatal(err)
	}
	if err := evidence.Validate(expanded.Packet); err != nil {
		t.Fatal(err)
	}
	if used := evidence.MeasureModelVisible(expanded.Packet, budget.NewEstimator()); used != expanded.Packet.Budget.Used || used > 4000 {
		t.Fatalf("expanded wire %d", used)
	}
	for _, item := range expanded.Packet.Evidence {
		if item.Source != "" && item.Symbol == "認証" {
			span := strings.Join(lines[item.Location.Lines[0]-1:item.Location.Lines[1]], "")
			// Chunk spans end before the final line terminator.
			if item.Source != strings.TrimSuffix(span, "\r\n") {
				t.Fatalf("expanded source mismatch: %q != %q", item.Source, span)
			}
			foundSource = true
		}
	}
	if _, err := evidence.EncodeContextV2(expanded.Packet, budget.NewEstimator()); err != nil {
		t.Fatal(err)
	}
	if !foundSource {
		t.Fatal("no verbatim source returned at sufficient budget")
	}
}
