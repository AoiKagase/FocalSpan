package evidence

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func compilerCandidates() []model.RankedCandidate {
	targetContent := "func ValidateToken() error {\n" + strings.Repeat("\twork()\n", 80) + "\treturn ErrExpiredToken\n}\n"
	return []model.RankedCandidate{
		{Handle: "target", Path: "auth/service.go", Language: "go", Kind: "method", Symbol: "ValidateToken", Signature: "func ValidateToken() error", StartLine: 10, EndLine: 93, Content: targetContent, ContentHash: "target-hash", Reasons: []model.ScoreReason{{Code: "symbol-exact"}}},
		{Handle: "caller", Path: "http/middleware.go", Language: "go", Kind: "method", Symbol: "Authenticate", Signature: "func Authenticate() error", StartLine: 5, EndLine: 8, Content: "func Authenticate() error {\n\treturn service.ValidateToken()\n}\n", ContentHash: "caller-hash", Relation: "callers", RelationContext: &model.RelationContext{AnchorHandle: "target", Kind: "callers", Direction: model.RelationIncoming, Confidence: .95, Source: "go-ast", Resolved: true}},
		{Handle: "test", Path: "auth/service_test.go", Language: "go", Kind: "test", Symbol: "TestValidateToken", Signature: "func TestValidateToken(t *testing.T)", StartLine: 7, EndLine: 14, Content: "func TestValidateToken(t *testing.T) {\n\t_ = ErrExpiredToken\n}\n", ContentHash: "test-hash", Relation: "tests", RelationContext: &model.RelationContext{AnchorHandle: "target", Kind: "tests", Direction: model.RelationIncoming, Confidence: .8, Source: "go-ast", Resolved: true}},
		{Handle: "noise", Path: "unrelated/report.go", Language: "go", Kind: "function", Symbol: "BuildReport", Signature: "func BuildReport()", StartLine: 1, EndLine: 40, Content: strings.Repeat("report line\n", 40), ContentHash: "noise-hash", Reasons: []model.ScoreReason{{Code: "lexical"}}},
	}
}

func compilerRequest(tokenBudget int) CompileRequest {
	return CompileRequest{
		Plan:     query.Plan{RawQuery: "what calls ValidateToken?", PrimaryIntent: query.IntentCallers, Intents: []query.Intent{query.IntentCallers}, Anchors: []string{"ValidateToken"}, Terms: query.Terms{Identifiers: []string{"ValidateToken", "ErrExpiredToken"}, Words: []string{"expired", "token"}}},
		Revision: "rev-1", TokenBudget: tokenBudget, Mode: ModeFocused, Candidates: compilerCandidates(),
	}
}

func TestCompilerRespectsFinalModelVisibleBudget(t *testing.T) {
	estimator := budget.NewEstimator()
	compiler := NewCompiler(estimator)
	for _, requested := range []int{256, 512, 1200, 4000, 64000, 0, 1, 64001} {
		t.Run(strconv.Itoa(requested), func(t *testing.T) {
			result, err := compiler.Compile(compilerRequest(requested))
			if err != nil {
				t.Fatal(err)
			}
			used := MeasureModelVisible(result.Packet, estimator)
			if used > result.Packet.Budget.Limit {
				t.Fatalf("wire packet uses %d > %d", used, result.Packet.Budget.Limit)
			}
			if used != result.Packet.Budget.Used || used != result.Stats.WireTokens {
				t.Fatalf("reported=%d stats=%d measured=%d", result.Packet.Budget.Used, result.Stats.WireTokens, used)
			}
			wantLimit := requested
			if wantLimit < budget.MinBudget {
				wantLimit = budget.MinBudget
			}
			if wantLimit > budget.MaxBudget {
				wantLimit = budget.MaxBudget
			}
			if result.Packet.Budget.Limit != wantLimit {
				t.Fatalf("limit = %d, want %d", result.Packet.Budget.Limit, wantLimit)
			}
			if err := Validate(result.Packet); err != nil {
				t.Fatalf("compiled packet invalid: %v", err)
			}
		})
	}
}

