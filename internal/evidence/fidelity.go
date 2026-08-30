package evidence

import (
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/query"
)

type ContentVariant struct {
	Fidelity       Fidelity
	Source         string
	Segments       []Segment
	Outline        string
	Signature      string
	EvidenceTokens int
}

func BuildVariants(candidate ClassifiedCandidate, plan query.Plan, mode Mode, estimator budget.TokenEstimator) []ContentVariant {
	if estimator == nil {
		estimator = budget.NewEstimator()
	}
	signature := strings.TrimSpace(candidate.Candidate.Signature)
	if signature == "" {
		signature = fallbackSignature(candidate)
	}
	verbatim := func() ContentVariant {
		return ContentVariant{Fidelity: FidelityVerbatim, Source: candidate.Candidate.Content, EvidenceTokens: estimator.Estimate(candidate.Candidate.Content)}
	}
	excerpt := func() ContentVariant {
		segments := focusedSegments(candidate, plan)
		var source strings.Builder
		for _, segment := range segments {
			if segment.Kind == SegmentSource {
				source.WriteString(segment.Text)
			}
		}
		return ContentVariant{Fidelity: FidelityExcerpt, Segments: segments, EvidenceTokens: estimator.Estimate(source.String())}
	}
	signatureVariant := ContentVariant{Fidelity: FidelitySignature, Signature: signature, EvidenceTokens: estimator.Estimate(signature)}
	synthetic := ContentVariant{Fidelity: FidelitySynthetic, Outline: candidate.Candidate.Content, EvidenceTokens: estimator.Estimate(candidate.Candidate.Content)}
	lineCount := len(indexLines(candidate.Candidate.Content))
	if lineCount == 0 && candidate.Candidate.EndLine >= candidate.Candidate.StartLine {
		lineCount = candidate.Candidate.EndLine - candidate.Candidate.StartLine + 1
	}
	isSynthetic := syntheticCandidate(candidate)

	variants := make([]ContentVariant, 0, 3)
	switch mode {
	case ModeOutline:
		if isSynthetic && synthetic.Outline != "" {
			variants = append(variants, synthetic)
		}
		variants = append(variants, signatureVariant)
	case ModeSource:
		if candidate.Candidate.Content != "" {
			variants = append(variants, verbatim())
			if lineCount > 1 {
				variants = append(variants, excerpt())
			}
		}
		variants = append(variants, signatureVariant)
	default:
		switch {
		case isSynthetic && candidate.Candidate.Content != "":
			variants = append(variants, synthetic)
		case compactVerbatimCandidate(candidate, lineCount) && candidate.Candidate.Content != "":
			variants = append(variants, verbatim())
		case candidate.Candidate.Content != "" && (candidate.Role == RoleTarget || candidate.Role == RoleImplementation || hasFocusedHits(candidate, plan)):
			variants = append(variants, excerpt())
		}
		variants = append(variants, signatureVariant)
	}
	return removeDuplicateVariants(variants)
}

func compactVerbatimCandidate(candidate ClassifiedCandidate, lineCount int) bool {
	if (candidate.Role == RoleTarget || candidate.Role == RoleImplementation) && lineCount < 40 {
		return true
	}
	return (candidate.Role == RoleCaller || candidate.Role == RoleCallee || candidate.Role == RoleTest) && lineCount < 24
}

func hasFocusedHits(candidate ClassifiedCandidate, plan query.Plan) bool {
	for _, line := range indexLines(candidate.Candidate.Content) {
		if distinctMatches(line.text, focusedTerms(plan)) > 0 {
			return true
		}
	}
	return false
}

func syntheticCandidate(candidate ClassifiedCandidate) bool {
	kind := strings.ToLower(candidate.Candidate.Kind)
	signature := strings.ToLower(candidate.Candidate.Signature)
	return strings.HasSuffix(kind, "-outline") || kind == "test-suite" || strings.Contains(signature, "synthetic outline")
}

func fallbackSignature(candidate ClassifiedCandidate) string {
	symbol := strings.TrimSpace(candidate.Candidate.Symbol)
	if symbol == "" {
		symbol = "unnamed"
	}
	kind := strings.TrimSpace(candidate.Candidate.Kind)
	if kind == "" {
		kind = "source"
	}
	return fmt.Sprintf("%s %s (%s:%d-%d)", kind, symbol, candidate.Candidate.Path, candidate.Candidate.StartLine, candidate.Candidate.EndLine)
}

func removeDuplicateVariants(variants []ContentVariant) []ContentVariant {
	result := make([]ContentVariant, 0, len(variants))
	seen := make(map[string]bool, len(variants))
	for _, variant := range variants {
		if variant.Fidelity == FidelityExcerpt && len(variant.Segments) == 0 {
			continue
		}
		key := string(variant.Fidelity) + "\x00" + variant.Source + "\x00" + variant.Outline + "\x00" + variant.Signature
		for _, segment := range variant.Segments {
			key += fmt.Sprintf("\x00%s:%d:%d:%s", segment.Kind, segment.Lines[0], segment.Lines[1], segment.Text)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, variant)
	}
	return result
}
