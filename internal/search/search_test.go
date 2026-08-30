package search

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func TestSearchDetailedReturnsPlanAndOptionalSourceFreeTrace(t *testing.T) {
	store := fakeStore{results: []model.RankedCandidate{{Handle: "target", Path: "auth.go", Symbol: "ValidateToken", Kind: "function", Content: "secret source body"}}}
	s := New(store)
	withoutTrace, err := s.SearchDetailed(context.Background(), SearchRequest{Query: "ValidateToken", Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if withoutTrace.Plan.PrimaryIntent != query.IntentDefinition || withoutTrace.Trace != nil {
		t.Fatalf("without trace=%+v", withoutTrace)
	}
	withTrace, err := s.SearchDetailed(context.Background(), SearchRequest{Query: "ValidateToken", Mode: RetrievalFull, Trace: true})
	if err != nil {
		t.Fatal(err)
	}
	if withTrace.Trace == nil || len(withTrace.Trace.Candidates) == 0 || withTrace.Trace.Candidates[0].FinalScore <= 0 {
		t.Fatalf("trace=%+v", withTrace.Trace)
	}
	payload, err := json.Marshal(withTrace.Trace)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret source body") {
		t.Fatalf("trace contains source content: %s", payload)
	}
}

type fakeStore struct{ results []model.RankedCandidate }

func (f fakeStore) SearchFTS(context.Context, string, int) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

func (f fakeStore) SearchQualifiedSymbols(context.Context, []string, int) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

func (f fakeStore) SearchExactSymbols(context.Context, []string, int) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

func (f fakeStore) SearchSymbolPrefixes(context.Context, []string, int) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

func (f fakeStore) SearchPaths(context.Context, []string, int) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

func (f fakeStore) RelatedCandidates(context.Context, []string, string) ([]model.RankedCandidate, error) {
	return nil, nil
}

func (f fakeStore) RelatedCandidateHits(context.Context, []string, string) ([]model.RelationHit, error) {
	return nil, nil
}

type relationFakeStore struct {
	fakeStore
	related     []model.RankedCandidate
	relatedHits []model.RelationHit
}

func (f relationFakeStore) RelatedCandidates(context.Context, []string, string) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.related...), nil
}

func (f relationFakeStore) RelatedCandidateHits(context.Context, []string, string) ([]model.RelationHit, error) {
	if len(f.relatedHits) > 0 {
		return append([]model.RelationHit(nil), f.relatedHits...), nil
	}
	hits := make([]model.RelationHit, 0, len(f.related))
	for _, candidate := range f.related {
		hits = append(hits, model.RelationHit{Candidate: candidate, Context: model.RelationContext{AnchorHandle: "target", Kind: "callers", Direction: model.RelationIncoming, Confidence: 1, Resolved: true}})
	}
	return hits, nil
}

func TestSearchFiltersPathsAndChangedOnly(t *testing.T) {
	s := New(fakeStore{results: []model.RankedCandidate{
		{Handle: "a", Path: "auth/service.go", Symbol: "ValidateToken", StartLine: 10, EndLine: 20, Content: "expired token"},
		{Handle: "b", Path: "other.go", Symbol: "Report", StartLine: 1, EndLine: 4, Content: "expired token"},
	}})
	got, err := s.Search(context.Background(), SearchRequest{Query: "expired token", Paths: []string{"auth/"}, ChangedOnly: true, Changed: map[string][]LineRange{"auth/service.go": {{Start: 12, End: 12}}}})
	if err != nil || len(got) != 1 || got[0].Symbol != "ValidateToken" || !got[0].Changed {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestSearchAddsCallerRelationsForExplicitCallQuestion(t *testing.T) {
	s := New(relationFakeStore{
		fakeStore: fakeStore{results: []model.RankedCandidate{{Handle: "target", Path: "auth/service.go", Symbol: "ValidateToken", Content: "expired token"}}},
		related:   []model.RankedCandidate{{Handle: "caller", Path: "http/middleware.go", Symbol: "Authenticate", Content: "service.ValidateToken()"}},
	})
	got, err := s.Search(context.Background(), SearchRequest{Query: "what calls ValidateToken?"})
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range got {
		if candidate.Path == "http/middleware.go" && candidate.Relation == "callers" {
			return
		}
	}
	t.Fatalf("caller relation missing: %+v", got)
}

func TestSearchDetailedPreservesResolvedRelationProvenance(t *testing.T) {
	resolved := model.RelationContext{AnchorHandle: "target", Kind: "callers", Direction: model.RelationIncoming, Confidence: .95, Source: "go-ast", Resolved: true}
	lexical := model.RelationContext{AnchorHandle: "target", Kind: "callers", Direction: model.RelationIncoming, Confidence: .4, Source: "generic", Resolved: false}
	caller := model.RankedCandidate{Handle: "caller", Path: "http/middleware.go", Symbol: "Authenticate", Content: "service.ValidateToken()"}
	s := New(relationFakeStore{
		fakeStore: fakeStore{results: []model.RankedCandidate{{Handle: "target", Path: "auth/service.go", Symbol: "ValidateToken", Content: "expired token"}}},
		relatedHits: []model.RelationHit{
			{Candidate: caller, Context: lexical},
			{Candidate: caller, Context: resolved},
		},
	})

	result, err := s.SearchDetailed(context.Background(), SearchRequest{Query: "what calls ValidateToken?"})
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range result.Candidates {
		if candidate.Handle != "caller" {
			continue
		}
		if candidate.Relation != "callers" || candidate.RelationContext == nil {
			t.Fatalf("caller provenance absent: %+v", candidate)
		}
		if *candidate.RelationContext != resolved {
			t.Fatalf("context = %+v, want resolved %+v", *candidate.RelationContext, resolved)
		}
		return
	}
	t.Fatalf("caller absent: %+v", result.Candidates)
}

func TestSearchOnlyExpandsExactStructuralAnchor(t *testing.T) {
	s := New(relationFakeStore{
		fakeStore: fakeStore{results: []model.RankedCandidate{
			{Handle: "target", Path: "auth/service.go", Symbol: "ValidateToken", Content: "expired token"},
			{Handle: "noise", Path: "docs/readme.md", Symbol: "Readme", Content: "ValidateToken is documented here"},
		}},
		related: []model.RankedCandidate{{Handle: "caller", Path: "http/middleware.go", Symbol: "Authenticate"}},
	})
	got, err := s.Search(context.Background(), SearchRequest{Query: "what calls ValidateToken?"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, candidate := range got {
		if candidate.Relation == "callers" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("relation candidates=%d, got=%+v", count, got)
	}
}
