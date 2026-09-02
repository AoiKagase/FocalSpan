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

func TestAdaptiveCompilerKeepsRelationAndBudgetInvariants(t *testing.T) {
	for _, requested := range []int{512, 1200, 2048} {
		t.Run(strconv.Itoa(requested), func(t *testing.T) {
			req := compilerRequest(requested)
			result, err := NewCompiler(nil).Compile(req)
			if err != nil {
				t.Fatal(err)
			}
			if result.Packet.Budget.Used > result.Packet.Budget.Limit {
				t.Fatalf("budget exceeded: %+v", result.Packet.Budget)
			}
			byHandle := make(map[string]model.RankedCandidate, len(req.Candidates))
			for _, candidate := range req.Candidates {
				byHandle[candidate.Handle] = candidate
			}
			ids := make(map[string]bool, len(result.Packet.Evidence))
			for _, item := range result.Packet.Evidence {
				ids[item.ID] = true
				candidate, ok := byHandle[item.Handle]
				if !ok {
					t.Fatalf("unknown evidence handle %q", item.Handle)
				}
				for _, segment := range item.Segments {
					if segment.Kind != SegmentSource {
						if segment.Text != "" {
							t.Fatalf("omitted segment contains text: %+v", segment)
						}
						continue
					}
					want := testSourceLines(candidate.Content, segment.Lines, candidate.StartLine)
					if segment.Text != want {
						t.Fatalf("source fidelity mismatch for %s: got=%q want=%q", item.Handle, segment.Text, want)
					}
				}
			}
			for _, edge := range result.Packet.Relations {
				if !ids[edge.From] || !ids[edge.To] {
					t.Fatalf("dangling relation: %+v", edge)
				}
			}
			target, targetOK := findEvidence(result.Packet.Evidence, "target")
			caller, callerOK := findEvidence(result.Packet.Evidence, "caller")
			if !targetOK || !callerOK {
				t.Fatalf("required target/caller missing: %+v", result.Packet.Evidence)
			}
			wantCallerEdge := Edge{From: caller.ID, To: target.ID, Kind: "calls", Certainty: CertaintyExact}
			callsExact := false
			for _, edge := range result.Packet.Relations {
				if edge == wantCallerEdge {
					callsExact = true
				}
			}
			if !callsExact {
				t.Fatalf("required exact caller relation missing: got=%+v want=%+v", result.Packet.Relations, wantCallerEdge)
			}
			if err := Validate(result.Packet); err != nil {
				t.Fatalf("packet invalid: %v", err)
			}
		})
	}
}

func findEvidence(items []Item, handle string) (Item, bool) {
	for _, item := range items {
		if item.Handle == handle {
			return item, true
		}
	}
	return Item{}, false
}
