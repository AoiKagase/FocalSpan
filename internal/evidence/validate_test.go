package evidence_test

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func validPacket() evidence.Packet {
	return evidence.Packet{
		Schema: evidence.SchemaContextV1,
		Mode:   evidence.ModeFocused,
		Budget: evidence.Budget{Limit: 1200, Used: 300},
		Evidence: []evidence.Item{{
			ID:        "e1",
			Handle:    "sym_target",
			Role:      evidence.RoleTarget,
			Location:  evidence.Location{Path: "auth/service.go", Lines: [2]int{44, 51}},
			Signature: "func ValidateToken(token string) error",
			Fidelity:  evidence.FidelitySignature,
		}},
	}
}

func TestValidateAcceptsValidPackets(t *testing.T) {
	packets := []evidence.Packet{
		validPacket(),
		{Schema: evidence.SchemaContextV1, Mode: evidence.ModeOutline, Budget: evidence.Budget{Limit: 256}, Evidence: []evidence.Item{}},
		{Schema: evidence.SchemaContextV1, Mode: evidence.ModeSource, Budget: evidence.Budget{Limit: 256}, Evidence: []evidence.Item{}},
	}
	for _, packet := range packets {
		if err := evidence.Validate(packet); err != nil {
			t.Fatalf("Validate(%+v): %v", packet, err)
		}
	}
}

func TestValidateRejectsContractViolations(t *testing.T) {
	tests := []struct {
		name string
		edit func(*evidence.Packet)
		want string
	}{
		{"wrong schema", func(p *evidence.Packet) { p.Schema = "other" }, "schema"},
		{"unsupported mode", func(p *evidence.Packet) { p.Mode = "wide" }, "mode"},
		{"non sequential ID", func(p *evidence.Packet) { p.Evidence[0].ID = "e2" }, "sequential"},
		{"blank handle", func(p *evidence.Packet) { p.Evidence[0].Handle = "" }, "handle"},
		{"duplicate handle", func(p *evidence.Packet) { p.Evidence = append(p.Evidence, p.Evidence[0]); p.Evidence[1].ID = "e2" }, "duplicate handle"},
		{"absolute path", func(p *evidence.Packet) { p.Evidence[0].Location.Path = "C:/repo/auth.go" }, "relative"},
		{"rooted path", func(p *evidence.Packet) { p.Evidence[0].Location.Path = "/repo/auth.go" }, "relative"},
		{"backslash path", func(p *evidence.Packet) { p.Evidence[0].Location.Path = `auth\service.go` }, "slash-normalized"},
		{"zero line", func(p *evidence.Packet) { p.Evidence[0].Location.Lines = [2]int{0, 1} }, "line range"},
		{"reversed line", func(p *evidence.Packet) { p.Evidence[0].Location.Lines = [2]int{5, 4} }, "line range"},
		{"unknown role", func(p *evidence.Packet) { p.Evidence[0].Role = "owner" }, "role"},
		{"unknown fidelity", func(p *evidence.Packet) { p.Evidence[0].Fidelity = "summary" }, "fidelity"},
		{"verbatim missing source", func(p *evidence.Packet) {
			p.Evidence[0].Fidelity = evidence.FidelityVerbatim
			p.Evidence[0].Signature = ""
		}, "verbatim"},
		{"verbatim with segments", func(p *evidence.Packet) {
			p.Evidence[0].Fidelity = evidence.FidelityVerbatim
			p.Evidence[0].Signature = ""
			p.Evidence[0].Source = "source"
			p.Evidence[0].Segments = []evidence.Segment{{Kind: evidence.SegmentSource, Lines: [2]int{44, 44}, Text: "source"}}
		}, "verbatim"},
		{"excerpt without source segment", func(p *evidence.Packet) {
			p.Evidence[0].Fidelity = evidence.FidelityExcerpt
			p.Evidence[0].Signature = ""
			p.Evidence[0].Segments = []evidence.Segment{{Kind: evidence.SegmentOmitted, Lines: [2]int{44, 51}}}
		}, "source segment"},
		{"excerpt with source", func(p *evidence.Packet) {
			p.Evidence[0].Fidelity = evidence.FidelityExcerpt
			p.Evidence[0].Signature = ""
			p.Evidence[0].Source = "source"
			p.Evidence[0].Segments = []evidence.Segment{{Kind: evidence.SegmentSource, Lines: [2]int{44, 44}, Text: "source"}}
		}, "excerpt"},
		{"signature with source", func(p *evidence.Packet) { p.Evidence[0].Source = "source" }, "signature"},
		{"synthetic without outline", func(p *evidence.Packet) {
			p.Evidence[0].Fidelity = evidence.FidelitySynthetic
			p.Evidence[0].Signature = ""
		}, "synthetic"},
		{"edge missing local ID", func(p *evidence.Packet) {
			p.Relations = []evidence.Edge{{From: "e1", To: "e2", Kind: "calls", Certainty: evidence.CertaintyExact}}
		}, "missing local ID"},
		{"self edge", func(p *evidence.Packet) {
			p.Relations = []evidence.Edge{{From: "e1", To: "e1", Kind: "calls", Certainty: evidence.CertaintyExact}}
		}, "self-edge"},
		{"duplicate edge", func(p *evidence.Packet) {
			e := evidence.Edge{From: "e1", To: "e1", Kind: "self", Certainty: evidence.CertaintyExact}
			p.Relations = []evidence.Edge{e, e}
		}, "duplicate edge"},
		{"unknown certainty", func(p *evidence.Packet) {
			p.Relations = []evidence.Edge{{From: "e1", To: "e1", Kind: "self", Certainty: "certain"}}
		}, "certainty"},
		{"too many why", func(p *evidence.Packet) { p.Evidence[0].Why = []string{"a", "b", "c", "d", "e"} }, "why"},
		{"too many limitations", func(p *evidence.Packet) { p.Limitations = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"} }, "limitations"},
		{"too many next", func(p *evidence.Packet) { p.Next = make([]evidence.NextAction, 5) }, "next"},
		{"used over limit", func(p *evidence.Packet) { p.Budget.Used = 1201 }, "budget"},
		{"negative skipped known", func(p *evidence.Packet) { p.SkippedKnown = -1 }, "skipped_known"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := validPacket()
			tt.edit(&packet)
			err := evidence.Validate(packet)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
