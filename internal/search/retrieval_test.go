package search

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

type retrievalRecordingStore struct {
	called      []RetrieverID
	operations  []string
	fts         []model.RankedCandidate
	qualified   []model.RankedCandidate
	exact       []model.RankedCandidate
	prefix      []model.RankedCandidate
	paths       []model.RankedCandidate
	pathHints   [][]string
	filePaths   []string
	fileHints   [][]string
	scoped      []model.RankedCandidate
	scopedPaths [][]string
	symbolHints [][]string
	ftsQueries  []string
	perPathCaps []int
	totalCaps   []int
	related     []model.RankedCandidate
	relatedHits []model.RelationHit
	errFor      RetrieverID
}

func (s *retrievalRecordingStore) record(id RetrieverID) error {
	s.called = append(s.called, id)
	s.operations = append(s.operations, string(id))
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
func (s *retrievalRecordingStore) SearchFilePaths(_ context.Context, hints []string, _ int) ([]string, error) {
	s.operations = append(s.operations, "file-probe")
	s.fileHints = append(s.fileHints, append([]string(nil), hints...))
	return append([]string(nil), s.filePaths...), nil
}
func (s *retrievalRecordingStore) SearchSymbolsInPaths(_ context.Context, paths []string, hints []string, ftsQuery string, perPathLimit, limit int) ([]model.RankedCandidate, error) {
	if err := s.record(RetrieverPathScopedSymbol); err != nil {
		return nil, err
	}
	s.scopedPaths = append(s.scopedPaths, append([]string(nil), paths...))
	s.symbolHints = append(s.symbolHints, append([]string(nil), hints...))
	s.ftsQueries = append(s.ftsQueries, ftsQuery)
	s.perPathCaps = append(s.perPathCaps, perPathLimit)
	s.totalCaps = append(s.totalCaps, limit)
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
		wantOps  []string
		wantList int
	}{
		{name: "definition", plan: query.Plan{Terms: query.Terms{Identifiers: []string{"ValidateToken"}}, PrimaryIntent: query.IntentDefinition}, mode: RetrievalFull, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath, RetrieverPathScopedSymbol}, wantOps: []string{"qualified-symbol", "symbol-exact", "symbol-prefix", "fts", "path", "file-probe", "path-scoped-symbol"}, wantList: 5},
		{name: "callers", plan: query.Plan{Terms: query.Terms{Identifiers: []string{"ValidateToken"}}, PrimaryIntent: query.IntentCallers, Intents: []query.Intent{query.IntentCallers}, Anchors: []string{"ValidateToken"}, Relations: []string{"callers"}}, mode: RetrievalFull, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath, RetrieverPathScopedSymbol, RetrieverRelation}, wantOps: []string{"qualified-symbol", "symbol-exact", "symbol-prefix", "fts", "path", "path-scoped-symbol", "relation"}, wantList: 6},
		{name: "fts-only", plan: query.Plan{PrimaryIntent: query.IntentCallers, Relations: []string{"callers"}}, mode: RetrievalFTSOnly, wantCall: []RetrieverID{RetrieverFTS}, wantOps: []string{"fts"}, wantList: 1},
		{name: "no-relations", plan: query.Plan{PrimaryIntent: query.IntentDefinition}, mode: RetrievalNoRelations, wantCall: []RetrieverID{RetrieverQualified, RetrieverSymbol, RetrieverPrefix, RetrieverFTS, RetrieverPath, RetrieverPathScopedSymbol}, wantOps: []string{"qualified-symbol", "symbol-exact", "symbol-prefix", "fts", "path", "file-probe", "path-scoped-symbol"}, wantList: 5},
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
			if !reflect.DeepEqual(store.operations, test.wantOps) {
				t.Fatalf("operations=%v, want %v", store.operations, test.wantOps)
			}
			if len(lists) != test.wantList {
				t.Fatalf("lists=%v, want count %d", lists, test.wantList)
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

func TestPathScopedRetrieverFindsPHPRunFromLexicalFileProbe(t *testing.T) {
	store := &retrievalRecordingStore{
		filePaths: []string{"internal/indexer/indexer.go"},
		scoped:    []model.RankedCandidate{{Handle: "run", Path: "internal/indexer/indexer.go", Symbol: "Run"}},
	}
	plan := query.PlanQuery("PHPの.inc抽出結果をindexへ保存する流れはどこですか")
	lists, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: plan.RawQuery, Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.fileHints) != 1 || !containsFold(store.fileHints[0], "index") {
		t.Fatalf("file probe hints=%v", store.fileHints)
	}
	if len(store.scopedPaths) != 1 || !slices.Equal(store.scopedPaths[0], []string{"internal/indexer/indexer.go"}) || store.perPathCaps[0] != 8 || store.totalCaps[0] != 40 {
		t.Fatalf("scoped call paths=%v perPath=%v total=%v", store.scopedPaths, store.perPathCaps, store.totalCaps)
	}
	if !listContainsSymbol(lists, RetrieverPathScopedSymbol, "Run") {
		t.Fatalf("lists=%+v", lists)
	}
}

