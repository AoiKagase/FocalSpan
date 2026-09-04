package benchmark

import (
	"encoding/json"
	"strings"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"testing"
)

func TestMetricPacketByteBreakdownIsExactAndSourceFree(t *testing.T) {
	packet := evidence.Packet{
		Schema: evidence.SchemaContextV1, Intent: "definition", Mode: evidence.ModeFocused,
		Budget:      evidence.Budget{Limit: 1000, Used: 300},
		Evidence:    []evidence.Item{{ID: "e1", Handle: "handle-one", Role: evidence.RoleTarget, Location: evidence.Location{Path: "a.go", Lines: [2]int{1, 2}}, Fidelity: evidence.FidelitySignature, Signature: "func Target()"}},
		Limitations: []string{"source_reduced_to_signature"},
		Next:        []evidence.NextAction{{Handle: "handle-one", Relation: "self", Reason: "source_body_omitted"}},
	}
	result := MeasurePacket(Case{ID: "bytes"}, "p", 1000, packet, true, nil)
	payload, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	wantWire := len(payload) + 1 + len(evidence.Summary(packet))
	if result.WireBytes != wantWire || result.PacketJSONBytes != len(payload) || result.SummaryBytes != len(evidence.Summary(packet))+1 {
		t.Fatalf("byte metrics=%+v want wire/json/summary=%d/%d/%d", result, wantWire, len(payload), len(evidence.Summary(packet))+1)
	}
	if result.EvidenceContentBytes+result.GuidanceBytes+result.EnvelopeMetadataBytes+result.SummaryBytes != result.WireBytes {
		t.Fatalf("byte breakdown does not sum: %+v", result)
	}
	if result.SignatureItems != 1 || result.VerbatimItems != 0 || result.ExcerptItems != 0 || result.SyntheticItems != 0 {
		t.Fatalf("fidelity counts=%+v", result)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "func Target()") {
		t.Fatalf("source content leaked into metrics: %s", encoded)
	}
}

func TestMetricResultFromPacket(t *testing.T) {
	packet := evidence.Packet{Intent: "definition", Budget: evidence.Budget{Limit: 1000, Used: 400}, Evidence: []evidence.Item{{ID: "e1", Role: evidence.RoleTarget, Location: evidence.Location{Path: "a.go"}, Symbol: "Target", Source: "same"}, {ID: "e2", Location: evidence.Location{Path: "b.go"}, Source: "same"}}}
	result := MeasurePacket(Case{ID: "c", ExpectedIntent: "definition", RequiredPaths: []string{"a.go", "missing.go"}, OptionalPaths: []string{}, ForbiddenPaths: []string{"b.go"}, RequiredSymbols: []SymbolExpectation{{Path: "a.go", Name: "Target", Role: "target"}}}, "p", 1000, packet, true, nil)
	if result.TargetRank != 1 || result.ReciprocalRank != 1 || result.RequiredPathRecall != .5 || result.OptionalPathRecall != 1 || result.RequiredSymbolRecall != 1 || result.IntentCorrect != 1 || result.BudgetCompliant != 1 || result.ForbiddenViolations != 1 || result.DuplicateSourceRatio <= 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAggregateQuality(t *testing.T) {
	aggregate := AggregateResults([]QualityResult{
		{Profile: "p", Budget: 100, TargetRank: 1, RequiredPathRecall: 1, RequiredSymbolRecall: 1, IntentCorrect: 1, RoleAccuracy: 1, BudgetCompliant: 1, Deterministic: 1, WireTokens: 80, MetadataOverheadRatio: .25, DuplicateSourceRatio: .5, ChangedPathRecall: 1},
		{Profile: "p", Budget: 100, TargetRank: 0, RequiredPathRecall: 0, RequiredSymbolRecall: 0, IntentCorrect: 0, RoleAccuracy: 0, BudgetCompliant: 1, Deterministic: 1, WireTokens: 40, MetadataOverheadRatio: .75, DuplicateSourceRatio: 0, ChangedPathRecall: 0},
	})
	if len(aggregate.Groups) != 1 {
		t.Fatalf("aggregate = %+v", aggregate)
	}
	group := aggregate.Groups[0]
	if group.HitAt1 != .5 || group.MedianRequiredPathRecall != .5 || group.IntentAccuracy != .5 || group.RoleAccuracy != .5 || group.MedianWireTokens != 60 || group.MedianMetadataOverhead != .5 || group.MedianDuplicateSourceRatio != .25 || group.MedianChangedPathRecall != .5 {
		t.Fatalf("aggregate = %+v", aggregate)
	}
}

func TestMetricEvidenceTokensUseProductEstimator(t *testing.T) {
	text := strings.Repeat("日本語Evidence", 20)
	packet := evidence.Packet{Budget: evidence.Budget{Limit: 1000, Used: 500}, Evidence: []evidence.Item{{ID: "e1", Location: evidence.Location{Path: "a.go"}, Source: text}}}
	result := MeasurePacket(Case{ID: "unicode"}, "p", 1000, packet, true, nil)
	want := budget.NewEstimator().Estimate(text)
	if result.EvidenceTokens != want {
		t.Fatalf("evidence tokens=%d, want product estimator=%d", result.EvidenceTokens, want)
	}
}

func TestMetricMissDiagnosticsContainLabelsButNoSource(t *testing.T) {
	packet := evidence.Packet{Budget: evidence.Budget{Limit: 1000, Used: 100}, Evidence: []evidence.Item{{ID: "e1", Role: evidence.RoleCaller, Location: evidence.Location{Path: "selected.go"}, Symbol: "Selected", Source: "secret body"}}}
	result := MeasurePacket(Case{ID: "miss", RequiredPaths: []string{"required.go"}, RequiredSymbols: []SymbolExpectation{{Path: "required.go", Name: "Required"}}}, "p", 1000, packet, true, nil)
	if len(result.Misses) != 2 {
		t.Fatalf("misses=%+v", result.Misses)
	}
	encoded := result.Misses[0].Selected[0]
	if encoded.Path != "selected.go" || encoded.Symbol != "Selected" || encoded.Role != "caller" {
		t.Fatalf("selected label=%+v", encoded)
	}
	if strings.Contains(result.Misses[0].ExpectedPath+result.Misses[0].ExpectedSymbol+encoded.Path+encoded.Symbol+encoded.Role, "secret body") {
		t.Fatalf("source leaked in diagnostic: %+v", result.Misses)
	}
}
