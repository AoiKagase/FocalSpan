package evidence

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func TestClassifyMapsCandidateRoles(t *testing.T) {
	resolvedIncoming := func(kind string) *model.RelationContext {
		return &model.RelationContext{AnchorHandle: "anchor", Kind: kind, Direction: model.RelationIncoming, Confidence: .95, Resolved: true}
	}
	resolvedOutgoing := func(kind string) *model.RelationContext {
		return &model.RelationContext{AnchorHandle: "anchor", Kind: kind, Direction: model.RelationOutgoing, Confidence: .95, Resolved: true}
	}
	tests := []struct {
		name      string
		plan      query.Plan
		candidate model.RankedCandidate
		want      Role
	}{
		{name: "exact anchor", plan: query.Plan{PrimaryIntent: query.IntentDefinition, Anchors: []string{"ValidateToken"}}, candidate: model.RankedCandidate{Handle: "target", Symbol: "ValidateToken", Kind: "method", Reasons: []model.ScoreReason{{Code: "symbol-exact"}}}, want: RoleTarget},
		{name: "caller", candidate: model.RankedCandidate{Handle: "caller", Relation: "callers", RelationContext: resolvedIncoming("callers")}, want: RoleCaller},
		{name: "callee", candidate: model.RankedCandidate{Handle: "callee", Relation: "callees", RelationContext: resolvedOutgoing("callees")}, want: RoleCallee},
		{name: "test relation", candidate: model.RankedCandidate{Handle: "test", Relation: "tests", RelationContext: resolvedIncoming("tests")}, want: RoleTest},
		{name: "import outgoing", candidate: model.RankedCandidate{Handle: "import", Relation: "imports", RelationContext: resolvedOutgoing("imports")}, want: RoleImport},
		{name: "import incoming", candidate: model.RankedCandidate{Handle: "dependent", Relation: "imports", RelationContext: resolvedIncoming("imports")}, want: RoleDependent},
		{name: "reference", candidate: model.RankedCandidate{Handle: "reference", Relation: "references", RelationContext: resolvedIncoming("references")}, want: RoleReference},
		{name: "impact change", plan: query.Plan{PrimaryIntent: query.IntentImpact}, candidate: model.RankedCandidate{Handle: "change", Kind: "method", Changed: true}, want: RoleChange},
		{name: "impact reverse dependency", plan: query.Plan{PrimaryIntent: query.IntentImpact}, candidate: model.RankedCandidate{Handle: "dependent", Relation: "callers", RelationContext: resolvedIncoming("callers")}, want: RoleDependent},
		{name: "test kind", candidate: model.RankedCandidate{Handle: "test", Path: "src/token_test.go", Kind: "test"}, want: RoleTest},
		{name: "type", candidate: model.RankedCandidate{Handle: "type", Kind: "struct-outline"}, want: RoleType},
		{name: "template", candidate: model.RankedCandidate{Handle: "template", Kind: "template-block"}, want: RoleTemplate},
		{name: "config", candidate: model.RankedCandidate{Handle: "config", Path: "config/auth.json", Kind: "object"}, want: RoleConfig},
		{name: "documentation", candidate: model.RankedCandidate{Handle: "docs", Path: "README.md", Language: "markdown", Kind: "heading"}, want: RoleDocumentation},
		{name: "implementation", candidate: model.RankedCandidate{Handle: "impl", Kind: "method"}, want: RoleImplementation},
		{name: "declaration", candidate: model.RankedCandidate{Handle: "decl", Kind: "prototype"}, want: RoleDeclaration},
		{name: "definition", candidate: model.RankedCandidate{Handle: "def", Kind: "definition"}, want: RoleDefinition},
		{name: "context", candidate: model.RankedCandidate{Handle: "context", Kind: "unknown"}, want: RoleContext},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.plan, []model.RankedCandidate{tt.candidate})
			if len(got) != 1 || got[0].Role != tt.want {
				t.Fatalf("role = %+v, want %q", got, tt.want)
			}
		})
	}
}

