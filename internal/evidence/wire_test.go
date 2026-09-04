package evidence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
)

func TestSummaryIsCanonicalAndSourceFree(t *testing.T) {
	packet := Packet{
		Schema:   SchemaContextV1,
		Mode:     ModeFocused,
		Budget:   Budget{Limit: 1200, Used: 934, Omitted: 2},
		Evidence: []Item{{ID: "e1", Handle: "secret_handle", Role: RoleTarget, Location: Location{Path: "secret/path.go", Lines: [2]int{1, 1}}, Fidelity: FidelityVerbatim, Source: "SECRET_SOURCE"}},
	}
	want := "items=1 tokens=934/1200 omitted=2"
	if got := Summary(packet); got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
	for _, forbidden := range []string{"SECRET_SOURCE", "secret/path.go", "secret_handle"} {
		if strings.Contains(Summary(packet), forbidden) {
			t.Fatalf("summary contains %q", forbidden)
		}
	}
}

func TestMeasureModelVisibleUsesCompactJSONAndSummary(t *testing.T) {
	estimator := budget.NewEstimator()
	packet := Packet{Schema: SchemaContextV1, Mode: ModeFocused, Budget: Budget{Limit: 256}, Evidence: []Item{}}
	compact, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	want := estimator.Estimate(string(compact) + "\n" + Summary(packet))
	if got := MeasureModelVisible(packet, estimator); got != want {
		t.Fatalf("MeasureModelVisible() = %d, want %d", got, want)
	}
}
