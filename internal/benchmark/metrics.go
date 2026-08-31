package benchmark

import (
	"sort"

	"github.com/focalspan/focalspan/internal/evidence"
)

type QualityResult struct {
	CaseID                string   `json:"case_id"`
	Profile               string   `json:"profile"`
	Budget                int      `json:"budget"`
	TargetRank            int      `json:"target_rank"`
	ReciprocalRank        float64  `json:"reciprocal_rank"`
	RequiredPathRecall    float64  `json:"required_path_recall"`
	OptionalPathRecall    float64  `json:"optional_path_recall"`
	RequiredSymbolRecall  float64  `json:"required_symbol_recall"`
	IntentCorrect         int      `json:"intent_correct"`
	RoleAccuracy          float64  `json:"role_accuracy"`
	RelationValid         int      `json:"relation_valid"`
	BudgetCompliant       int      `json:"budget_compliant"`
	Deterministic         int      `json:"deterministic"`
	ForbiddenViolations   int      `json:"forbidden_violations"`
	WireTokens            int      `json:"wire_tokens"`
	EvidenceTokens        int      `json:"evidence_tokens"`
	MetadataOverheadRatio float64  `json:"metadata_overhead_ratio"`
	DuplicateSourceRatio  float64  `json:"duplicate_source_ratio"`
	ChangedPathRecall     float64  `json:"changed_path_recall"`
	FailureCodes          []string `json:"failure_codes,omitempty"`
}

type PerformanceResult struct {
	CaseID        string  `json:"case_id"`
	Profile       string  `json:"profile"`
	Budget        int     `json:"budget"`
	SnapshotMS    int64   `json:"snapshot_ms"`
	IndexMS       int64   `json:"index_ms"`
	QueryMS       []int64 `json:"query_ms"`
	DatabaseBytes int64   `json:"database_bytes"`
	Files         int     `json:"files"`
	Symbols       int     `json:"symbols"`
	Chunks        int     `json:"chunks"`
	Relations     int     `json:"relations"`
}

type AggregateQuality struct {
	Groups []AggregateGroup `json:"groups,omitempty"`
}
type AggregateGroup struct {
	Profile                    string  `json:"profile"`
	Budget                     int     `json:"budget"`
	Cases                      int     `json:"cases"`
	InvalidCases               int     `json:"invalid_cases"`
	HitAt1                     float64 `json:"hit_at_1"`
	HitAt3                     float64 `json:"hit_at_3"`
	HitAt5                     float64 `json:"hit_at_5"`
	MeanReciprocalRank         float64 `json:"mean_reciprocal_rank"`
	MedianRequiredPathRecall   float64 `json:"median_required_path_recall"`
	MedianRequiredSymbolRecall float64 `json:"median_required_symbol_recall"`
	BudgetCompliance           float64 `json:"budget_compliance"`
	DeterministicOutput        float64 `json:"deterministic_output"`
	ForbiddenViolations        int     `json:"forbidden_violations"`
}

