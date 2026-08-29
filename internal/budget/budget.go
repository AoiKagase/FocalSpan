package budget

import (
	"encoding/json"
	"math"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/model"
)

const (
	MinBudget = 256
	MaxBudget = 64000
)

type TokenEstimator interface{ Estimate(text string) int }

type Estimator struct{}

func NewEstimator() Estimator { return Estimator{} }

func (Estimator) Estimate(text string) int {
	if text == "" {
		return 0
	}
	ascii, nonASCII, punctuation, identifiers := 0, 0, 0, 0
	inIdentifier := false
	for _, r := range text {
		if r < unicode.MaxASCII {
			ascii++
		} else {
			nonASCII++
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if !inIdentifier {
				identifiers++
				inIdentifier = true
			}
		} else {
			inIdentifier = false
			if unicode.IsPunct(r) || unicode.IsSymbol(r) {
				punctuation++
			}
		}
	}
	base := float64(ascii)/4.0 + float64(nonASCII)/1.8 + float64(punctuation)/8.0 + float64(identifiers)/3.0 + 4
	return int(math.Ceil(base * 1.12))
}

type Packer struct{ estimator TokenEstimator }

func NewPacker(estimator TokenEstimator) *Packer {
	if estimator == nil {
		estimator = NewEstimator()
	}
	return &Packer{estimator: estimator}
}

func (p *Packer) Pack(req model.PackRequest) model.ContextBundle {
	budget := req.TokenBudget
	if budget < MinBudget {
		budget = MinBudget
	}
	if budget > MaxBudget {
		budget = MaxBudget
	}
	mode := req.Mode
	if mode != "outline" && mode != "source" {
		mode = "source"
	}
	bundle := model.ContextBundle{Query: req.Query, IndexRevision: req.IndexRevision, BudgetTokens: budget, Items: make([]model.ContextItem, 0)}
	testIntent := hasTestIntent(req.Query)
	hasTestCandidate := false
	hasNonDocumentationCandidate := false
	for _, candidate := range req.Candidates {
		if isTestCandidate(candidate) {
			hasTestCandidate = true
		}
		if !isDocumentationCandidate(candidate) {
			hasNonDocumentationCandidate = true
		}
	}
	testItems := 0
	relationItems := make(map[string]int)
	hasProductionCandidate := false
	for _, candidate := range req.Candidates {
		if !isDocumentationCandidate(candidate) && !isTestCandidate(candidate) {
			hasProductionCandidate = true
			break
		}
	}
	for index, candidate := range req.Candidates {
		if hasNonDocumentationCandidate && isDocumentationCandidate(candidate) {
			bundle.OmittedCount++
			bundle.Truncated = true
			continue
		}
		if testIntent && hasTestCandidate {
			if !isTestCandidate(candidate) {
				bundle.OmittedCount++
				bundle.Truncated = true
				continue
			}
			if testItems >= 2 {
				bundle.OmittedCount++
				bundle.Truncated = true
				continue
			}
			testItems++
		} else if !testIntent && hasProductionCandidate && candidate.Language != "" && isTestCandidate(candidate) {
			bundle.OmittedCount++
			bundle.Truncated = true
			continue
		}
		item := candidateItem(candidate, mode)
		if candidate.Relation != "" {
			if relationItems[candidate.Relation] > 0 && mode == "source" && item.Content != "" {
				item.Content = item.Signature
				if item.Content == "" {
					item.Content = "[...elided...]\n"
				}
				item.Elided = true
			}
			relationItems[candidate.Relation]++
		}
		if index > 0 && len(bundle.Items) > 0 && lowUtilityUnderPressure(p.estimator, bundle, item, candidate, req.Candidates[0], budget) {
			bundle.OmittedCount++
			bundle.Truncated = true
			continue
		}
		if mode == "source" && item.Content != "" {
			if estimateBundle(p.estimator, bundle, item) > budget || p.estimator.Estimate(item.Content) > budget*40/100 && index != 0 {
				item.Content = p.elide(candidate.Content, budget/3)
				item.Elided = true
			}
		}
		if estimateBundle(p.estimator, bundle, item) > budget && mode == "source" {
			item.Content = p.elide(candidate.Content, budget/5)
			item.Elided = true
		}
		if estimateBundle(p.estimator, bundle, item) > budget {
			item.Content = "[...elided...]"
			item.Elided = true
		}
		if estimateBundle(p.estimator, bundle, item) > budget {
			bundle.OmittedCount++
			bundle.Truncated = true
			continue
		}
		item.EstimatedTokens = p.estimator.Estimate(item.Content)
		bundle.Items = append(bundle.Items, item)
	}
	bundle.Truncated = bundle.Truncated || bundle.OmittedCount > 0
	for {
		if estimateBundle(p.estimator, bundle, model.ContextItem{}) <= budget {
			break
		}
		if len(bundle.Items) == 0 {
			break
		}
		bundle.Items = bundle.Items[:len(bundle.Items)-1]
		bundle.OmittedCount++
		bundle.Truncated = true
	}
	// The hard limit above is checked against the serialized JSON contract,
	// including diagnostics and score metadata. Report the size of the
	// source-oriented payload instead so savings compares source spans with
	// the full-file baseline rather than measuring JSON bookkeeping twice.
	bundle.EstimatedTokens = estimateSourcePayload(p.estimator, bundle)
	return bundle
}