func TestWhyCodesAreStableBoundedAndOrdered(t *testing.T) {
	candidate := model.RankedCandidate{
		Handle: "caller", Path: "auth/service.go", Kind: "method", Changed: true, Relation: "callers",
		RelationContext: &model.RelationContext{AnchorHandle: "target", Kind: "callers", Direction: model.RelationIncoming, Confidence: .95, Resolved: true},
		Reasons:         []model.ScoreReason{{Code: "lexical"}, {Code: "path"}, {Code: "symbol-exact"}, {Code: "relation-callers"}, {Code: "symbol-exact"}},
	}
	got := Classify(query.Plan{PrimaryIntent: query.IntentCallers}, []model.RankedCandidate{candidate})
	want := []string{"exact_symbol", "direct_caller", "changed_span", "path_match"}
	if !reflect.DeepEqual(got[0].Why, want) {
		t.Fatalf("why = %v, want %v", got[0].Why, want)
	}
}

func TestCertaintyMapping(t *testing.T) {
	tests := []struct {
		context model.RelationContext
		want    Certainty
	}{
		{model.RelationContext{Resolved: true, Confidence: .9}, CertaintyExact},
		{model.RelationContext{Resolved: true, Confidence: .89}, CertaintyScoped},
		{model.RelationContext{Resolved: false, Confidence: 1}, CertaintyLexical},
	}
	for _, tt := range tests {
		if got := certaintyFor(tt.context); got != tt.want {
			t.Fatalf("certaintyFor(%+v) = %q, want %q", tt.context, got, tt.want)
		}
	}
}

func TestPresentationOrderUsesIntentThenOriginalRank(t *testing.T) {
	classified := []ClassifiedCandidate{
		{Candidate: model.RankedCandidate{Handle: "context", Path: "z.go", StartLine: 1}, OriginalRank: 0, Role: RoleContext},
		{Candidate: model.RankedCandidate{Handle: "caller-b", Path: "b.go", StartLine: 2}, OriginalRank: 2, Role: RoleCaller},
		{Candidate: model.RankedCandidate{Handle: "target", Path: "target.go", StartLine: 1}, OriginalRank: 3, Role: RoleTarget},
		{Candidate: model.RankedCandidate{Handle: "caller-a", Path: "a.go", StartLine: 1}, OriginalRank: 1, Role: RoleCaller},
	}
	SortForPresentation(query.Plan{PrimaryIntent: query.IntentCallers}, classified)
	want := []string{"target", "caller-a", "caller-b", "context"}
	got := make([]string, len(classified))
	for i := range classified {
		got[i] = classified[i].Candidate.Handle
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestClassifyIsInputSafeAndDeterministic(t *testing.T) {
	input := []model.RankedCandidate{
		{Handle: "b", Path: "b.go", Kind: "method", Reasons: []model.ScoreReason{{Code: "lexical"}, {Code: "symbol-exact"}}},
		{Handle: "a", Path: "a.go", Kind: "test", Relation: "tests", Reasons: []model.ScoreReason{{Code: "test-pair"}}},
	}
	before, _ := json.Marshal(input)
	plan := query.Plan{PrimaryIntent: query.IntentTests}
	var want []byte
	for iteration := 0; iteration < 100; iteration++ {
		copyInput := append([]model.RankedCandidate(nil), input...)
		if iteration%2 == 1 {
			rand.New(rand.NewSource(int64(iteration))).Shuffle(len(copyInput), func(i, j int) { copyInput[i], copyInput[j] = copyInput[j], copyInput[i] })
		}
		classified := Classify(plan, copyInput)
		SortForPresentation(plan, classified)
		stable := make([]struct {
			Handle    string
			Role      Role
			Certainty Certainty
			Why       []string
		}, len(classified))
		for i := range classified {
			stable[i] = struct {
				Handle    string
				Role      Role
				Certainty Certainty
				Why       []string
			}{classified[i].Candidate.Handle, classified[i].Role, classified[i].Certainty, classified[i].Why}
		}
		got, _ := json.Marshal(stable)
		if want == nil {
			want = got
		} else if string(got) != string(want) {
			t.Fatalf("iteration %d differs\n got: %s\nwant: %s", iteration, got, want)
		}
	}
	after, _ := json.Marshal(input)
	if string(after) != string(before) {
		t.Fatalf("Classify mutated input\n before: %s\n after: %s", before, after)
	}
}
