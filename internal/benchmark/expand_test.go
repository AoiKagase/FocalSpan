package benchmark

import (
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestFindExpansionAnchorExact(t *testing.T) {
	packet := evidence.Packet{Evidence: []evidence.Item{{Handle: "h1", Location: evidence.Location{Path: "a.go"}, Symbol: "Target", Kind: "method"}}}
	handle, err := FindExpansionAnchor(packet, SymbolExpectation{Path: "a.go", Name: "Target", Kind: "method"})
	if err != nil || handle != "h1" {
		t.Fatalf("handle=%q err=%v", handle, err)
	}
	if _, err := FindExpansionAnchor(packet, SymbolExpectation{Path: "a.go", Name: "target"}); err == nil {
		t.Fatal("expected missing")
	}
	packet.Evidence = append(packet.Evidence, packet.Evidence[0])
	if _, err := FindExpansionAnchor(packet, SymbolExpectation{Path: "a.go", Name: "Target"}); err == nil {
		t.Fatal("expected ambiguous")
	}
}

func TestMeasureExpansionKnownHandleDelta(t *testing.T) {
	initial := evidence.Packet{Budget: evidence.Budget{Used: 100}, Evidence: []evidence.Item{{Handle: "h1"}, {Handle: "h2"}}}
	known := evidence.Packet{Budget: evidence.Budget{Used: 40}, Evidence: []evidence.Item{{Handle: "h3", Location: evidence.Location{Path: "needed.go"}}}}
	control := evidence.Packet{Budget: evidence.Budget{Used: 80}, Evidence: []evidence.Item{{Handle: "h1"}, {Handle: "h3"}}}
	metrics := MeasureExpansion(initial, known, control, ExpandExpectation{RequiredPaths: []string{"needed.go"}})
	if metrics.CumulativeWireTokens != 140 || metrics.CumulativeWireTokensWithoutKnown != 180 || metrics.DeltaTokenRatio != float64(140)/180 || metrics.KnownResendCount != 0 || metrics.RequiredPathRecall != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}
