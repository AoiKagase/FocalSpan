package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
)

func TestEvaluateEvidenceMeasuresContractComparisonAndDelta(t *testing.T) {
	root := t.TempDir()
	content := "package auth\n\nfunc Authenticate() error { return ValidateToken(\"token\") }\n\nfunc ValidateToken(token string) error {\n" + strings.Repeat("\t_ = token\n", 35) + "\tif token == \"expired\" { return ErrExpiredToken }\n\treturn nil\n}\n\nvar ErrExpiredToken = errors.New(\"expired\")\n"
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	cases := []EvidenceCase{{
		Name: "late expired branch", Query: "ValidateToken expired callers", TokenBudget: 1600, Mode: evidence.ModeFocused,
		Expected:         []EvidenceExpectation{{Path: "auth.go", Symbol: "ValidateToken", Roles: []evidence.Role{evidence.RoleTarget}, Contains: []string{"ErrExpiredToken"}, Fidelity: []evidence.Fidelity{evidence.FidelityExcerpt, evidence.FidelityVerbatim}}},
		FollowUpRelation: "callers",
	}}
	report, err := EvaluateEvidence(context.Background(), service, cases, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 1 || report.ExpectedCoverage != 1 || report.RoleAccuracy != 1 || report.FidelityValidity != 1 || report.RelationValidity != 1 || report.WireBudgetCompliance != 1 || report.DeterministicOutput != 1 {
		t.Fatalf("report=%+v", report)
	}
	result := report.Cases[0]
	if result.LegacyWireTokens == 0 || result.EvidenceVsLegacyRatio <= 0 || result.KnownResendCount != 0 || result.DeltaTokenRatio <= 0 {
		t.Fatalf("case=%+v", result)
	}
}

func TestEvidenceFixtureDeltaRatioImprovesWithKnownGuidancePruning(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos", "evidencesample")
	cases, err := LoadEvidenceCases(filepath.Join("..", "..", "testdata", "eval", "evidence-cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	report, err := EvaluateEvidence(context.Background(), service, cases, false)
	if err != nil {
		t.Fatal(err)
	}
	const baseline = 0.5578351609480015
	for _, result := range report.Cases {
		if result.Name == "go-stateless-delta" {
			if result.DeltaTokenRatio <= 0 || result.DeltaTokenRatio >= baseline {
				t.Fatalf("delta ratio=%v, want less than baseline %v", result.DeltaTokenRatio, baseline)
			}
			return
		}
	}
	t.Fatal("go-stateless-delta case missing")
}

func TestForbiddenEvidenceKeysInspectsObjectKeysOnly(t *testing.T) {
	allowed, _ := json.Marshal(map[string]any{"source": "score token_savings are source words"})
	if err := forbiddenEvidenceKeys(allowed); err != nil {
		t.Fatalf("source text caused false positive: %v", err)
	}
	for _, key := range []string{"score", "retrieval_score", "weight", "detail", "token_savings", "baseline_tokens", "saved_tokens", "savings_ratio"} {
		payload, _ := json.Marshal(map[string]any{"nested": []any{map[string]any{key: 1}}})
		if err := forbiddenEvidenceKeys(payload); err == nil {
			t.Fatalf("forbidden key %q accepted", key)
		}
	}
}

func TestLoadEvidenceCasesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cases.jsonl")
	line := `{"name":"go","query":"ValidateToken","token_budget":1200,"mode":"focused","expected":[{"path":"auth.go","roles":["target"]}]}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	cases, err := LoadEvidenceCases(path)
	if err != nil || len(cases) != 1 || cases[0].Expected[0].Roles[0] != evidence.RoleTarget {
		t.Fatalf("cases=%+v err=%v", cases, err)
	}
}
