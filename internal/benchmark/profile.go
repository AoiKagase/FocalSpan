package benchmark

import (
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/search"
)

type Profile struct {
	Name          string
	RetrievalMode search.RetrievalMode
	Contract      string
	EvidenceMode  evidence.Mode
	Budgets       []int
	RunExpansion  bool
}

var DefaultProfiles = []Profile{
	{Name: "full-evidence-focused", RetrievalMode: search.RetrievalFull, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{1024, 2048, 4096}, RunExpansion: true},
	{Name: "fts-evidence-focused", RetrievalMode: search.RetrievalFTSOnly, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{2048}},
	{Name: "no-relations-evidence-focused", RetrievalMode: search.RetrievalNoRelations, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{2048}},
	{Name: "full-legacy-source", RetrievalMode: search.RetrievalFull, Contract: "legacy", EvidenceMode: evidence.ModeSource, Budgets: []int{2048}},
}

func ResolveProfiles(selection string) ([]Profile, error) {
	selection = strings.TrimSpace(selection)
	if selection == "" || selection == "default" {
		return cloneProfiles(DefaultProfiles), nil
	}
	available := make(map[string]Profile, len(DefaultProfiles))
	valid := make([]string, 0, len(DefaultProfiles))
	for _, profile := range DefaultProfiles {
		available[profile.Name] = profile
		valid = append(valid, profile.Name)
	}
	requested := strings.Split(selection, ",")
	result := make([]Profile, 0, len(requested))
	seen := map[string]struct{}{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		profile, exists := available[name]
		if !exists {
			return nil, fmt.Errorf("unknown benchmark profile %q; valid profiles: %s", name, strings.Join(valid, ", "))
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("duplicate benchmark profile %q", name)
		}
		seen[name] = struct{}{}
		result = append(result, cloneProfile(profile))
	}
	return result, nil
}

func cloneProfiles(profiles []Profile) []Profile {
	result := make([]Profile, len(profiles))
	for index, profile := range profiles {
		result[index] = cloneProfile(profile)
	}
	return result
}

func cloneProfile(profile Profile) Profile {
	profile.Budgets = append([]int(nil), profile.Budgets...)
	return profile
}
