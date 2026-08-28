package search

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

type fakeStore struct{ results []model.RankedCandidate }

func (f fakeStore) SearchFTS(context.Context, string) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.results...), nil
}

type relationFakeStore struct {
	fakeStore
	related []model.RankedCandidate
}

func (f relationFakeStore) RelatedCandidates(context.Context, []string, string) ([]model.RankedCandidate, error) {
	return append([]model.RankedCandidate(nil), f.related...), nil
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
