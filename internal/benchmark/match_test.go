package benchmark

import (
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestMatchRequiredPathsExactAndUnique(t *testing.T) {
	packet := evidence.Packet{Evidence: []evidence.Item{{ID: "e1", Location: evidence.Location{Path: "auth/service.go"}}, {ID: "e2", Location: evidence.Location{Path: "auth/service.go"}}, {ID: "e3", Location: evidence.Location{Path: "Auth/Service.go"}}}}
	match := MatchRequiredPaths(packet, []string{"auth/service.go", "Auth/Service.go", "missing.go"})
	if match.Matched != 2 || match.Total != 3 {
		t.Fatalf("match = %+v", match)
	}
}

func TestMatchRequiredSymbolsUsesPathNameKindRole(t *testing.T) {
	packet := evidence.Packet{Evidence: []evidence.Item{{ID: "e1", Location: evidence.Location{Path: "auth/service.go"}, Symbol: "ValidateToken", Kind: "method", Role: evidence.RoleTarget, Fidelity: evidence.FidelitySynthetic}}}
	match := MatchRequiredSymbols(packet, []SymbolExpectation{{Path: "auth/service.go", Name: "ValidateToken", Kind: "method", Role: "target"}, {Path: "auth/service.go", Name: "validatetoken"}})
	if match.Matched != 1 || match.Total != 2 {
		t.Fatalf("match = %+v", match)
	}
}

func TestMatchIgnoresPacketWithDanglingRelation(t *testing.T) {
	packet := evidence.Packet{Evidence: []evidence.Item{{ID: "e1", Location: evidence.Location{Path: "auth/service.go"}}}, Relations: []evidence.Edge{{From: "e1", To: "missing", Kind: "calls", Certainty: evidence.CertaintyExact}}}
	if match := MatchRequiredPaths(packet, []string{"auth/service.go"}); match.Matched != 0 {
		t.Fatalf("match = %+v", match)
	}
}