func MeasurePacket(c Case, profile string, budget int, packet evidence.Packet, deterministic bool, changedPaths []string) QualityResult {
	r := QualityResult{CaseID: c.ID, Profile: profile, Budget: budget, RelationValid: 1, OptionalPathRecall: 1}
	paths := MatchRequiredPaths(packet, c.RequiredPaths)
	r.RequiredPathRecall = ratio(paths.Matched, paths.Total)
	if len(c.OptionalPaths) > 0 {
		m := MatchRequiredPaths(packet, c.OptionalPaths)
		r.OptionalPathRecall = ratio(m.Matched, m.Total)
	}
	symbols := MatchRequiredSymbols(packet, c.RequiredSymbols)
	r.RequiredSymbolRecall = ratio(symbols.Matched, symbols.Total)
	if c.ExpectedIntent == "" || packet.Intent == c.ExpectedIntent {
		r.IntentCorrect = 1
	}
	if deterministic {
		r.Deterministic = 1
	}
	if packet.Budget.Used <= packet.Budget.Limit && packet.Budget.Used <= budget {
		r.BudgetCompliant = 1
	}
	if !packetRelationsValid(packet) {
		r.RelationValid = 0
	}
	for i, item := range packet.Evidence {
		if item.Role == evidence.RoleTarget && r.TargetRank == 0 {
			r.TargetRank = i + 1
		}
		for _, p := range c.ForbiddenPaths {
			if item.Location.Path == p {
				r.ForbiddenViolations++
			}
		}
	}
	if r.TargetRank > 0 {
		r.ReciprocalRank = 1 / float64(r.TargetRank)
	}
	r.RoleAccuracy = roleAccuracy(packet, c.RequiredSymbols)
	r.WireTokens = packet.Budget.Used
	evidenceBytes, totalSource, duplicate := 0, 0, 0
	seen := map[string]bool{}
	for _, item := range packet.Evidence {
		text := item.Signature + item.Source + item.Outline
		for _, s := range item.Segments {
			text += s.Text
		}
		evidenceBytes += len(text)
		if item.Source != "" {
			totalSource += len(item.Source)
			if seen[item.Source] {
				duplicate += len(item.Source)
			}
			seen[item.Source] = true
		}
	}
	r.EvidenceTokens = (evidenceBytes + 3) / 4
	if r.WireTokens > 0 {
		overhead := float64(r.WireTokens-r.EvidenceTokens) / float64(r.WireTokens)
		if overhead < 0 {
			overhead = 0
		}
		if overhead > 1 {
			overhead = 1
		}
		r.MetadataOverheadRatio = overhead
	}
	if totalSource > 0 {
		r.DuplicateSourceRatio = float64(duplicate) / float64(totalSource)
	}
	if len(changedPaths) > 0 {
		selected := 0
		for _, p := range changedPaths {
			for _, item := range packet.Evidence {
				if item.Location.Path == p {
					selected++
					break
				}
			}
		}
		r.ChangedPathRecall = ratio(selected, len(changedPaths))
	}
	return r
}

func roleAccuracy(packet evidence.Packet, expected []SymbolExpectation) float64 {
	total, matched := 0, 0
	for _, want := range expected {
		if want.Role == "" {
			continue
		}
		total++
		for _, item := range packet.Evidence {
			if item.Location.Path == want.Path && item.Symbol == want.Name && string(item.Role) == want.Role {
				matched++
				break
			}
		}
	}
	return ratio(matched, total)
}
func ratio(matched, total int) float64 {
	if total == 0 {
		return 1
	}
	return float64(matched) / float64(total)
}

func AggregateResults(results []QualityResult) AggregateQuality {
	type key struct {
		p string
		b int
	}
	groups := map[key][]QualityResult{}
	var keys []key
	for _, r := range results {
		k := key{r.Profile, r.Budget}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], r)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].p == keys[j].p {
			return keys[i].b < keys[j].b
		}
		return keys[i].p < keys[j].p
	})
	result := AggregateQuality{}
	for _, k := range keys {
		values := groups[k]
		g := AggregateGroup{Profile: k.p, Budget: k.b, Cases: len(values)}
		var recalls, symbols []float64
		for _, r := range values {
			if r.TargetRank == 1 {
				g.HitAt1++
			}
			if r.TargetRank > 0 && r.TargetRank <= 3 {
				g.HitAt3++
			}
			if r.TargetRank > 0 && r.TargetRank <= 5 {
				g.HitAt5++
			}
			g.MeanReciprocalRank += r.ReciprocalRank
			recalls = append(recalls, r.RequiredPathRecall)
			symbols = append(symbols, r.RequiredSymbolRecall)
			g.BudgetCompliance += float64(r.BudgetCompliant)
			g.DeterministicOutput += float64(r.Deterministic)
			g.ForbiddenViolations += r.ForbiddenViolations
		}
		n := float64(len(values))
		g.HitAt1 /= n
		g.HitAt3 /= n
		g.HitAt5 /= n
		g.MeanReciprocalRank /= n
		g.BudgetCompliance /= n
		g.DeterministicOutput /= n
		g.MedianRequiredPathRecall = median(recalls)
		g.MedianRequiredSymbolRecall = median(symbols)
		result.Groups = append(result.Groups, g)
	}
	return result
}
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	m := len(v) / 2
	if len(v)%2 == 0 {
		return (v[m-1] + v[m]) / 2
	}
	return v[m]
}
