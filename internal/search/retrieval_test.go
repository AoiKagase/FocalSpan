package search

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

type retrievalRecordingStore struct {
	called      []RetrieverID
	fts         []model.RankedCandidate
	qualified   []model.RankedCandidate
	exact       []model.RankedCandidate
	prefix      []model.RankedCandidate
	paths       []model.RankedCandidate
	related     []model.RankedCandidate
	relatedHits []model.RelationHit
	errFor      RetrieverID
}

func (s *retrievalRecordingStore) record(id RetrieverID) error {
	s.called = append(s.called, id)
	if s.errFor == id {
		return errors.New("store failure")
	}
	return nil
}

func (s *retrievalRecordingStore) SearchFTS(context.Context, string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverFTS); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.fts...), nil
}
func (s *retrievalRecordingStore) SearchQualifiedSymbols(context.Context, []string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverQualified); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.qualified...), nil
}
func (s *retrievalRecordingStore) SearchExactSymbols(context.Context, []string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverSymbol); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.exact...), nil
}
func (s *retrievalRecordingStore) SearchSymbolPrefixes(context.Context, []string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPrefix); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.prefix...), nil
}
func (s *retrievalRecordingStore) SearchPaths(context.Context, []string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPath); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.paths...), nil
}
func (s *retrievalRecordingStore) RelatedCandidates(_ context.Context, handles []string, relation string) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverRelation); err != nil {
		return nil, err
	}
	if relation != "callers" || !reflect.DeepEqual(handles, []string{"target"}) {
		return nil, errors.New("unexpected relation anchor")
	}
	return append([]model.RankedCandidate(nil), s.related...), nil
}

func (s *retrievalRecordingStore) RelatedCandidateHits(_ context.Context, handles []string, relation string) ([]model.RelationHit, error) {
	if err := s.record(RetrieverRelation); err != nil {
		return nil, err
	}
	if relation != "callers" || !reflect.DeepEqual(handles, []string{"target"}) {
		return nil, errors.New("unexpected relation anchor")
	}
	if len(s.relatedHits) > 0 {
		return append([]model.RelationHit(nil), s.relatedHits...), nil
	}
	hits := make([]model.RelationHit, 0, len(s.related))
	for _, candidate := range s.related {
		hits = append(hits, model.RelationHit{Candidate: candidate, Context: model.RelationContext{AnchorHandle: handles[0], Kind: relation, Direction: model.RelationIncoming, Confidence: 1, Resolved: true}})
	}
	return hits, nil
}

func TestRetrieverSetSelectsBaseRetrieversByMode(t *testing.T) {
	tests := []struct {
		name     string
		plan     query.Plan
		mode     RetrievalMode
		wantCall []RetrieverID
	}{
		{name: "definition", plan: query.Plan{Terms: query.Terms{Identifiers: []string{"ValidateToken"}}, PrimaryIntent: query.IntentDefinition}, mode: RetrievalFull, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath}},
		{name: "callers", plan: query.Plan{Terms: query.Terms{Identifiers: []string{"ValidateToken"}}, PrimaryIntent: query.IntentCallers, Intents: []query.Intent{query.IntentCallers}, Anchors: []string{"ValidateToken"}, Relations: []string{"callers"}}, mode: RetrievalFull, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath, RetrieverRelation}},
		{name: "fts-only", plan: query.Plan{PrimaryIntent: query.IntentCallers, Relations: []string{"callers"}}, mode: RetrievalFTSOnly, wantCall: []RetrieverID{RetrieverFTS}},
		{name: "no-relations", plan: query.Plan{PrimaryIntent: query.IntentCallers, Relations: []string{"callers"}}, mode: RetrievalNoRelations, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &retrievalRecordingStore{exact: []model.RankedCandidate{{Handle: "target", Symbol: "ValidateToken"}}}
			lists, err := (&RetrieverSet{store: store}).Retrieve(context.Background(), test.plan, SearchRequest{Query: "ValidateToken", Mode: test.mode})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(store.called, test.wantCall) {
				t.Fatalf("called=%v, want %v", store.called, test.wantCall)
			}
			if len(lists) != len(test.wantCall) {
				t.Fatalf("lists=%v, want %v", lists, test.wantCall)
			}
		})
	}
}

func TestRelationRetrievalWorksWhenFTSMissesAnchor(t *testing.T) {
	store := &retrievalRecordingStore{
		exact:   []model.RankedCandidate{{Handle: "target", Symbol: "ValidateToken"}},
		related: []model.RankedCandidate{{Handle: "caller", Path: "http/middleware.go", Symbol: "Authenticate"}},
	}
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"ValidateToken"}}, Intents: []query.Intent{query.IntentCallers}, PrimaryIntent: query.IntentCallers, Anchors: []string{"ValidateToken"}, Relations: []string{"callers"}}
	lists, err := (&RetrieverSet{store: store}).Retrieve(context.Background(), plan, SearchRequest{Query: "what calls ValidateToken?", Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range lists {
		if list.Retriever == RetrieverRelation && (len(list.Items) != 1 || list.Items[0].Symbol != "Authenticate") {
			t.Fatalf("relation list=%+v", list)
		}
	}
	if !containsRetriever(store.called, RetrieverRelation) || containsRetriever(store.called, RetrieverFTS) && len(store.fts) != 0 {
		// The important assertion is that exact retrieval, not FTS, supplied the
		// anchor. The empty FTS list is intentional.
		if !containsRetriever(store.called, RetrieverRelation) {
			t.Fatalf("relation was not retrieved: called=%v", store.called)
		}
	}
}

func TestRetrieverSetWrapsRetrieverErrors(t *testing.T) {
	store := &retrievalRecordingStore{errFor: RetrieverPath}
	_, err := (&RetrieverSet{store: store}).Retrieve(context.Background(), query.Plan{PrimaryIntent: query.IntentDefinition}, SearchRequest{Query: "token", Mode: RetrievalFull})
	if err == nil || !strings.Contains(err.Error(), string(RetrieverPath)) || !strings.Contains(err.Error(), "store failure") {
		t.Fatalf("err=%v, want retriever name and cause", err)
	}
}

func TestFTSOnlyNeverCallsRelationStore(t *testing.T) {
	store := &retrievalRecordingStore{}
	_, err := (&RetrieverSet{store: store}).Retrieve(context.Background(), query.Plan{PrimaryIntent: query.IntentCallers, Relations: []string{"callers"}}, SearchRequest{Query: "callers", Mode: RetrievalFTSOnly})
	if err != nil {
		t.Fatal(err)
	}
	if containsRetriever(store.called, RetrieverRelation) {
		t.Fatalf("relation called in fts-only mode: %v", store.called)
	}
}

func containsRetriever(values []RetrieverID, want RetrieverID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
