package benchmark

import (
	"reflect"
	"testing"
)

func TestFailureCodesStableOrder(t *testing.T) {
	r := QualityResult{TargetRank: 6, RequiredPathRecall: .5, RequiredSymbolRecall: 0, IntentCorrect: 0, RoleAccuracy: .5, RelationValid: 0, BudgetCompliant: 0, Deterministic: 0, ForbiddenViolations: 1, KnownResendCount: 1, FailureCodes: []string{"ranked_candidate_not_packed"}}
	want := []string{"intent_mismatch", "target_below_5", "required_path_missing", "required_symbol_missing", "forbidden_selected", "relation_invalid", "role_mismatch", "ranked_candidate_not_packed", "budget_exceeded", "nondeterministic_output", "known_handle_resent"}
	if got := FailureCodes(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