func lowUtilityUnderPressure(estimator TokenEstimator, bundle model.ContextBundle, item model.ContextItem, candidate, top model.RankedCandidate, budget int) bool {
	for _, reason := range candidate.Reasons {
		if reason.Code == "symbol-exact" {
			return false
		}
	}
	if candidate.Relation == "" && candidate.Score > 0 && top.Score > 0 && candidate.Score*2 < top.Score {
		return true
	}
	if top.Score <= 0 || candidate.Score <= 0 || candidate.Score*4 >= top.Score {
		return false
	}
	if estimateBundle(estimator, bundle, item) <= budget*3/5 {
		return false
	}
	return true
}

func hasTestIntent(query string) bool {
	for _, term := range strings.Fields(strings.ToLower(query)) {
		term = strings.Trim(term, ".,?!:;()[]{}")
		switch term {
		case "test", "tests", "testing", "coverage", "cover", "spec", "specs":
			return true
		}
	}
	return false
}

func isDocumentationCandidate(candidate model.RankedCandidate) bool {
	return candidate.Kind == "heading" || candidate.Language == "markdown" || candidate.Language == "text"
}

func isTestCandidate(candidate model.RankedCandidate) bool {
	if candidate.Kind != "test" && candidate.Kind != "test-suite" && !strings.HasPrefix(candidate.Symbol, "Test") {
		return false
	}
	path := strings.ToLower(strings.ReplaceAll(candidate.Path, "\\", "/"))
	return strings.Contains(path, "/test") || strings.HasPrefix(path, "test") || strings.Contains(path, "_test.") || strings.Contains(path, ".test.") || strings.Contains(path, ".spec.")
}

func candidateItem(candidate model.RankedCandidate, mode string) model.ContextItem {
	content := candidate.Content
	if mode == "outline" {
		content = ""
	}
	return model.ContextItem{Handle: candidate.Handle, Path: candidate.Path, Language: candidate.Language, Kind: candidate.Kind, Symbol: candidate.Symbol, Signature: candidate.Signature, StartLine: candidate.StartLine, EndLine: candidate.EndLine, Score: candidate.Score, Reasons: candidate.Reasons, Relation: candidate.Relation, EstimatedTokens: candidate.EstimatedTokens, Content: content}
}

func estimateBundle(est TokenEstimator, bundle model.ContextBundle, extra model.ContextItem) int {
	copyBundle := bundle
	copyBundle.EstimatedTokens = 0
	if extra.Handle != "" {
		copyBundle.Items = append(append([]model.ContextItem(nil), bundle.Items...), extra)
	}
	b, _ := json.Marshal(copyBundle)
	return est.Estimate(string(b))
}

func estimateSourcePayload(est TokenEstimator, bundle model.ContextBundle) int {
	var b strings.Builder
	for _, item := range bundle.Items {
		if item.Content != "" {
			b.WriteString(item.Content)
		} else {
			b.WriteString(item.Signature)
		}
		b.WriteByte('\n')
	}
	return est.Estimate(b.String())
}

func (p *Packer) elide(content string, maxTokens int) string {
	if content == "" {
		return "[...elided...]\n"
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if maxTokens < 8 {
		return "[...elided...]\n"
	}
	for len(lines) > 0 {
		candidate := strings.Join(lines, "\n") + "\n[...elided...]\n"
		if p.estimator.Estimate(candidate) <= maxTokens {
			return candidate
		}
		lines = lines[:len(lines)-1]
	}
	return "[...elided...]\n"
}
