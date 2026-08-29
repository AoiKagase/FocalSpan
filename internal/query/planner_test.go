package query

import (
	"reflect"
	"testing"
)

func TestPlanJapaneseCallerIntent(t *testing.T) {
	plan := PlanQuery(`ValidateToken の呼び出し元はどこですか`)
	if !plan.HasIntent(IntentCallers) || plan.PrimaryIntent != IntentCallers {
		t.Fatalf("plan=%+v, want callers as primary intent", plan)
	}
	if !containsIntent(plan.Relations, "callers") || !containsString(plan.Anchors, "ValidateToken") {
		t.Fatalf("plan=%+v, want callers relation and ValidateToken anchor", plan)
	}
}

func TestPlanJapaneseTestIntent(t *testing.T) {
	plan := PlanQuery(`ValidateTokenを検証するテスト`)
	if !plan.HasIntent(IntentTests) || plan.PrimaryIntent != IntentTests {
		t.Fatalf("plan=%+v, want tests as primary intent", plan)
	}
	if !containsIntent(plan.Relations, "tests") || !containsString(plan.Anchors, "ValidateToken") {
		t.Fatalf("plan=%+v, want tests relation and ValidateToken anchor", plan)
	}
}

func TestPlanEnglishCalleeIntent(t *testing.T) {
	plan := PlanQuery(`what does ValidateToken call?`)
	if !plan.HasIntent(IntentCallees) || plan.PrimaryIntent != IntentCallees {
		t.Fatalf("plan=%+v, want callees as primary intent", plan)
	}
	if plan.HasIntent(IntentCallers) || !reflect.DeepEqual(plan.Relations, []string{"callees"}) {
		t.Fatalf("plan=%+v, want only callees relation", plan)
	}
}

func TestPlanDoesNotTreatCallerIDAsCallerIntent(t *testing.T) {
	plan := PlanQuery(`find callerID`)
	if plan.HasIntent(IntentCallers) {
		t.Fatalf("plan=%+v, callerID must not imply callers intent", plan)
	}
}

func TestPlanUsesIntentPrecedenceAndRetainsAllIntents(t *testing.T) {
	plan := PlanQuery(`ValidateTokenの実装とテスト`)
	if plan.PrimaryIntent != IntentTests || !plan.HasIntent(IntentDefinition) || !plan.HasIntent(IntentTests) {
		t.Fatalf("plan=%+v, want tests primary and definition retained", plan)
	}
	if !reflect.DeepEqual(plan.Relations, []string{"tests"}) {
		t.Fatalf("relations=%v, want tests only", plan.Relations)
	}
}

func TestPlanRecognizesJapaneseImportsAndImpact(t *testing.T) {
	imports := PlanQuery(`auth.tsをimportしている箇所`)
	if imports.PrimaryIntent != IntentImports || !containsString(imports.Anchors, "auth.ts") || !containsIntent(imports.Relations, "imports") {
		t.Fatalf("imports=%+v", imports)
	}
	impact := PlanQuery(`変更すると何が壊れるか`)
	if impact.PrimaryIntent != IntentImpact || !reflect.DeepEqual(impact.Relations, []string{"callers", "tests", "references"}) {
		t.Fatalf("impact=%+v", impact)
	}
}

func TestPlanKeepsQualifiedAnchorsBeforeFallbackWords(t *testing.T) {
	plan := PlanQuery(`App\Auth\TokenService::ValidateToken の実装`)
	if len(plan.Anchors) == 0 || plan.Anchors[0] != `App\Auth\TokenService::ValidateToken` {
		t.Fatalf("anchors=%v, want qualified identifier first", plan.Anchors)
	}
	for _, anchor := range plan.Anchors {
		if anchor == "実装" || anchor == "の" {
			t.Fatalf("intent/particle leaked into anchors: %v", plan.Anchors)
		}
	}
}

func TestPlanDoesNotInferIntentFromIdentifierNames(t *testing.T) {
	for _, raw := range []string{`importToken`, `coverageReport`, `TestValidateToken`} {
		plan := PlanQuery(raw)
		if plan.HasIntent(IntentImports) || plan.HasIntent(IntentTests) {
			t.Fatalf("query=%q plan=%+v inferred intent from identifier", raw, plan)
		}
	}
}

func TestPlanRecognizesJapaneseImportInflection(t *testing.T) {
	plan := PlanQuery(`token-serviceを読み込んでいるmodule`)
	if !plan.HasIntent(IntentImports) {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestPlanIntentStringsAreCopiedAndDeterministic(t *testing.T) {
	first := PlanQuery(`what calls ValidateToken and what tests cover it?`)
	second := PlanQuery(`what calls ValidateToken and what tests cover it?`)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("plans differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
	intents := first.IntentStrings()
	intents[0] = "changed"
	if first.Intents[0] == "changed" {
		t.Fatal("IntentStrings returned an aliased slice")
	}
}

func containsIntent(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
