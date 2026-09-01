package search

import (
	"math"
	"reflect"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestFuseRankedListsUsesWeightedReciprocalRankFusion(t *testing.T) {
	lists := []RankedList{
		{Retriever: RetrieverFTS, Items: []model.RankedCandidate{{Handle: "same", Path: "a.go", Symbol: "ValidateToken", Score: 99}}},
		{Retriever: RetrieverSymbol, Items: []model.RankedCandidate{{Handle: "same", Path: "a.go", Symbol: "ValidateToken", Score: 1}}},
	}
	got, traces := fuseRankedLists(lists, 10)
	if len(got) != 1 || got[0].Score != 0 {
		t.Fatalf("fused=%+v, want one zero-raw-score candidate", got)
	}
	want := retrieverWeights[RetrieverFTS]/(rrfK+1) + retrieverWeights[RetrieverSymbol]/(rrfK+1)
	if math.Abs(got[0].RetrievalScore-want) >= 1e-12 {
		t.Fatalf("retrieval score=%v, want %v", got[0].RetrievalScore, want)
	}
	if len(traces) != 1 || len(traces[0].Contributions) != 2 {
		t.Fatalf("traces=%+v", traces)
	}
	if traces[0].Contributions[0].Retriever != RetrieverFTS || traces[0].Contributions[1].Retriever != RetrieverSymbol {
		t.Fatalf("contribution order=%+v", traces[0].Contributions)
	}
}

func TestFuseRankedListsKeepsDifferentHandlesSeparate(t *testing.T) {
	lists := []RankedList{
		{Retriever: RetrieverSymbol, Items: []model.RankedCandidate{{Handle: "one", Symbol: "ValidateToken", Path: "a.go", Confidence: 1}}},
		{Retriever: RetrieverFTS, Items: []model.RankedCandidate{{Handle: "two", Symbol: "ValidateToken", Path: "a.go", Confidence: 1}}},
	}
	got, _ := fuseRankedLists(lists, 10)
	if len(got) != 2 || got[0].Handle == got[1].Handle {
		t.Fatalf("fused=%+v, different handles must remain separate", got)
	}
}

func TestFuseRankedListsIsStableAndCappedAfterFusion(t *testing.T) {
	lists := []RankedList{
		{Retriever: RetrieverFTS, Items: []model.RankedCandidate{
			{Handle: "b", Path: "b.go", Confidence: 1},
			{Handle: "a", Path: "a.go", Confidence: 1},
			{Handle: "c", Path: "c.go", Confidence: 1},
		}},
	}
	first, _ := fuseRankedLists(lists, 2)
	second, _ := fuseRankedLists([]RankedList{{Retriever: RetrieverFTS, Items: append([]model.RankedCandidate(nil), lists[0].Items...)}}, 2)
	if len(first) != 2 || !reflect.DeepEqual(first, second) {
		t.Fatalf("fused results are not deterministic/capped: first=%+v second=%+v", first, second)
	}
	if first[0].Path != "b.go" || first[1].Path != "a.go" {
		t.Fatalf("tie order=%+v", first)
	}
}