func TestPathScopedRetrieverGeneratesMCPNamingVariant(t *testing.T) {
	store := &retrievalRecordingStore{
		fts:    []model.RankedCandidate{{Handle: "other", Path: "internal/mcpserver/server.go", Symbol: "Serve"}},
		scoped: []model.RankedCandidate{{Handle: "code-context", Path: "internal/mcpserver/server.go", Symbol: "codeContext"}},
	}
	plan := query.PlanQuery("code_contextの応答を組み立てるhandlerはどこですか")
	lists, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: plan.RawQuery, Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.symbolHints) != 1 || !slices.Contains(store.symbolHints[0], "codeContext") {
		t.Fatalf("symbol hints=%v", store.symbolHints)
	}
	if !listContainsSymbol(lists, RetrieverPathScopedSymbol, "codeContext") {
		t.Fatalf("lists=%+v", lists)
	}
}

func TestRelationRetrievalUsesScopedAnchorWithoutLexicalProbe(t *testing.T) {
	store := &retrievalRecordingStore{
		fts:       []model.RankedCandidate{{Handle: "fts-seed", Path: "auth/service.go", Symbol: "Other"}},
		filePaths: []string{"noise/generated.go"},
		scoped: []model.RankedCandidate{
			{Handle: "noise", Path: "auth/service.go", Symbol: "Other"},
			{Handle: "target", Path: "auth/service.go", Symbol: "ValidateToken"},
		},
		related: []model.RankedCandidate{{Handle: "caller", Path: "http.go", Symbol: "Authenticate"}},
	}
	plan := query.PlanQuery("what calls ValidateToken?")
	lists, err := NewRetrieverSet(store).Retrieve(context.Background(), plan, SearchRequest{Query: plan.RawQuery, Mode: RetrievalFull})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.fileHints) != 0 || slices.Contains(store.operations, "file-probe") {
		t.Fatalf("relation query launched lexical probe: hints=%v operations=%v", store.fileHints, store.operations)
	}
	if len(store.scopedPaths) != 1 || !slices.Equal(store.scopedPaths[0], []string{"auth/service.go"}) {
		t.Fatalf("relation scopes=%v", store.scopedPaths)
	}
	if !listContainsSymbol(lists, RetrieverRelation, "Authenticate") {
		t.Fatalf("relation lists=%+v", lists)
	}
}

func TestPathScopeHintsSelectsBoundedNonNavigationTerms(t *testing.T) {
	tests := []struct {
		query    string
		includes []string
		excludes []string
	}{
		{query: "PHPの.inc抽出結果をindexへ保存する流れはどこですか", includes: []string{"index"}, excludes: []string{"どこ", "流れ", "処理"}},
		{query: "Where is the extractor registry assembled before adding C++?", includes: []string{"extractor", "registry"}, excludes: []string{"where", "before", "adding"}},
		{query: "code_contextの応答を組み立てるhandlerはどこですか", includes: []string{"code_context", "handler"}, excludes: []string{"どこ"}},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			plan := query.PlanQuery(test.query)
			first := pathScopeHints(plan)
			second := pathScopeHints(plan)
			if len(first) > 8 || !slices.Equal(first, second) {
				t.Fatalf("hints=%v stable=%v", first, slices.Equal(first, second))
			}
			for _, want := range test.includes {
				if !containsFold(first, want) {
					t.Fatalf("hints=%v missing %q", first, want)
				}
			}
			for _, excluded := range test.excludes {
				if containsFold(first, excluded) {
					t.Fatalf("hints=%v unexpectedly contain %q", first, excluded)
				}
			}
		})
	}

	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"Index", "index", "Registry", "registry", "Handler", "handler", "ExtraOne", "ExtraTwo", "ExtraThree", "ExtraFour", "ExtraFive"}}}
	got := pathScopeHints(plan)
	if len(got) != 8 || got[0] != "Index" || got[1] != "Registry" {
		t.Fatalf("deduplicated capped hints=%v", got)
	}
}

func TestIdentifierStyleVariantsUsesExactNonFuzzySets(t *testing.T) {
	tests := []struct {
		value string
		want  []string
	}{
		{value: "code_context", want: []string{"code_context", "codeContext", "CodeContext"}},
		{value: "Service.ValidateToken", want: []string{"Service.ValidateToken", "ValidateToken"}},
		{value: "internal/search/search.go", want: []string{"internal/search/search.go", "search.go", "search"}},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := identifierStyleVariants(test.value); !slices.Equal(got, test.want) {
				t.Fatalf("variants=%v, want %v", got, test.want)
			}
		})
	}
}

