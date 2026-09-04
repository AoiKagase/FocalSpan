package evidence_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
)

func contextV2Packet() evidence.Packet {
	return evidence.Packet{
		Schema:       evidence.SchemaContextV1,
		Revision:     "abc123",
		Intent:       "definition",
		Mode:         evidence.ModeFocused,
		Budget:       evidence.Budget{Limit: 4096, Used: 1800, Truncated: true, Omitted: 2},
		SkippedKnown: 3,
		Evidence: []evidence.Item{
			{ID: "e1", Handle: "h1", Role: evidence.RoleTarget, Location: evidence.Location{Path: "auth/service.go", Lines: [2]int{10, 14}}, Language: "go", Kind: "function", Symbol: "Validate", Fidelity: evidence.FidelityVerbatim, Why: []string{"exact_symbol"}, Source: "func Validate() error {\n\treturn nil\n}"},
			{ID: "e2", Handle: "h2", Role: evidence.RoleImplementation, Location: evidence.Location{Path: "auth/service.go", Lines: [2]int{20, 35}}, Kind: "method", Symbol: "Service.Check", Fidelity: evidence.FidelityExcerpt, Segments: []evidence.Segment{{Kind: evidence.SegmentSource, Lines: [2]int{20, 22}, Text: "func (s Service) Check() error {"}, {Kind: evidence.SegmentOmitted, Lines: [2]int{23, 34}}, {Kind: evidence.SegmentSource, Lines: [2]int{35, 35}, Text: "}"}}},
			{ID: "e3", Handle: "h3", Role: evidence.RoleCaller, Location: evidence.Location{Path: "api/handler.go", Lines: [2]int{40, 44}}, Language: "go", Kind: "function", Symbol: "Handle", Fidelity: evidence.FidelitySignature, Signature: "func Handle(w http.ResponseWriter)"},
			{ID: "e4", Handle: "h4", Role: evidence.RoleContext, Location: evidence.Location{Path: "docs/auth.md", Lines: [2]int{1, 8}}, Language: "markdown", Fidelity: evidence.FidelitySynthetic, Outline: "Authentication flow"},
		},
		Relations: []evidence.Edge{
			{From: "e3", To: "e1", Kind: "calls", Certainty: evidence.CertaintyExact},
			{From: "e2", To: "e1", Kind: "implements", Certainty: evidence.CertaintyScoped},
		},
		Limitations: []string{"dynamic calls unresolved"},
		Next:        []evidence.NextAction{{Handle: "h1", Relation: "callers", Reason: "inspect callers"}},
	}
}

func TestContextV2RoundTripPreservesCanonicalPacket(t *testing.T) {
	packet := contextV2Packet()
	raw, err := evidence.EncodeContextV2(packet, budget.NewEstimator())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"schema":"focalspan.context.v2"`)) {
		t.Fatalf("v2=%s", raw)
	}
	decoded, err := evidence.DecodeContextV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	packet.Budget.Used = decoded.Budget.Used
	if !reflect.DeepEqual(decoded, packet) {
		t.Fatalf("decoded mismatch\n got: %#v\nwant: %#v", decoded, packet)
	}
	if err := evidence.Validate(decoded); err != nil {
		t.Fatalf("decoded validation: %v", err)
	}
}

func TestContextV2EncodingIsDeterministicAndSmaller(t *testing.T) {
	packet := contextV2Packet()
	first, err := evidence.EncodeContextV2(packet, budget.NewEstimator())
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.EncodeContextV2(packet, budget.NewEstimator())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("nondeterministic\nfirst=%s\nsecond=%s", first, second)
	}
	v1, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) >= len(v1) {
		t.Fatalf("v2 bytes=%d, v1 bytes=%d", len(first), len(v1))
	}
}

func TestDecodeContextV2RejectsInvalidTables(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"wrong schema", `{"schema":"other","m":"focused","b":[256,0,false,0],"e":[]}`, "schema"},
		{"missing evidence", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0]}`, "evidence"},
		{"bad budget row", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0],"e":[]}`, "budget"},
		{"bad evidence row", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0],"p":["a.go"],"e":[["h"]]}`, "evidence row"},
		{"path index", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0],"p":["a.go"],"e":[["h","target",2,1,1,"signature","func F()",{}]]}`, "path index"},
		{"relation index", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0],"p":["a.go"],"e":[["h","target",0,1,1,"signature","func F()",{}]],"x":[[0,3,"calls","exact"]]}`, "relation"},
		{"unknown attribute", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0],"p":["a.go"],"e":[["h","target",0,1,1,"signature","func F()",{"z":1}]]}`, "attribute"},
		{"unknown top field", `{"schema":"focalspan.context.v2","m":"focused","b":[256,0,false,0],"e":[],"debug":true}`, "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evidence.DecodeContextV2([]byte(tt.raw))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
}
