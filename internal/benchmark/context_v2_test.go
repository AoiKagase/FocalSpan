package benchmark

import (
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestContextV2ProfilesAreExplicitOnly(t *testing.T) {
	defaults, err := ResolveProfiles("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range defaults {
		if profile.ContextEncoding != "" {
			t.Fatalf("default profile opted into %q: %+v", profile.ContextEncoding, profile)
		}
	}
	v2Profiles, err := ResolveProfiles("full-evidence-focused-v2,fts-evidence-focused-v2,no-relations-evidence-focused-v2")
	if err != nil {
		t.Fatal(err)
	}
	if len(v2Profiles) != 3 {
		t.Fatalf("v2 profiles=%+v", v2Profiles)
	}
	for _, profile := range v2Profiles {
		if profile.ContextEncoding != evidence.SchemaContextV2 || profile.Contract != "evidence" {
			t.Fatalf("v2 profile=%+v", profile)
		}
	}
}

func TestMeasurePacketForProfileUsesNegotiatedContextV2Wire(t *testing.T) {
	packet := contextV2PacketForBenchmark()
	legacy := MeasurePacket(Case{ID: "v2"}, "v1", packet.Budget.Limit, packet, true, nil)
	profile := Profile{Name: "v2", Contract: "evidence", EvidenceMode: evidence.ModeFocused, ContextEncoding: evidence.SchemaContextV2}
	compact, measured, err := measurePacketForProfile(Case{ID: "v2"}, profile, packet.Budget.Limit, packet, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !compact {
		t.Fatal("expected compact representation")
	}
	if measured.WireTokens >= legacy.WireTokens || measured.WireBytes >= legacy.WireBytes || measured.PacketJSONBytes >= legacy.PacketJSONBytes {
		t.Fatalf("legacy=%+v compact=%+v", legacy, measured)
	}
	if measured.EvidenceTokens != legacy.EvidenceTokens || measured.RelationValid != legacy.RelationValid || measured.Deterministic != legacy.Deterministic {
		t.Fatalf("quality changed: legacy=%+v compact=%+v", legacy, measured)
	}
}

func contextV2PacketForBenchmark() evidence.Packet {
	return evidence.Packet{
		Schema: evidence.SchemaContextV1,
		Intent: "definition",
		Mode:   evidence.ModeFocused,
		Budget: evidence.Budget{Limit: 2048, Used: 900},
		Evidence: []evidence.Item{
			{ID: "e1", Handle: "target_handle", Role: evidence.RoleTarget, Location: evidence.Location{Path: "internal/evidence/context_v2.go", Lines: [2]int{1, 10}}, Language: "go", Kind: "function", Symbol: "EncodeContextV2", Fidelity: evidence.FidelityVerbatim, Why: []string{"exact_symbol"}, Source: "func EncodeContextV2(packet Packet) ([]byte, error) {\n\treturn encode(packet)\n}"},
			{ID: "e2", Handle: "caller_handle", Role: evidence.RoleCaller, Location: evidence.Location{Path: "internal/mcpserver/server.go", Lines: [2]int{1, 4}}, Language: "go", Kind: "function", Symbol: "negotiatedContextOutput", Fidelity: evidence.FidelitySignature, Signature: "func negotiatedContextOutput(packet evidence.Packet) any"},
		},
		Relations: []evidence.Edge{{From: "e2", To: "e1", Kind: "calls", Certainty: evidence.CertaintyExact}},
		Next:      []evidence.NextAction{{Handle: "target_handle", Relation: "callers", Reason: "inspect callers"}},
	}
}
