package benchmark

import (
	"fmt"

	"github.com/focalspan/focalspan/internal/evidence"
)

type ExpansionMetrics struct {
	RequiredPathRecall               float64
	RequiredSymbolRecall             float64
	ForbiddenViolations              int
	RelationValid                    int
	CumulativeWireTokens             int
	CumulativeWireTokensWithoutKnown int
	DeltaTokenRatio                  float64
	KnownResendCount                 int
}

func FindExpansionAnchor(packet evidence.Packet, want SymbolExpectation) (string, error) {
	var handles []string
	for _, item := range packet.Evidence {
		if item.Location.Path == want.Path && item.Symbol == want.Name && (want.Kind == "" || item.Kind == want.Kind) {
			handles = append(handles, item.Handle)
		}
	}
	if len(handles) == 0 {
		return "", fmt.Errorf("expansion anchor missing: %s:%s", want.Path, want.Name)
	}
	if len(handles) > 1 {
		return "", fmt.Errorf("expansion anchor ambiguous: %s:%s", want.Path, want.Name)
	}
	return handles[0], nil
}

func MeasureExpansion(initial, known, control evidence.Packet, want ExpandExpectation) ExpansionMetrics {
	paths := MatchRequiredPaths(known, want.RequiredPaths)
	symbols := MatchRequiredSymbols(known, want.RequiredSymbols)
	r := ExpansionMetrics{RequiredPathRecall: ratio(paths.Matched, paths.Total), RequiredSymbolRecall: ratio(symbols.Matched, symbols.Total), RelationValid: boolInt(packetRelationsValid(known)), CumulativeWireTokens: initial.Budget.Used + known.Budget.Used, CumulativeWireTokensWithoutKnown: initial.Budget.Used + control.Budget.Used}
	if r.CumulativeWireTokensWithoutKnown > 0 {
		r.DeltaTokenRatio = float64(r.CumulativeWireTokens) / float64(r.CumulativeWireTokensWithoutKnown)
	}
	initialHandles := map[string]bool{}
	for _, item := range initial.Evidence {
		initialHandles[item.Handle] = true
	}
	for _, item := range known.Evidence {
		if initialHandles[item.Handle] {
			r.KnownResendCount++
		}
		for _, path := range want.ForbiddenPaths {
			if item.Location.Path == path {
				r.ForbiddenViolations++
			}
		}
	}
	return r
}
