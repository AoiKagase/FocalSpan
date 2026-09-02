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

func TestCompilerReservesLaterExactAnchorUnderBudgetPressure(t *testing.T) {
	req := compilerRequest(512)
	req.Mode = ModeOutline
	candidates := []model.RankedCandidate{
		{Handle: "caller", Path: "src/caller.go", Language: "go", Kind: "method", Symbol: "Caller", Signature: "func Caller()", StartLine: 1, EndLine: 90, Score: 500, Relation: "callers", RelationContext: &model.RelationContext{AnchorHandle: "first-exact", Kind: "callers", Direction: model.RelationIncoming, Confidence: .95, Source: "go-ast", Resolved: true}},
		{Handle: "first-exact", Path: "src/first.go", Language: "go", Kind: "function", Symbol: "First", Signature: "func First()", StartLine: 1, EndLine: 1, Score: 40, Reasons: []model.ScoreReason{{Code: "symbol-exact"}}, Content: "func First() {}"},
		{Handle: "noise-1", Path: "src/noise-1.go", Language: "go", Kind: "function", Symbol: "NoiseOne", Signature: "func NoiseOne()", StartLine: 1, EndLine: 1, Score: 30, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-2", Path: "src/noise-2.go", Language: "go", Kind: "function", Symbol: "NoiseTwo", Signature: "func NoiseTwo()", StartLine: 1, EndLine: 1, Score: 20, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-3", Path: "src/noise-3.go", Language: "go", Kind: "function", Symbol: "NoiseThree", Signature: "func NoiseThree()", StartLine: 1, EndLine: 1, Score: 10, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-4", Path: "src/noise-4.go", Language: "go", Kind: "function", Symbol: "NoiseFour", Signature: "func NoiseFour()", StartLine: 1, EndLine: 1, Score: 9, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-5", Path: "src/noise-5.go", Language: "go", Kind: "function", Symbol: "NoiseFive", Signature: "func NoiseFive()", StartLine: 1, EndLine: 1, Score: 8, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-6", Path: "src/noise-6.go", Language: "go", Kind: "function", Symbol: "NoiseSix", Signature: "func NoiseSix()", StartLine: 1, EndLine: 1, Score: 7, Reasons: []model.ScoreReason{{Code: "lexical"}}},
	}
	for index := 7; index <= 20; index++ {
		candidates = append(candidates, model.RankedCandidate{Handle: "noise-" + strconv.Itoa(index), Path: "src/noise-" + strconv.Itoa(index) + ".go", Language: "go", Kind: "function", Symbol: "Noise" + strconv.Itoa(index), Signature: "func Noise" + strconv.Itoa(index) + "()", StartLine: 1, EndLine: 1, Score: float64(30 - index), Reasons: []model.ScoreReason{{Code: "lexical"}}})
	}
	req.Candidates = append(candidates, model.RankedCandidate{Handle: "later-exact", Path: "src/target.go", Language: "go", Kind: "function", Symbol: "Target", Signature: "func Target()", StartLine: 1, EndLine: 1, Score: 10, Reasons: []model.ScoreReason{{Code: "symbol-exact"}}, Content: "func Target() {}"})
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	if !packetHasHandle(result.Packet, "later-exact") {
		t.Fatalf("later exact anchor was dropped: packet=%+v", result.Packet)
	}
}

func TestCompilerReservesRelationAnchorAndCandidateWithoutDanglingEdge(t *testing.T) {
	req := compilerRequest(512)
	req.Mode = ModeOutline
	req.Candidates = []model.RankedCandidate{
		{Handle: "target", Path: "src/target.go", Language: "go", Kind: "function", Symbol: "Target", Signature: "func Target()", StartLine: 1, EndLine: 1, Score: 100, Reasons: []model.ScoreReason{{Code: "symbol-exact"}}, Content: "func Target() {}"},
		{Handle: "dependent", Path: "src/dependent.go", Language: "go", Kind: "function", Symbol: "Dependent", Signature: "func Dependent()", StartLine: 1, EndLine: 80, Score: 90, Content: "func Dependent() {\n" + strings.Repeat("\tuse()\n", 80) + "}\n", Relation: "callers", RelationContext: &model.RelationContext{AnchorHandle: "path-anchor", Kind: "callers", Direction: model.RelationIncoming, Confidence: .95, Source: "go-ast", Resolved: true}},
		{Handle: "noise-1", Path: "src/noise-1.go", Language: "go", Kind: "function", Symbol: "NoiseOne", Signature: "func NoiseOne()", StartLine: 1, EndLine: 1, Score: 80, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-2", Path: "src/noise-2.go", Language: "go", Kind: "function", Symbol: "NoiseTwo", Signature: "func NoiseTwo()", StartLine: 1, EndLine: 1, Score: 70, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "noise-3", Path: "src/noise-3.go", Language: "go", Kind: "function", Symbol: "NoiseThree", Signature: "func NoiseThree()", StartLine: 1, EndLine: 1, Score: 60, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "path-anchor", Path: "src/path-anchor.go", Language: "go", Kind: "function", Symbol: "PathAnchor", Signature: "func PathAnchor()", StartLine: 1, EndLine: 1, Score: 5, Reasons: []model.ScoreReason{{Code: "path"}}, Content: "func PathAnchor() {}"},
	}
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	if !packetHasHandle(result.Packet, "dependent") || !packetHasHandle(result.Packet, "path-anchor") {
		t.Fatalf("relation anchor pair not retained: packet=%+v", result.Packet)
	}
	if len(result.Packet.Relations) != 1 {
		t.Fatalf("relation edge missing or duplicated: %+v", result.Packet.Relations)
	}
	ids := map[string]bool{}
	for _, item := range result.Packet.Evidence {
		ids[item.ID] = true
	}
	for _, edge := range result.Packet.Relations {
		if !ids[edge.From] || !ids[edge.To] {
			t.Fatalf("dangling relation edge: %+v", edge)
		}
	}
}

func TestCompilerDoesNotReserveGenericLexicalCandidate(t *testing.T) {
	req := compilerRequest(512)
	req.Mode = ModeOutline
	req.Candidates = []model.RankedCandidate{
		{Handle: "target", Path: "src/target.go", Language: "go", Kind: "function", Symbol: "Target", Signature: "func Target()", StartLine: 1, EndLine: 1, Score: 20, Reasons: []model.ScoreReason{{Code: "symbol-exact"}}, Content: "func Target() {}"},
		{Handle: "lexical-1", Path: "src/lexical-1.go", Language: "go", Kind: "function", Symbol: "LexicalOne", Signature: "func LexicalOne()", StartLine: 1, EndLine: 1, Score: 1000, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "lexical-2", Path: "src/lexical-2.go", Language: "go", Kind: "function", Symbol: "LexicalTwo", Signature: "func LexicalTwo()", StartLine: 1, EndLine: 1, Score: 900, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "lexical-3", Path: "src/lexical-3.go", Language: "go", Kind: "function", Symbol: "LexicalThree", Signature: "func LexicalThree()", StartLine: 1, EndLine: 1, Score: 800, Reasons: []model.ScoreReason{{Code: "lexical"}}},
		{Handle: "path-anchor", Path: "src/path-anchor.go", Language: "go", Kind: "function", Symbol: "PathAnchor", Signature: "func PathAnchor()", StartLine: 1, EndLine: 1, Score: 5, Reasons: []model.ScoreReason{{Code: "path"}}, Content: "func PathAnchor() {}"},
	}
	req.Plan.Anchors = []string{"Target", "PathAnchor"}
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	if !packetHasHandle(result.Packet, "path-anchor") {
		t.Fatalf("path anchor was dropped while lexical noise was retained: %+v", result.Packet.Evidence)
	}
	lexicalCount := 0
	for _, item := range result.Packet.Evidence {
		if strings.HasPrefix(item.Handle, "lexical-") {
			lexicalCount++
		}
	}
	if lexicalCount == 3 {
		t.Fatalf("generic lexical candidates were all retained as if reserved: %+v", result.Packet.Evidence)
	}
}

func TestCompilerKeepsAnchorWithSignatureFallbackAtTightBudget(t *testing.T) {
	req := compilerRequest(512)
	req.Mode = ModeSource
	req.Candidates = []model.RankedCandidate{
		{Handle: "target", Path: "src/target.go", Language: "go", Kind: "function", Symbol: "Target", Signature: "func Target()", StartLine: 1, EndLine: 1, Score: 100, Reasons: []model.ScoreReason{{Code: "symbol-exact"}}, Content: "func Target() {}"},
		{Handle: "lexical-1", Path: "src/lexical-1.go", Language: "go", Kind: "function", Symbol: "LexicalOne", Signature: "func LexicalOne()", StartLine: 1, EndLine: 100, Score: 1000, Reasons: []model.ScoreReason{{Code: "lexical"}}, Content: "func LexicalOne() {\n" + strings.Repeat("\tnoise()\n", 100) + "}\n"},
		{Handle: "lexical-2", Path: "src/lexical-2.go", Language: "go", Kind: "function", Symbol: "LexicalTwo", Signature: "func LexicalTwo()", StartLine: 1, EndLine: 100, Score: 900, Reasons: []model.ScoreReason{{Code: "lexical"}}, Content: "func LexicalTwo() {\n" + strings.Repeat("\tnoise()\n", 100) + "}\n"},
		{Handle: "lexical-3", Path: "src/lexical-3.go", Language: "go", Kind: "function", Symbol: "LexicalThree", Signature: "func LexicalThree()", StartLine: 1, EndLine: 100, Score: 800, Reasons: []model.ScoreReason{{Code: "lexical"}}, Content: "func LexicalThree() {\n" + strings.Repeat("\tnoise()\n", 100) + "}\n"},
		{Handle: "path-anchor", Path: "src/path-anchor.go", Language: "go", Kind: "function", Symbol: "PathAnchor", Signature: "func PathAnchor()", StartLine: 1, EndLine: 122, Score: 5, Reasons: []model.ScoreReason{{Code: "path"}}, Content: "func PathAnchor() {\n" + strings.Repeat("\tveryLongBody()\n", 120) + "}\n"},
	}
	result, err := NewCompiler(nil).Compile(req)
	if err != nil {
		t.Fatal(err)
	}
	var anchor Item
	for _, item := range result.Packet.Evidence {
		if item.Handle == "path-anchor" {
			anchor = item
		}
	}
	if anchor.Handle == "" {
		t.Fatalf("tight-budget path anchor was omitted: %+v", result.Packet.Evidence)
	}
	if anchor.Fidelity != FidelitySignature && anchor.Fidelity != FidelityExcerpt {
		t.Fatalf("tight-budget anchor retained without fallback: %+v", anchor)
	}
	if result.Packet.Budget.Used > result.Packet.Budget.Limit {
		t.Fatalf("budget exceeded: %+v", result.Packet.Budget)
	}
}

func packetHasHandle(packet Packet, handle string) bool {
	for _, item := range packet.Evidence {
		if item.Handle == handle {
			return true
		}
	}
	return false
}
