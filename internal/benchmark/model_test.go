package benchmark

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSuiteValid(t *testing.T) {
	suite, err := LoadSuite(filepath.Join("..", "..", "testdata", "benchmark", "schema-valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	if suite.Schema != SuiteSchemaV1 || suite.Name != "valid" || len(suite.Cases) != 1 {
		t.Fatalf("suite = %+v", suite)
	}
	case0 := suite.Cases[0]
	if case0.ID != "callers" || case0.RequiredPaths[0] != "internal/app/service.go" || case0.RequiredSymbols[0].Role != "target" {
		t.Fatalf("case = %+v", case0)
	}
}

func TestValidateSuiteReportsDeterministicErrors(t *testing.T) {
	invalid := Suite{Schema: "wrong", Cases: []Case{{
		ID: "duplicate", Repository: "", BaseRef: "same", TargetRef: "same",
		Budgets: []int{128, 128}, RequiredPaths: []string{"C:/absolute", "../escape"},
		ForbiddenPaths:  []string{"C:/absolute"},
		RequiredSymbols: []SymbolExpectation{{Path: "", Name: "", Role: "unknown"}},
		Expand:          []ExpandExpectation{{Relation: "unknown", Budget: 1}},
	}, {ID: "duplicate", Repository: "self", BaseRef: "a", TargetRef: "b", Query: "q", Budgets: []int{2048}}}}

	err1 := ValidateSuite(invalid)
	err2 := ValidateSuite(invalid)
	if err1 == nil || err2 == nil || err1.Error() != err2.Error() {
		t.Fatalf("errors not deterministic: %v / %v", err1, err2)
	}
	for _, fragment := range []string{"schema", "name", "duplicate case id", "repository", "query", "base_ref and target_ref", "budget", "absolute", "..", "required and forbidden", "symbol", "role", "relation", "expansion budget"} {
		if !strings.Contains(err1.Error(), fragment) {
			t.Errorf("error %q missing %q", err1, fragment)
		}
	}
}

func TestNormalizeSuiteRoundTripDeterministic(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "benchmark", "schema-valid.json")
	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := json.MarshalIndent(NormalizeSuite(suite), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var decoded Suite
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	second, err := json.MarshalIndent(NormalizeSuite(decoded), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("round trip changed bytes\n%s\n%s", first, second)
	}
}

func TestLoadRegistry(t *testing.T) {
	registry, err := LoadRegistry(filepath.Join("..", "..", "testdata", "benchmark", "registry-valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	if registry.Schema != SuiteSchemaV1 || registry.Repositories["self"] != "." {
		t.Fatalf("registry = %+v", registry)
	}
}
