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
	constructor []model.RankedCandidate
	exactHints  [][]string
	exactLimits []int
	prefix      []model.RankedCandidate
	paths       []model.RankedCandidate
	pathHints   [][]string
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
func (s *retrievalRecordingStore) SearchExactSymbols(_ context.Context, hints []string, limit int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverSymbol); err != nil {
		return nil, err
	}
	s.exactHints = append(s.exactHints, append([]string(nil), hints...))
	s.exactLimits = append(s.exactLimits, limit)
	if reflect.DeepEqual(hints, []string{"NewWithConfig"}) {
		return append([]model.RankedCandidate(nil), s.constructor...), nil
	}
	return append([]model.RankedCandidate(nil), s.exact...), nil
}
func (s *retrievalRecordingStore) SearchSymbolPrefixes(context.Context, []string, int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPrefix); err != nil {
		return nil, err
	}
	return append([]model.RankedCandidate(nil), s.prefix...), nil
}
func (s *retrievalRecordingStore) SearchPaths(_ context.Context, hints []string, _ int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPath); err != nil {
		return nil, err
	}
	s.pathHints = append(s.pathHints, append([]string(nil), hints...))
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

func TestRetrieverSetPassesOnlyExplicitPathTermsToPathSearch(t *testing.T) {
	store := &retrievalRecordingStore{}
	plan := query.Plan{
		Terms:         query.Terms{Paths: []string{"src/token.ts"}, Words: []string{"token", "module"}},
		PrimaryIntent: query.IntentDefinition,
	}

	if _, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Mode: RetrievalFull}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.pathHints, [][]string{{"src/token.ts"}}) {
		t.Fatalf("path hints=%v", store.pathHints)
	}
}

func TestStructuralConstructorHintsAreNarrow(t *testing.T) {
	positives := []string{
		"Where is the extractor registry assembled?",
		"Where are structural extractors wired?",
		"Rust extractorを既存serviceへ登録する場所",
	}
	for _, raw := range positives {
		if got := structuralConstructorHints(query.PlanQuery(raw), SearchRequest{}); !reflect.DeepEqual(got, []string{"NewWithConfig"}) {
			t.Fatalf("hints(%q)=%v", raw, got)
		}
	}
	for _, raw := range []string{"combine extracted files before metadata linking", "ValidateToken implementation"} {
		if got := structuralConstructorHints(query.PlanQuery(raw), SearchRequest{}); len(got) != 0 {
			t.Fatalf("unexpected hints(%q)=%v", raw, got)
		}
	}
	plan := query.PlanQuery("Where is the extractor registry assembled?")
	plan.Terms.Paths = []string{"internal/app/service.go"}
	if got := structuralConstructorHints(plan, SearchRequest{Paths: []string{"internal/app/service.go"}}); len(got) != 0 {
		t.Fatalf("explicit path produced hints=%v", got)
	}
}

func TestStructuralConstructorRetrieverIsBoundedAndTraceable(t *testing.T) {
	store := &retrievalRecordingStore{constructor: []model.RankedCandidate{{Handle: "constructor", Path: "internal/app/service.go", Symbol: "NewWithConfig"}}}
	lists, err := NewRetrieverSet(store).Retrieve(context.Background(), query.PlanQuery("Where is the extractor registry assembled?"), SearchRequest{Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.exactHints) != 2 || !reflect.DeepEqual(store.exactHints[1], []string{"NewWithConfig"}) || store.exactLimits[1] != 8 {
		t.Fatalf("exact calls hints=%v limits=%v", store.exactHints, store.exactLimits)
	}
	for _, list := range lists {
		if list.Retriever == RetrieverStructuralConstructor {
			if len(list.Items) != 1 || list.Items[0].Symbol != "NewWithConfig" || !candidateHasReasonCode(list.Items[0], "structural-constructor") {
				t.Fatalf("constructor list=%+v", list)
			}
			return
		}
	}
	t.Fatalf("constructor retriever absent: %+v", lists)
}

func candidateHasReasonCode(candidate model.RankedCandidate, want string) bool {
	for _, reason := range candidate.Reasons {
		if reason.Code == want {
			return true
		}
	}
	return false
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
