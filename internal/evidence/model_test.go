package evidence_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestPacketJSONContract(t *testing.T) {
	packet := evidence.Packet{
		Schema: evidence.SchemaContextV1,
		Intent: "callers",
		Mode:   evidence.ModeFocused,
		Budget: evidence.Budget{Limit: 1200, Used: 380},
		Evidence: []evidence.Item{{
			ID:        "e1",
			Handle:    "sym_target",
			Role:      evidence.RoleTarget,
			Location:  evidence.Location{Path: "auth/service.go", Lines: [2]int{44, 51}},
			Language:  "go",
			Kind:      "method",
			Symbol:    "Service.ValidateToken",
			Signature: "func (s *Service) ValidateToken(token string) error",
			Fidelity:  evidence.FidelitySignature,
			Why:       []string{"exact_symbol"},
		}},
	}

	got, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"focalspan.context.v1","intent":"callers","mode":"focused","budget":{"limit":1200,"used":380},"evidence":[{"id":"e1","handle":"sym_target","role":"target","location":{"path":"auth/service.go","lines":[44,51]},"language":"go","kind":"method","symbol":"Service.ValidateToken","signature":"func (s *Service) ValidateToken(token string) error","fidelity":"signature","why":["exact_symbol"]}]}`
	if string(got) != want {
		t.Fatalf("packet JSON mismatch\n got: %s\nwant: %s", got, want)
	}
	for _, forbidden := range []string{"score", "weight", "token_savings", "baseline_tokens", "saved_tokens", "savings_ratio", "diagnostic_stage", "path_hits"} {
		if strings.Contains(string(got), `"`+forbidden+`"`) {
			t.Fatalf("packet contains forbidden key %q: %s", forbidden, got)
		}
	}
}

func TestPacketJSONAlwaysContainsEvidenceArray(t *testing.T) {
	got, err := json.Marshal(evidence.Packet{
		Schema: evidence.SchemaContextV1,
		Mode:   evidence.ModeFocused,
		Budget: evidence.Budget{Limit: 256, Used: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"evidence":[]`) {
		t.Fatalf("nil evidence must serialize as an array: %s", got)
	}
}

func TestAssignLocalIDsUsesPresentationOrder(t *testing.T) {
	items := []evidence.Item{{Handle: "sym_b"}, {Handle: "sym_a"}, {Handle: "sym_c"}}
	got := evidence.AssignLocalIDs(items)
	wantIDs := []string{"e1", "e2", "e3"}
	for i := range items {
		if items[i].ID != wantIDs[i] {
			t.Fatalf("item %d ID = %q, want %q", i, items[i].ID, wantIDs[i])
		}
		if got[items[i].Handle] != wantIDs[i] {
			t.Fatalf("map[%q] = %q, want %q", items[i].Handle, got[items[i].Handle], wantIDs[i])
		}
	}
}
