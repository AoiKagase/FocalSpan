package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
)

func TestEvaluateEvidenceMeasuresContractComparisonAndDelta(t *testing.T) {
	root := t.TempDir()
	content := "package auth\n\nfunc Authenticate() error { return ValidateToken(\"token\") }\n\nfunc ValidateToken(token string) error {\n" + strings.Repeat("\t_ = token\n", 35) + "\tif token == \"expired\" { return ErrExpiredToken }\n\treturn nil\n}\n\nvar ErrExpiredToken = errors.New(\"expired\")\n"
	if err := os.WriteFile(filepath.Join(root, "auth.go"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewWithConfigForUpdate(root, cfg)
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
	cfg, _, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewWithConfigForUpdate(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	noResult, err := service.QueryEvidenceAttributed(context.Background(), app.EvidenceQueryRequest{Query: "zzzz_no_such_focalspan_symbol_7f3d", TokenBudget: 700, Mode: evidence.ModeFocused, NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(noResult.Compile.Packet.Evidence) != 0 {
		t.Fatalf("no-result trace retained Evidence: packet=%+v trace=%+v", noResult.Compile.Packet, noResult.Trace)
	}
	report, err := EvaluateEvidence(context.Background(), service, cases, false)
	if err != nil {
		t.Fatal(err)
	}
	const baseline = 0.5578351609480015
	foundDelta, foundLong, foundEmpty := false, false, false
	for _, result := range report.Cases {
		switch result.Name {
		case "go-stateless-delta":
			if result.DeltaTokenRatio <= 0 || result.DeltaTokenRatio >= baseline {
				t.Fatalf("delta ratio=%v, want less than baseline %v", result.DeltaTokenRatio, baseline)
			}
			foundDelta = true
		case "go-late-expired-token":
			if result.ExpectedCoverage != 1 || result.FocusedLateHit != 1 {
				t.Fatalf("long focused case=%+v", result)
			}
			foundLong = true
		case "no-relevant-source":
			if result.EmptyPacketCorrect != 1 || result.EvidenceItems != 0 {
				t.Fatalf("no-result query did not abstain: %+v", result)
			}
			foundEmpty = true
		}
	}
	if !foundDelta || !foundLong || !foundEmpty {
		t.Fatalf("coverage cases delta/long/empty=%t/%t/%t", foundDelta, foundLong, foundEmpty)
	}
	if report.EmptyPacketValidity != 1 {
		t.Fatalf("empty packet validity=%v", report.EmptyPacketValidity)
	}
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
