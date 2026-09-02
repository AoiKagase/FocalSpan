package evidence

import (
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func TestBuildVariantsImplementsModeAndFidelityRules(t *testing.T) {
	estimator := budget.NewEstimator()
	compact := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "compact", Path: "a.go", Kind: "method", Symbol: "Run", Signature: "func Run()", StartLine: 1, EndLine: 3, Content: "func Run() {\n\twork()\n}\n"}, Role: RoleTarget}
	large := compact
	large.Candidate.Handle = "large"
	large.Candidate.EndLine = 90
	large.Candidate.Content = "func Run() {\n" + strings.Repeat("\twork()\n", 80) + "\treturn ErrLate\n}\n"
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"ErrLate"}}}

	tests := []struct {
		name      string
		candidate ClassifiedCandidate
		mode      Mode
		wantFirst Fidelity
	}{
		{name: "focused compact target", candidate: compact, mode: ModeFocused, wantFirst: FidelityVerbatim},
		{name: "focused large target", candidate: large, mode: ModeFocused, wantFirst: FidelityExcerpt},
		{name: "source compact", candidate: compact, mode: ModeSource, wantFirst: FidelityVerbatim},
		{name: "outline symbol", candidate: compact, mode: ModeOutline, wantFirst: FidelitySignature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variants := BuildVariants(tt.candidate, plan, tt.mode, estimator)
			if len(variants) == 0 || variants[0].Fidelity != tt.wantFirst {
				t.Fatalf("variants = %+v, want first fidelity %q", variants, tt.wantFirst)
			}
			assertVariantRepresentations(t, variants)
		})
	}
}

func TestBuildVariantsFocusedLateHitFitsCommonAllowances(t *testing.T) {
	content := "func ValidateToken() error {\n" + strings.Repeat("\twork()\n", 130) + "\tif token.ExpiresAt.Before(now) {\n\t\treturn ErrExpiredToken\n\t}\n}\n"
	candidate := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "target", Path: "auth/service.go", Kind: "method", Symbol: "ValidateToken", Signature: "func ValidateToken() error", StartLine: 10, EndLine: 144, Content: content}, Role: RoleTarget}
	plan := query.Plan{Terms: query.Terms{Words: []string{"expired", "token"}, Identifiers: []string{"ErrExpiredToken"}}}
	variants := BuildVariants(candidate, plan, ModeFocused, budget.NewEstimator())
	for _, allowance := range []int{512, 1200, 4000} {
		var selected *ContentVariant
		for index := range variants {
			if variants[index].EvidenceTokens <= allowance {
				selected = &variants[index]
				break
			}
		}
		if selected == nil || !variantContains(*selected, "ErrExpiredToken") {
			t.Fatalf("allowance %d lost late branch: %+v", allowance, selected)
		}
	}
}

func TestBuildVariantsSupportsSyntheticAndBlankSignatureFallback(t *testing.T) {
	estimator := budget.NewEstimator()
	synthetic := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "outline", Path: "a.go", Kind: "module-outline", Symbol: "a", Signature: "synthetic outline", StartLine: 1, EndLine: 1, Content: "functions: Run"}, Role: RoleContext}
	variants := BuildVariants(synthetic, query.Plan{}, ModeOutline, estimator)
	if len(variants) == 0 || variants[0].Fidelity != FidelitySynthetic {
		t.Fatalf("synthetic variants = %+v", variants)
	}
	blank := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "blank", Path: "a.go", Kind: "function", Symbol: "Run", StartLine: 4, EndLine: 8, Content: "func Run() {}"}, Role: RoleContext}
	variants = BuildVariants(blank, query.Plan{}, ModeOutline, estimator)
	if len(variants) == 0 || variants[len(variants)-1].Signature == "" {
		t.Fatalf("blank signature fallback absent: %+v", variants)
	}
}

func assertVariantRepresentations(t *testing.T, variants []ContentVariant) {
	t.Helper()
	for _, variant := range variants {
		count := 0
		if variant.Source != "" {
			count++
		}
		if len(variant.Segments) != 0 {
			count++
		}
		if variant.Outline != "" {
			count++
		}
		if variant.Signature != "" {
			count++
		}
		if count != 1 {
			t.Fatalf("variant has %d representations: %+v", count, variant)
		}
	}
}

func variantContains(variant ContentVariant, text string) bool {
	if strings.Contains(variant.Source, text) || strings.Contains(variant.Outline, text) || strings.Contains(variant.Signature, text) {
		return true
	}
	for _, segment := range variant.Segments {
		if strings.Contains(segment.Text, text) {
			return true
		}
	}
	return false
}
