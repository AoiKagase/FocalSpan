package render

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestEvidenceJSONValidatesAndIndentsContract(t *testing.T) {
	packet := renderTestPacket()
	data, err := EvidenceJSON(packet)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "\n  \"schema\": \"focalspan.context.v1\"") || strings.Contains(text, "score") {
		t.Fatalf("unexpected JSON:\n%s", text)
	}
	packet.Schema = "bad"
	if _, err := EvidenceJSON(packet); err == nil {
		t.Fatal("invalid packet rendered")
	}
}

func TestEvidenceCompactRendersRolesRelationsGuidanceAndExactSource(t *testing.T) {
	packet := renderTestPacket()
	got := EvidenceCompact(packet)
	for _, want := range []string{
		"schema: focalspan.context.v1",
		"intent: callers",
		"mode: focused",
		"budget: 380/1200",
		"[e1 target] auth/service.go:44-51",
		"Service.ValidateToken",
		"func ValidateToken() error {\r\n\treturn nil\r\n}",
		"e2 -calls/exact-> e1",
		"callers sym_target (more_callers_omitted)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"score", "savings", "0.9"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("forbidden %q in:\n%s", forbidden, got)
		}
	}
}

func TestEvidenceCompactRendersOmittedSegmentOutsideSource(t *testing.T) {
	packet := renderTestPacket()
	packet.Evidence[0].Fidelity = evidence.FidelityExcerpt
	packet.Evidence[0].Source = ""
	packet.Evidence[0].Segments = []evidence.Segment{
		{Kind: evidence.SegmentSource, Lines: [2]int{44, 45}, Text: "line44\nline45\n"},
		{Kind: evidence.SegmentOmitted, Lines: [2]int{46, 49}},
		{Kind: evidence.SegmentSource, Lines: [2]int{50, 51}, Text: "line50\nline51\n"},
	}
	got := EvidenceCompact(packet)
	if !strings.Contains(got, "--- lines 46-49 omitted ---") || !strings.Contains(got, "line44\nline45\n") {
		t.Fatalf("excerpt not rendered:\n%s", got)
	}
}

func renderTestPacket() evidence.Packet {
	return evidence.Packet{
		Schema: evidence.SchemaContextV1,
		Intent: "callers",
		Mode:   evidence.ModeFocused,
		Budget: evidence.Budget{Limit: 1200, Used: 380},
		Evidence: []evidence.Item{
			{ID: "e1", Handle: "sym_target", Role: evidence.RoleTarget, Location: evidence.Location{Path: "auth/service.go", Lines: [2]int{44, 51}}, Language: "go", Kind: "method", Symbol: "Service.ValidateToken", Fidelity: evidence.FidelityVerbatim, Source: "func ValidateToken() error {\r\n\treturn nil\r\n}", Why: []string{"exact_symbol"}},
			{ID: "e2", Handle: "sym_caller", Role: evidence.RoleCaller, Location: evidence.Location{Path: "http/middleware.go", Lines: [2]int{21, 38}}, Language: "go", Kind: "function", Symbol: "Authenticate", Fidelity: evidence.FidelitySignature, Signature: "func Authenticate() error", Why: []string{"direct_caller"}},
		},
		Relations: []evidence.Edge{{From: "e2", To: "e1", Kind: "calls", Certainty: evidence.CertaintyExact}},
		Next:      []evidence.NextAction{{Handle: "sym_target", Relation: "callers", Reason: "more_callers_omitted"}},
	}
}