func TestCompilerKeepsCompactTargetAndBuildsLocalEdges(t *testing.T) {
	result, err := NewCompiler(nil).Compile(compilerRequest(4000))
	if err != nil {
		t.Fatal(err)
	}
	byHandle := make(map[string]Item)
	for _, item := range result.Packet.Evidence {
		byHandle[item.Handle] = item
	}
	target, targetOK := byHandle["target"]
	caller, callerOK := byHandle["caller"]
	if !targetOK || target.Role != RoleTarget || !callerOK || caller.Role != RoleCaller {
		t.Fatalf("target/caller roles absent: %+v", result.Packet.Evidence)
	}
	wantEdge := Edge{From: caller.ID, To: target.ID, Kind: "calls", Certainty: CertaintyExact}
	found := false
	for _, edge := range result.Packet.Relations {
		found = found || edge == wantEdge
	}
	if !found {
		t.Fatalf("edge %+v absent: %+v", wantEdge, result.Packet.Relations)
	}
}

func TestCompilerPreprocessesDuplicatesAndKnownHandles(t *testing.T) {
	req := compilerRequest(4000)
	req.Candidates = append(req.Candidates, req.Candidates[0], req.Candidates[1])
	req.KnownHandles = []string{"caller"}
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range result.Packet.Evidence {
		if item.Handle == "caller" {
			t.Fatal("known caller was retransmitted")
		}
		if seen[item.Handle] {
			t.Fatalf("duplicate handle %q", item.Handle)
		}
		seen[item.Handle] = true
	}
	if result.Packet.SkippedKnown != 1 || result.Stats.SkippedKnown != 1 {
		t.Fatalf("skipped known packet=%d stats=%d", result.Packet.SkippedKnown, result.Stats.SkippedKnown)
	}
	for _, edge := range result.Packet.Relations {
		if edge.From == "" || edge.To == "" {
			t.Fatalf("dangling edge: %+v", edge)
		}
	}
}

func TestCompilerIsDeterministic(t *testing.T) {
	compiler := NewCompiler(nil)
	var want []byte
	for iteration := 0; iteration < 100; iteration++ {
		result, err := compiler.Compile(compilerRequest(1200))
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(result.Packet)
		if err != nil {
			t.Fatal(err)
		}
		if want == nil {
			want = got
		} else if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d differs\n got: %s\nwant: %s", iteration, got, want)
		}
	}
}