func TestPathScopedSymbolHintsPreservesSourcePriorityAndCaps(t *testing.T) {
	plan := query.Plan{
		Anchors: []string{"Service.ValidateToken"},
		Terms: query.Terms{
			Symbols:     []string{"code_context"},
			Identifiers: []string{"Handler"},
			Words:       []string{"search", "where", "before", "adding"},
		},
	}
	got := pathScopedSymbolHints(plan)
	wantPrefix := []string{"Service.ValidateToken", "ValidateToken", "code_context", "codeContext", "CodeContext", "Handler", "search"}
	if len(got) < len(wantPrefix) || !slices.Equal(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("symbol hints=%v, want prefix %v", got, wantPrefix)
	}
	for _, excluded := range []string{"where", "before", "adding"} {
		if containsFold(got, excluded) {
			t.Fatalf("symbol hints=%v unexpectedly contain %q", got, excluded)
		}
	}

	longPlan := query.Plan{Anchors: []string{"Anchor"}, Terms: query.Terms{Words: []string{"word00", "word01", "word02", "word03", "word04", "word05", "word06", "word07", "word08", "word09", "word10", "word11", "word12", "word13", "word14", "word15", "word16", "word17"}}}
	if capped := pathScopedSymbolHints(longPlan); len(capped) != 16 || capped[0] != "Anchor" {
		t.Fatalf("capped symbol hints=%v", capped)
	}
}

func TestCollectScopedPathsUsesFrozenPriorityAndModeBoundaries(t *testing.T) {
	plan := query.Plan{}
	req := SearchRequest{Mode: RetrievalFull, Paths: []string{`request\a.go`}}
	lists := []RankedList{
		{Retriever: RetrieverFTS, Items: []model.RankedCandidate{{Path: "fts/d.go"}, {Path: "shared/c.go"}}},
		{Retriever: RetrieverPath, Items: []model.RankedCandidate{{Path: "explicit/b.go"}, {Path: "shared/c.go"}}},
	}
	probed := []string{"probe/e.go", "explicit/b.go", "probe/f.go", "probe/g.go", "probe/h.go", "probe/i.go"}
	want := []string{"request/a.go", "explicit/b.go", "shared/c.go", "fts/d.go", "probe/e.go", "probe/f.go", "probe/g.go", "probe/h.go"}
	if got := collectScopedPaths(plan, req, lists, probed); !slices.Equal(got, want) {
		t.Fatalf("scoped paths=%v, want %v", got, want)
	}

	relationPlan := query.Plan{Relations: []string{"callers"}}
	wantRelation := []string{"request/a.go", "explicit/b.go", "shared/c.go", "fts/d.go"}
	if got := collectScopedPaths(relationPlan, req, lists, probed); !slices.Equal(got, wantRelation) {
		t.Fatalf("relation scoped paths=%v, want %v", got, wantRelation)
	}

	ftsOnlyReq := req
	ftsOnlyReq.Mode = RetrievalFTSOnly
	if got := collectScopedPaths(plan, ftsOnlyReq, lists, probed); got == nil || len(got) != 0 {
		t.Fatalf("FTS-only scoped paths=%v", got)
	}
}

func TestPathScopeHelpersBoundPunctuationUnicodeAndNUL(t *testing.T) {
	long := strings.Repeat("界", 200) + "\x00tail"
	plan := query.Plan{
		Anchors: []string{`(code_context)`, long},
		Terms: query.Terms{
			Paths:       []string{`[internal\search\search.go]`},
			Identifiers: []string{"", `"Service.ValidateToken"`},
			Symbols:     []string{"処理"},
			Words:       []string{"!!!", "検索", "search"},
		},
	}
	for name, values := range map[string][]string{
		"path":   pathScopeHints(plan),
		"symbol": pathScopedSymbolHints(plan),
	} {
		for _, value := range values {
			if strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > 128 {
				t.Fatalf("%s helper emitted invalid value %q (%d runes)", name, value, utf8.RuneCountInString(value))
			}
		}
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

func containsFold(values []string, want string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return strings.EqualFold(value, want) })
}

func listContainsSymbol(lists []RankedList, retriever RetrieverID, symbol string) bool {
	for _, list := range lists {
		if list.Retriever == retriever && slices.ContainsFunc(list.Items, func(candidate model.RankedCandidate) bool { return candidate.Symbol == symbol }) {
			return true
		}
	}
	return false
}
