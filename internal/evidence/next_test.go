package evidence

import (
	"reflect"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func guidanceCandidate(handle string, role Role, relation, anchor string) ClassifiedCandidate {
	candidate := model.RankedCandidate{Handle: handle, Path: "a.go", Kind: "function", StartLine: 1, EndLine: 2}
	if relation != "" {
		candidate.Relation = relation
		candidate.RelationContext = &model.RelationContext{AnchorHandle: anchor, Kind: relation, Direction: model.RelationIncoming, Confidence: .8, Resolved: true}
	}
	return ClassifiedCandidate{Candidate: candidate, Role: role}
}

func TestGuidanceBuildsIntentActionsAndLimitations(t *testing.T) {
	input := GuidanceInput{
		Plan: query.Plan{PrimaryIntent: query.IntentCallers}, Truncated: true,
		Selected: []GuidanceSelection{
			{Candidate: guidanceCandidate("target", RoleTarget, "", ""), Fidelity: FidelitySignature},
			{Candidate: guidanceCandidate("caller", RoleCaller, "callers", "target"), Fidelity: FidelityVerbatim},
		},
		Omitted: []ClassifiedCandidate{
			guidanceCandidate("caller-2", RoleCaller, "callers", "target"),
			guidanceCandidate("test", RoleTest, "tests", "target"),
		},
	}
	limitations, next := BuildGuidance(input)
	for _, want := range []string{"budget_limited", "source_reduced_to_signature", "additional_callers_omitted", "additional_tests_omitted"} {
		if !containsString(limitations, want) {
			t.Fatalf("limitation %q absent: %v", want, limitations)
		}
	}
	wantActions := []NextAction{
		{Handle: "target", Relation: "callers", Reason: "more_callers_omitted"},
		{Handle: "target", Relation: "tests", Reason: "more_tests_omitted"},
		{Handle: "target", Relation: "self", Reason: "source_body_omitted"},
	}
	if !reflect.DeepEqual(next, wantActions) {
		t.Fatalf("next = %+v, want %+v", next, wantActions)
	}
}

func TestGuidanceUsesKnownAnchorsAndParentChildrenActions(t *testing.T) {
	input := GuidanceInput{
		Plan:         query.Plan{PrimaryIntent: query.IntentDefinition},
		KnownHandles: []string{"known-target"},
		Selected:     []GuidanceSelection{{Candidate: guidanceCandidate("child", RoleContext, "parent", "known-target"), Fidelity: FidelitySignature}},
		Omitted:      []ClassifiedCandidate{guidanceCandidate("grandchild", RoleContext, "children", "child")},
	}
	limitations, next := BuildGuidance(input)
	if !containsString(limitations, "known_anchor_not_repeated") {
		t.Fatalf("known anchor limitation absent: %v", limitations)
	}
	want := []NextAction{
		{Handle: "known-target", Relation: "parent", Reason: "parent_context_available"},
		{Handle: "child", Relation: "children", Reason: "children_available"},
		{Handle: "child", Relation: "self", Reason: "source_body_omitted"},
	}
	if !reflect.DeepEqual(next, want) {
		t.Fatalf("next = %+v, want %+v", next, want)
	}
}

func TestGuidanceIsBoundedDeduplicatedAndDeterministic(t *testing.T) {
	omitted := []ClassifiedCandidate{
		guidanceCandidate("caller-1", RoleCaller, "callers", "target"),
		guidanceCandidate("caller-2", RoleCaller, "callers", "target"),
		guidanceCandidate("callee", RoleCallee, "callees", "target"),
		guidanceCandidate("test", RoleTest, "tests", "target"),
		guidanceCandidate("import", RoleImport, "imports", "target"),
		guidanceCandidate("reference", RoleReference, "references", "target"),
	}
	input := GuidanceInput{Plan: query.Plan{PrimaryIntent: query.IntentCallers}, Truncated: true, Selected: []GuidanceSelection{{Candidate: guidanceCandidate("target", RoleTarget, "", ""), Fidelity: FidelityVerbatim}}, Omitted: omitted}
	limitations, next := BuildGuidance(input)
	if len(limitations) > 8 || len(next) > 4 {
		t.Fatalf("guidance exceeds bounds: limitations=%v next=%v", limitations, next)
	}
	reversed := append([]ClassifiedCandidate(nil), omitted...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	input.Omitted = reversed
	limitations2, next2 := BuildGuidance(input)
	if !reflect.DeepEqual(limitations2, limitations) || !reflect.DeepEqual(next2, next) {
		t.Fatalf("guidance depends on input order\nfirst=%v %+v\nsecond=%v %+v", limitations, next, limitations2, next2)
	}
}

func TestGuidanceMarksLexicalAndSyntaxOnlyLimitations(t *testing.T) {
	lexical := guidanceCandidate("caller", RoleCaller, "callers", "target")
	lexical.Candidate.RelationContext.Resolved = false
	limitations, _ := BuildGuidance(GuidanceInput{Plan: query.Plan{PrimaryIntent: query.IntentImpact}, Selected: []GuidanceSelection{{Candidate: lexical, Fidelity: FidelityExcerpt}}})
	for _, want := range []string{"source_reduced_to_excerpt", "lexical_relation_only", "syntax_only_impact"} {
		if !containsString(limitations, want) {
			t.Fatalf("limitation %q absent: %v", want, limitations)
		}
	}
}
