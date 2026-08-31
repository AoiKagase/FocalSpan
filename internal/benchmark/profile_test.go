package benchmark

import (
	"reflect"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/search"
)

func TestProfilesDefault(t *testing.T) {
	got, err := ResolveProfiles("default")
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{Name: "full-evidence-focused", RetrievalMode: search.RetrievalFull, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{1024, 2048, 4096}, RunExpansion: true},
		{Name: "fts-evidence-focused", RetrievalMode: search.RetrievalFTSOnly, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{2048}},
		{Name: "no-relations-evidence-focused", RetrievalMode: search.RetrievalNoRelations, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{2048}},
		{Name: "full-legacy-source", RetrievalMode: search.RetrievalFull, Contract: "legacy", EvidenceMode: evidence.ModeSource, Budgets: []int{2048}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profiles = %#v, want %#v", got, want)
	}
}

func TestProfilesSelectExactNames(t *testing.T) {
	got, err := ResolveProfiles("full-legacy-source,full-evidence-focused")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "full-legacy-source" || got[1].Name != "full-evidence-focused" {
		t.Fatalf("profiles = %#v", got)
	}
	if _, err := ResolveProfiles("missing"); err == nil || !stringsContainAll(err.Error(), "missing", "full-evidence-focused") {
		t.Fatalf("unknown profile error = %v", err)
	}
}

func stringsContainAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
