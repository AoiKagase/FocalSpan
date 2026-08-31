package benchmark

import "github.com/focalspan/focalspan/internal/evidence"

type MatchResult struct {
	Matched int
	Total   int
	Items   []int
}

func MatchRequiredPaths(packet evidence.Packet, expected []string) MatchResult {
	result := MatchResult{Total: len(expected)}
	if !packetRelationsValid(packet) {
		return result
	}
	used := map[int]bool{}
	for _, path := range expected {
		for index, item := range packet.Evidence {
			if !used[index] && item.Location.Path == path {
				used[index] = true
				result.Matched++
				result.Items = append(result.Items, index)
				break
			}
		}
	}
	return result
}

func MatchRequiredSymbols(packet evidence.Packet, expected []SymbolExpectation) MatchResult {
	result := MatchResult{Total: len(expected)}
	if !packetRelationsValid(packet) {
		return result
	}
	used := map[int]bool{}
	for _, want := range expected {
		for index, item := range packet.Evidence {
			if used[index] || item.Location.Path != want.Path || item.Symbol != want.Name {
				continue
			}
			if want.Kind != "" && item.Kind != want.Kind {
				continue
			}
			if want.Role != "" && string(item.Role) != want.Role {
				continue
			}
			used[index] = true
			result.Matched++
			result.Items = append(result.Items, index)
			break
		}
	}
	return result
}

func packetRelationsValid(packet evidence.Packet) bool {
	ids := make(map[string]struct{}, len(packet.Evidence))
	for _, item := range packet.Evidence {
		ids[item.ID] = struct{}{}
	}
	for _, relation := range packet.Relations {
		if _, ok := ids[relation.From]; !ok {
			return false
		}
		if _, ok := ids[relation.To]; !ok {
			return false
		}
	}
	return true
}