func TestCompilerTinyBudgetBoundaryMatrix(t *testing.T) {
	estimator := budget.NewEstimator()
	for _, requested := range []int{0, 1, 255, 256, 257, 511, 512, 1199, 1200, 63999, 64000, 64001} {
		t.Run(strconv.Itoa(requested), func(t *testing.T) {
			result, err := NewCompiler(estimator).Compile(compilerRequest(requested))
			if err != nil {
				t.Fatal(err)
			}
			want := requested
			if want < budget.MinBudget {
				want = budget.MinBudget
			}
			if want > budget.MaxBudget {
				want = budget.MaxBudget
			}
			if result.Packet.Budget.Limit != want || result.Packet.Budget.Used > want || result.Packet.Budget.Used != MeasureModelVisible(result.Packet, estimator) {
				t.Fatalf("budget=%+v want limit=%d", result.Packet.Budget, want)
			}
			if err := Validate(result.Packet); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCompilerMixedKnownAnchorsNeverCreatesDanglingEdges(t *testing.T) {
	tests := []struct {
		name  string
		known []string
	}{
		{name: "anchor known candidate new", known: []string{"target"}},
		{name: "anchor new candidate known", known: []string{"caller"}},
		{name: "both known", known: []string{"target", "caller"}},
		{name: "neither known"},
		{name: "multiple anchors one known", known: []string{"other-anchor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := compilerRequest(4000)
			req.Candidates = req.Candidates[:2]
			req.KnownHandles = tt.known
			req.ExpansionAnchors = []string{"target", "other-anchor"}
			result, err := NewCompiler(nil).Compile(req)
			if err != nil {
				t.Fatal(err)
			}
			known := map[string]bool{}
			for _, handle := range tt.known {
				known[handle] = true
			}
			ids := map[string]bool{}
			for _, item := range result.Packet.Evidence {
				if known[item.Handle] {
					t.Fatalf("known handle retransmitted: %s", item.Handle)
				}
				ids[item.ID] = true
			}
			for _, edge := range result.Packet.Relations {
				if !ids[edge.From] || !ids[edge.To] {
					t.Fatalf("dangling edge: %+v packet=%+v", edge, result.Packet)
				}
			}
		})
	}
}

func TestCompilerUnresolvedRelationNeverProducesExactEdge(t *testing.T) {
	req := compilerRequest(4000)
	req.Candidates = req.Candidates[:2]
	req.Candidates[1].RelationContext.Resolved = false
	req.Candidates[1].RelationContext.Confidence = 1
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packet.Relations) == 0 || result.Packet.Relations[0].Certainty != CertaintyLexical {
		t.Fatalf("unresolved edge=%+v", result.Packet.Relations)
	}
	if !containsString(result.Packet.Limitations, "lexical_relation_only") {
		t.Fatalf("limitations=%v", result.Packet.Limitations)
	}
}

func TestPruneMetadataPreservesV1RequiredFields(t *testing.T) {
	packet := Packet{
		Schema: SchemaContextV1, Intent: "callers", Mode: ModeFocused,
		Budget: Budget{Limit: 1200},
		Evidence: []Item{
			{ID: "e1", Handle: "target", Role: RoleTarget, Location: Location{Path: "auth/service.go", Lines: [2]int{10, 20}}, Language: "go", Kind: "method", Symbol: "ValidateToken", Fidelity: FidelityVerbatim, Source: "func ValidateToken() error {\n\treturn nil\n}"},
			{ID: "e2", Handle: "caller", Role: RoleCaller, Location: Location{Path: "http/middleware.go", Lines: [2]int{4, 8}}, Language: "go", Kind: "method", Symbol: "Authenticate", Fidelity: FidelitySignature, Signature: "func Authenticate() error"},
		},
		Relations:    []Edge{{From: "e2", To: "e1", Kind: "calls", Certainty: CertaintyExact}},
		SkippedKnown: 1,
	}
	settleWireUsage(&packet, budget.NewEstimator())
	before := packet
	prunePacketMetadata(&packet)
	settleWireUsage(&packet, budget.NewEstimator())

	if packet.Schema != before.Schema || packet.Mode != before.Mode || packet.SkippedKnown != before.SkippedKnown {
		t.Fatalf("packet contract changed: before=%+v after=%+v", before, packet)
	}
	if packet.Evidence[0].Handle != before.Evidence[0].Handle || packet.Evidence[0].Role != before.Evidence[0].Role || packet.Evidence[0].Location != before.Evidence[0].Location || packet.Evidence[0].Fidelity != before.Evidence[0].Fidelity || packet.Evidence[0].Symbol != before.Evidence[0].Symbol || packet.Evidence[0].Source != before.Evidence[0].Source {
		t.Fatalf("target identity/fidelity changed: before=%+v after=%+v", before.Evidence[0], packet.Evidence[0])
	}
	if packet.Evidence[1].Handle != before.Evidence[1].Handle || packet.Evidence[1].Role != before.Evidence[1].Role || packet.Evidence[1].Location != before.Evidence[1].Location || packet.Evidence[1].Fidelity != before.Evidence[1].Fidelity || packet.Evidence[1].Signature != before.Evidence[1].Signature {
		t.Fatalf("caller identity/fidelity changed: before=%+v after=%+v", before.Evidence[1], packet.Evidence[1])
	}
	if !reflect.DeepEqual(packet.Relations, before.Relations) {
		t.Fatalf("relations changed: before=%v after=%v", before.Relations, packet.Relations)
	}
	if packet.Budget.Used > packet.Budget.Limit || packet.Budget.Used != MeasureModelVisible(packet, budget.NewEstimator()) {
		t.Fatalf("budget invalid after pruning: %+v", packet.Budget)
	}
	if err := Validate(packet); err != nil {
		t.Fatalf("pruned packet invalid: %v", err)
	}
}

func TestPruneMetadataDropsOnlyRedundantOptionalFields(t *testing.T) {
	packet := Packet{
		Schema: SchemaContextV1, Mode: ModeFocused, Budget: Budget{Limit: 1200},
		Evidence: []Item{
			{ID: "e1", Handle: "target", Role: RoleTarget, Location: Location{Path: "auth/service.go", Lines: [2]int{1, 4}}, Language: "go", Kind: "method", Symbol: "ValidateToken", Fidelity: FidelitySignature, Signature: "func ValidateToken() error", Why: []string{"exact_symbol", "path_match"}},
			{ID: "e2", Handle: "caller", Role: RoleCaller, Location: Location{Path: "http/middleware.go", Lines: [2]int{10, 14}}, Language: "go", Kind: "method", Symbol: "Authenticate", Fidelity: FidelityVerbatim, Source: "func Authenticate() error { return nil }", Why: []string{"direct_caller", "lexical_match", "same_file"}},
			{ID: "e3", Handle: "rust", Role: RoleCaller, Location: Location{Path: "worker/lib.rs", Lines: [2]int{2, 5}}, Language: "rust", Kind: "function", Symbol: "run", Fidelity: FidelitySignature, Signature: "fn run()", Why: []string{"qualified_symbol", "path_match"}},
		},
	}
	prunePacketMetadata(&packet)

	target, caller, rust := packet.Evidence[0], packet.Evidence[1], packet.Evidence[2]
	if target.Language != "go" || target.Kind != "method" || target.Symbol != "ValidateToken" || !reflect.DeepEqual(target.Why, []string{"exact_symbol"}) {
		t.Fatalf("target metadata was over-pruned: %+v", target)
	}
	if caller.Language != "" || caller.Kind != "" || caller.Symbol != "Authenticate" || !reflect.DeepEqual(caller.Why, []string{"direct_caller"}) {
		t.Fatalf("redundant caller metadata was not pruned: %+v", caller)
	}
	if rust.Language != "rust" || rust.Symbol != "run" || !reflect.DeepEqual(rust.Why, []string{"qualified_symbol"}) {
		t.Fatalf("distinct-language metadata was over-pruned: %+v", rust)
	}
}

func TestPruneMetadataReducesMeasuredWireWithoutChangingSelection(t *testing.T) {
	estimator := budget.NewEstimator()
	result, err := NewCompiler(estimator).Compile(compilerRequest(4000))
	if err != nil {
		t.Fatal(err)
	}
	selectedHandles := make([]string, 0, len(result.Packet.Evidence))
	selectedRoles := make([]Role, 0, len(result.Packet.Evidence))
	for _, item := range result.Packet.Evidence {
		selectedHandles = append(selectedHandles, item.Handle)
		selectedRoles = append(selectedRoles, item.Role)
	}
	full := result.Packet
	full.Evidence = append([]Item(nil), result.Packet.Evidence...)
	byHandle := make(map[string]model.RankedCandidate, len(compilerCandidates()))
	for _, candidate := range compilerCandidates() {
		byHandle[candidate.Handle] = candidate
	}
	for index := range full.Evidence {
		candidate := byHandle[full.Evidence[index].Handle]
		full.Evidence[index].Language = candidate.Language
		full.Evidence[index].Kind = candidate.Kind
		full.Evidence[index].Symbol = candidate.Symbol
		full.Evidence[index].Why = whyCodes(candidate)
	}
	full.Budget.Used = 0
	settleWireUsage(&full, estimator)
	if result.Packet.Budget.Used >= full.Budget.Used {
		t.Fatalf("pruning did not reduce measured wire: pruned=%d full=%d", result.Packet.Budget.Used, full.Budget.Used)
	}
	for index, item := range result.Packet.Evidence {
		if item.Handle != selectedHandles[index] || item.Role != selectedRoles[index] {
			t.Fatalf("selection changed at %d: got %s/%s want %s/%s", index, item.Handle, item.Role, selectedHandles[index], selectedRoles[index])
		}
	}
	if result.Packet.Budget.Used > result.Packet.Budget.Limit || result.Packet.Budget.Used != MeasureModelVisible(result.Packet, estimator) {
		t.Fatalf("pruned compile result exceeds budget: %+v", result.Packet.Budget)
	}
}

func TestPruneMetadataIsIdempotentAndSchemaCompatible(t *testing.T) {
	packet := Packet{Schema: SchemaContextV1, Mode: ModeFocused, Budget: Budget{Limit: 1200}, Evidence: []Item{{ID: "e1", Handle: "target", Role: RoleTarget, Location: Location{Path: "main.go", Lines: [2]int{1, 2}}, Language: "go", Kind: "function", Symbol: "main", Fidelity: FidelitySignature, Signature: "func main()", Why: []string{"exact_symbol", "lexical_match"}}}}
	prunePacketMetadata(&packet)
	first, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	prunePacketMetadata(&packet)
	second, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || packet.Schema != SchemaContextV1 {
		t.Fatalf("pruning is not idempotent/schema-compatible: first=%s second=%s schema=%s", first, second, packet.Schema)
	}
	for _, forbidden := range []string{"token_savings", "baseline_tokens", "saved_tokens", "diagnostic_stage"} {
		if strings.Contains(string(second), forbidden) {
			t.Fatalf("debug field leaked into packet: %s", forbidden)
		}
	}
}
