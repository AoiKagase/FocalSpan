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

func TestBuildVariantsAddsSmallerAdaptiveExcerptForLongFocusedSource(t *testing.T) {
	content := adaptiveLongContent()
	candidate := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "adaptive", Path: "service/worker.go", Kind: "method", Symbol: "Execute", Signature: "func Execute() error", StartLine: 20, EndLine: 20 + strings.Count(content, "\n"), Content: content}, Role: RoleTarget}
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"ErrFirst", "ErrSecond", "ErrThird"}}}
	variants := BuildVariants(candidate, plan, ModeFocused, budget.NewEstimator())
	excerpts := make([]ContentVariant, 0, 2)
	for _, variant := range variants {
		if variant.Fidelity == FidelityExcerpt {
			excerpts = append(excerpts, variant)
		}
	}
	if len(excerpts) < 2 {
		t.Fatalf("focused variants = %+v, want normal and adaptive excerpts", variants)
	}
	if excerpts[1].EvidenceTokens >= excerpts[0].EvidenceTokens {
		t.Fatalf("adaptive excerpt tokens=%d, normal=%d", excerpts[1].EvidenceTokens, excerpts[0].EvidenceTokens)
	}
	for _, hit := range []string{"ErrFirst", "ErrSecond", "ErrThird"} {
		if !variantContains(excerpts[1], hit) {
			t.Fatalf("adaptive excerpt lost required hit %q: %+v", hit, excerpts[1])
		}
	}
}

func TestAdaptiveFocusedSegmentsPreserveExactHitLines(t *testing.T) {
	content := strings.Join([]string{
		"func 検証() error {",
		strings.Repeat("\t通常処理\r\n", 24),
		"\treturn ErrLate\r\n",
		strings.Repeat("\t後続処理\r\n", 24),
		"}",
	}, "\r\n")
	candidate := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "adaptive", Path: "utf8/service.go", Kind: "method", Symbol: "検証", StartLine: 30, EndLine: 30 + strings.Count(content, "\n"), Content: content}, Role: RoleTarget}
	plan := query.Plan{Terms: query.Terms{Identifiers: []string{"ErrLate"}, UnicodeRuns: []string{"通常処理"}}}
	standard := focusedSegments(candidate, plan)
	adaptive := adaptiveFocusedSegments(candidate, plan)
	if len(adaptive) == 0 {
		t.Fatal("adaptiveFocusedSegments returned no segments")
	}
	standardSource, adaptiveSource := strings.Builder{}, strings.Builder{}
	for _, segment := range standard {
		if segment.Kind == SegmentSource {
			standardSource.WriteString(segment.Text)
		}
	}
	foundLate := false
	for _, segment := range adaptive {
		if segment.Kind == SegmentOmitted {
			if segment.Text != "" {
				t.Fatalf("omitted segment contains text: %+v", segment)
			}
			continue
		}
		want := testSourceLines(content, segment.Lines, candidate.Candidate.StartLine)
		if segment.Text != want {
			t.Fatalf("adaptive segment differs from source\n got: %q\nwant: %q", segment.Text, want)
		}
		adaptiveSource.WriteString(segment.Text)
		foundLate = foundLate || strings.Contains(segment.Text, "ErrLate")
	}
	if !foundLate {
		t.Fatalf("adaptive excerpt lost late hit: %+v", adaptive)
	}
	if budget.NewEstimator().Estimate(adaptiveSource.String()) >= budget.NewEstimator().Estimate(standardSource.String()) {
		t.Fatalf("adaptive source was not smaller: standard=%q adaptive=%q", standardSource.String(), adaptiveSource.String())
	}
}

func TestAdaptiveExcerptDoesNotChangeShortOrNonFocusedModes(t *testing.T) {
	short := ClassifiedCandidate{Candidate: model.RankedCandidate{Handle: "short", Path: "short.go", Kind: "function", Symbol: "Run", Signature: "func Run()", StartLine: 1, EndLine: 3, Content: "func Run() {\n\treturn nil\n}\n"}, Role: RoleTarget}
	focused := BuildVariants(short, query.Plan{Terms: query.Terms{Identifiers: []string{"Run"}}}, ModeFocused, budget.NewEstimator())
	for _, variant := range focused {
		if variant.Fidelity == FidelityExcerpt {
			t.Fatalf("short candidate unexpectedly gained excerpt: %+v", focused)
		}
	}
	source := BuildVariants(short, query.Plan{}, ModeSource, budget.NewEstimator())
	if len(source) == 0 || source[0].Fidelity != FidelityVerbatim {
		t.Fatalf("source variants changed: %+v", source)
	}
	outline := BuildVariants(short, query.Plan{}, ModeOutline, budget.NewEstimator())
	for _, variant := range outline {
		if variant.Fidelity == FidelityExcerpt {
			t.Fatalf("outline gained excerpt: %+v", outline)
		}
	}
}

func adaptiveLongContent() string {
	lines := []string{"func Execute() error {"}
	for index := 0; index < 18; index++ {
		lines = append(lines, "\tregularStep()")
	}
	lines = append(lines, "\tif first { return ErrFirst }")
	for index := 0; index < 18; index++ {
		lines = append(lines, "\tmiddleStep()")
	}
	lines = append(lines, "\tif second { return ErrSecond }")
	for index := 0; index < 18; index++ {
		lines = append(lines, "\tlateStep()")
	}
	lines = append(lines, "\tif third { return ErrThird }")
	lines = append(lines, "\treturn nil", "}")
	return strings.Join(lines, "\n") + "\n"
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
