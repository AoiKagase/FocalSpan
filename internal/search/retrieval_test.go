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
	called          []RetrieverID
	fts             []model.RankedCandidate
	qualified       []model.RankedCandidate
	exact           []model.RankedCandidate
	prefix          []model.RankedCandidate
	paths           []model.RankedCandidate
	pathHints       [][]string
	related         []model.RankedCandidate
	relatedHits     []model.RelationHit
	fileSymbolPaths []string
	fileFTSPaths    []string
	filePathPaths   []string
	scoped          []model.RankedCandidate
	scopeCalled     []string
	scopePaths      [][]string
	errFor          RetrieverID
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
func (s *retrievalRecordingStore) SearchPaths(_ context.Context, hints []string, _ int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPath); err != nil {
		return nil, err
	}
	s.pathHints = append(s.pathHints, append([]string(nil), hints...))
	return append([]model.RankedCandidate(nil), s.paths...), nil
}
func (s *retrievalRecordingStore) SearchSymbolFiles(_ context.Context, _ []string, _ int) ([]string, error) {
	s.scopeCalled = append(s.scopeCalled, "symbol-files")
	return append([]string(nil), s.fileSymbolPaths...), nil
}
func (s *retrievalRecordingStore) SearchFTSFiles(_ context.Context, _ string, _ int) ([]string, error) {
	s.scopeCalled = append(s.scopeCalled, "fts-files")
	return append([]string(nil), s.fileFTSPaths...), nil
}
func (s *retrievalRecordingStore) SearchPathFiles(_ context.Context, _ []string, _ int) ([]string, error) {
	s.scopeCalled = append(s.scopeCalled, "path-files")
	return append([]string(nil), s.filePathPaths...), nil
}
func (s *retrievalRecordingStore) SearchCandidatesInFiles(_ context.Context, paths, _ []string, _ string, _, _ int) ([]model.RankedCandidate, error) {
	s.scopeCalled = append(s.scopeCalled, "scoped-candidates")
	s.scopePaths = append(s.scopePaths, append([]string(nil), paths...))
	if len(paths) == 0 {
		return nil, errors.New("expected scoped paths")
	}
	return append([]model.RankedCandidate(nil), s.scoped...), nil
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

func TestRetrieverSetAggregatesFileScopeForDefinitionQueries(t *testing.T) {
	store := &retrievalRecordingStore{
		fileSymbolPaths: []string{"internal/indexer/indexer.go"},
		fileFTSPaths:    []string{"internal/indexer/indexer.go"},
		filePathPaths:   []string{},
		scoped:          []model.RankedCandidate{{Handle: "scoped", Path: "internal/indexer/indexer.go", Symbol: "Run"}},
	}
	plan := query.Plan{Terms: query.Terms{Words: []string{"indexer", "flow"}, Identifiers: []string{"Run"}}, PrimaryIntent: query.IntentDefinition}
	lists, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: "where is Run in the indexer flow", Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.scopeCalled, []string{"symbol-files", "fts-files", "path-files", "scoped-candidates"}) {
		t.Fatalf("scope calls=%v", store.scopeCalled)
	}
	var found bool
	for _, list := range lists {
		if list.Retriever == RetrieverPathScopedAggregate {
			found = true
			if len(list.Items) != 1 || list.Items[0].Handle != "scoped" {
				t.Fatalf("scoped list=%+v", list.Items)
			}
		}
	}
	if !found {
		t.Fatalf("lists=%v, want path-scope-aggregate", lists)
	}
}

func TestRetrieverSetFiltersAggregatedScopeToExplicitPaths(t *testing.T) {
	store := &retrievalRecordingStore{
		fileSymbolPaths: []string{"other.go", "src/allowed.go"},
		fileFTSPaths:    []string{"other.go"},
		filePathPaths:   []string{"src/allowed.go"},
		scoped:          []model.RankedCandidate{{Handle: "allowed", Path: "src/allowed.go", Symbol: "Run"}},
	}
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"Run"}}, PrimaryIntent: query.IntentDefinition}
	if _, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: "Run", Paths: []string{"src/allowed.go"}, Mode: RetrievalFull}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.scopePaths, [][]string{{"src/allowed.go"}}) {
		t.Fatalf("scope paths=%v, want explicit path only", store.scopePaths)
	}
}

func TestRetrieverSetSkipsFileScopeForRelationsAndFTSOnly(t *testing.T) {
	for _, mode := range []RetrievalMode{RetrievalFTSOnly, RetrievalNoRelations} {
		store := &retrievalRecordingStore{}
		plan := query.Plan{Terms: query.Terms{Identifiers: []string{"Run"}}, PrimaryIntent: query.IntentCallers, Relations: []string{"callers"}}
		if _, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: "what calls Run", Mode: mode}); err != nil {
			t.Fatal(err)
		}
		if len(store.scopeCalled) != 0 {
			t.Fatalf("mode=%s scope calls=%v", mode, store.scopeCalled)
		}
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
