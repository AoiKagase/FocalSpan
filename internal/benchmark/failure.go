package benchmark

func FailureCodes(r QualityResult) []string {
	var codes []string
	if r.IntentCorrect == 0 {
		codes = append(codes, "intent_mismatch")
	}
	if r.TargetRank == 0 {
		codes = append(codes, "target_not_selected")
	} else if r.TargetRank > 5 {
		codes = append(codes, "target_below_5")
	}
	if r.RequiredPathRecall < 1 {
		codes = append(codes, "required_path_missing")
	}
	if r.RequiredSymbolRecall < 1 {
		codes = append(codes, "required_symbol_missing")
	}
	if r.ForbiddenViolations > 0 {
		codes = append(codes, "forbidden_selected")
	}
	if r.RelationValid == 0 {
		codes = append(codes, "relation_invalid")
	}
	if r.RoleAccuracy < 1 {
		codes = append(codes, "role_mismatch")
	}
	for _, code := range r.FailureCodes {
		if code == "ranked_candidate_not_packed" {
			codes = append(codes, code)
		}
	}
	if r.BudgetCompliant == 0 {
		codes = append(codes, "budget_exceeded")
	}
	if r.Deterministic == 0 {
		codes = append(codes, "nondeterministic_output")
	}
	if r.ExpandRequiredPathRecall > 0 && r.ExpandRequiredPathRecall < 1 {
		codes = append(codes, "expansion_required_missing")
	}
	if r.KnownResendCount > 0 {
		codes = append(codes, "known_handle_resent")
	}
	return codes
}
