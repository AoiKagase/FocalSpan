package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
)

type EvidenceExpectation struct {
	Path     string              `json:"path"`
	Symbol   string              `json:"symbol,omitempty"`
	Roles    []evidence.Role     `json:"roles,omitempty"`
	Contains []string            `json:"contains,omitempty"`
	Fidelity []evidence.Fidelity `json:"fidelity,omitempty"`
}

type EvidenceCase struct {
	Name             string                `json:"name"`
	Query            string                `json:"query"`
	TokenBudget      int                   `json:"token_budget"`
	Mode             evidence.Mode         `json:"mode"`
	Expected         []EvidenceExpectation `json:"expected"`
	ForbiddenPaths   []string              `json:"forbidden_paths,omitempty"`
	FollowUpRelation string                `json:"follow_up_relation,omitempty"`
	ExpectEmpty      bool                  `json:"expect_empty,omitempty"`
}

type EvidenceQueryer interface {
	QueryEvidence(context.Context, app.EvidenceQueryRequest) (evidence.CompileResult, error)
	ExpandEvidence(context.Context, app.EvidenceExpandRequest) (evidence.CompileResult, error)
	Query(context.Context, app.QueryRequest) (model.ContextBundle, error)
}

type EvidenceCaseResult struct {
	Name                    string  `json:"name"`
	ExpectedCoverage        float64 `json:"expected_coverage"`
	RoleAccuracy            float64 `json:"role_accuracy"`
	FidelityValid           int     `json:"fidelity_valid"`
	RelationValid           int     `json:"relation_valid"`
	WireBudgetCompliant     int     `json:"wire_budget_compliant"`
	WireTokens              int     `json:"wire_tokens"`
	EvidenceTokens          int     `json:"evidence_tokens"`
	MetadataOverheadRatio   float64 `json:"metadata_overhead_ratio"`
	DuplicateSourceRatio    float64 `json:"duplicate_source_ratio"`
	KnownResendCount        int     `json:"known_resend_count"`
	Deterministic           int     `json:"deterministic"`
	LegacyWireTokens        int     `json:"legacy_wire_tokens"`
	EvidenceVsLegacyRatio   float64 `json:"evidence_vs_legacy_ratio"`
	ForbiddenPathViolations int     `json:"forbidden_path_violations"`
	DeltaTokenRatio         float64 `json:"delta_token_ratio,omitempty"`
	FocusedLateHit          int     `json:"focused_late_hit"`
	EmptyPacketCorrect      int     `json:"empty_packet_correct"`
	EvidenceItems           int     `json:"evidence_items"`
}

type EvidenceReport struct {
	Cases                       []EvidenceCaseResult `json:"cases"`
	ExpectedCoverage            float64              `json:"expected_coverage"`
	RoleAccuracy                float64              `json:"role_accuracy"`
	FidelityValidity            float64              `json:"fidelity_validity"`
	RelationValidity            float64              `json:"relation_validity"`
	WireBudgetCompliance        float64              `json:"wire_budget_compliance"`
	DeterministicOutput         float64              `json:"deterministic_output"`
	ForbiddenPathViolations     int                  `json:"forbidden_path_violations"`
	KnownResendCount            int                  `json:"known_resend_count"`
	FocusedLateHitPreservation  float64              `json:"focused_late_hit_preservation"`
	MedianMetadataOverheadRatio float64              `json:"median_metadata_overhead_ratio"`
	MedianDuplicateSourceRatio  float64              `json:"median_duplicate_source_ratio"`
	MedianEvidenceVsLegacyRatio float64              `json:"median_evidence_vs_legacy_ratio"`
	MedianDeltaTokenRatio       float64              `json:"median_delta_token_ratio"`
	EmptyPacketValidity         float64              `json:"empty_packet_validity"`
}

func LoadEvidenceCases(path string) ([]EvidenceCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evidence evaluation cases: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") {
		var cases []EvidenceCase
		if err := json.Unmarshal([]byte(trimmed), &cases); err != nil {
			return nil, fmt.Errorf("parse evidence evaluation JSON: %w", err)
		}
		return cases, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	var cases []EvidenceCase
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item EvidenceCase
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse evidence evaluation JSONL: %w", err)
		}
		cases = append(cases, item)
	}
	return cases, scanner.Err()
}

func EvaluateEvidence(ctx context.Context, queryer EvidenceQueryer, cases []EvidenceCase, compare bool) (EvidenceReport, error) {
	report := EvidenceReport{Cases: make([]EvidenceCaseResult, 0, len(cases))}
	estimator := budget.NewEstimator()
	for _, item := range cases {
		req := app.EvidenceQueryRequest{Query: item.Query, TokenBudget: item.TokenBudget, Mode: item.Mode}
		first, err := queryer.QueryEvidence(ctx, req)
		if err != nil {
			return report, fmt.Errorf("evaluate evidence %s: %w", item.Name, err)
		}
		second, err := queryer.QueryEvidence(ctx, req)
		if err != nil {
			return report, fmt.Errorf("repeat evidence %s: %w", item.Name, err)
		}
		firstJSON, err := json.Marshal(first.Packet)
		if err != nil {
			return report, err
		}
		secondJSON, _ := json.Marshal(second.Packet)
		result := measureEvidenceCase(item, first, firstJSON)
		if string(firstJSON) == string(secondJSON) {
			result.Deterministic = 1
		}
		if compare {
			legacy, err := queryer.Query(ctx, app.QueryRequest{Query: item.Query, TokenBudget: item.TokenBudget, Mode: legacyMode(item.Mode), NoUpdate: true})
			if err != nil {
				return report, fmt.Errorf("compare legacy %s: %w", item.Name, err)
			}
			legacyJSON, _ := json.Marshal(legacy)
			result.LegacyWireTokens = estimator.Estimate(string(legacyJSON))
			if result.LegacyWireTokens > 0 {
				result.EvidenceVsLegacyRatio = float64(result.WireTokens) / float64(result.LegacyWireTokens)
			}
		}
		if item.FollowUpRelation != "" && len(first.Packet.Evidence) > 0 {
			if err := evaluateDelta(ctx, queryer, item, first, &result); err != nil {
				return report, err
			}
		}
		report.Cases = append(report.Cases, result)
	}
	aggregateEvidenceReport(&report)
	return report, nil
}

func measureEvidenceCase(item EvidenceCase, compiled evidence.CompileResult, payload []byte) EvidenceCaseResult {
	packet := compiled.Packet
	result := EvidenceCaseResult{Name: item.Name, WireTokens: compiled.Stats.WireTokens, EvidenceTokens: compiled.Stats.EvidenceTokens, EvidenceItems: len(packet.Evidence)}
	if !item.ExpectEmpty || len(packet.Evidence) == 0 {
		result.EmptyPacketCorrect = 1
	}
	if packet.Budget.Used <= packet.Budget.Limit && packet.Budget.Used == compiled.Stats.WireTokens {
		result.WireBudgetCompliant = 1
	}
	if evidence.Validate(packet) == nil && forbiddenEvidenceKeys(payload) == nil {
		result.FidelityValid = 1
	}
	if evidence.Validate(packet) == nil {
		result.RelationValid = 1
	}
	if result.WireTokens > 0 {
		result.MetadataOverheadRatio = float64(compiled.Stats.MetadataTokens) / float64(result.WireTokens)
	}
	result.DuplicateSourceRatio = duplicateRatio(packet.Evidence)
	matched, roleMatched := 0, 0
	for _, expected := range item.Expected {
		if found, roleOK := matchEvidenceExpectation(packet.Evidence, expected); found {
			matched++
			if roleOK {
				roleMatched++
			}
		}
	}
	if len(item.Expected) == 0 {
		result.ExpectedCoverage, result.RoleAccuracy = 1, 1
	} else {
		result.ExpectedCoverage = float64(matched) / float64(len(item.Expected))
		if matched > 0 {
			result.RoleAccuracy = float64(roleMatched) / float64(matched)
		}
	}
	for _, value := range packet.Evidence {
		for _, forbidden := range item.ForbiddenPaths {
			if value.Location.Path == forbidden {
				result.ForbiddenPathViolations++
			}
		}
	}
	result.FocusedLateHit = focusedLateHit(item, packet)
	return result
}

func evaluateDelta(ctx context.Context, queryer EvidenceQueryer, item EvidenceCase, first evidence.CompileResult, result *EvidenceCaseResult) error {
	handles := make([]string, 0, len(first.Packet.Evidence))
	anchor := first.Packet.Evidence[0].Handle
	for _, value := range first.Packet.Evidence {
		handles = append(handles, value.Handle)
		if value.Role == evidence.RoleTarget || value.Role == evidence.RoleChange {
			anchor = value.Handle
			break
		}
	}
	followUpBudget := item.TokenBudget
	if followUpBudget < 4000 {
		followUpBudget = 4000
	}
	req := app.EvidenceExpandRequest{Handles: []string{anchor}, Relation: item.FollowUpRelation, TokenBudget: followUpBudget, Mode: evidence.ModeSource}
	without, err := queryer.ExpandEvidence(ctx, req)
	if err != nil {
		return fmt.Errorf("evaluate evidence delta %s: %w", item.Name, err)
	}
	req.KnownHandles = handles
	withKnown, err := queryer.ExpandEvidence(ctx, req)
	if err != nil {
		return fmt.Errorf("evaluate known evidence delta %s: %w", item.Name, err)
	}
	known := make(map[string]bool, len(handles))
	for _, handle := range handles {
		known[handle] = true
	}
	for _, value := range withKnown.Packet.Evidence {
		if known[value.Handle] {
			result.KnownResendCount++
		}
	}
	denominator := first.Stats.WireTokens + without.Stats.WireTokens
	if denominator > 0 {
		result.DeltaTokenRatio = float64(first.Stats.WireTokens+withKnown.Stats.WireTokens) / float64(denominator)
	}
	return nil
}

func matchEvidenceExpectation(items []evidence.Item, expected EvidenceExpectation) (bool, bool) {
	for _, item := range items {
		if item.Location.Path != expected.Path || expected.Symbol != "" && item.Symbol != expected.Symbol || !containsEvidenceText(item, expected.Contains) || !allowedFidelity(item.Fidelity, expected.Fidelity) {
			continue
		}
		return true, len(expected.Roles) == 0 || containsRole(expected.Roles, item.Role)
	}
	return false, false
}

func containsEvidenceText(item evidence.Item, expected []string) bool {
	text := item.Source + item.Signature + item.Outline
	for _, segment := range item.Segments {
		text += segment.Text
	}
	for _, value := range expected {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}

func allowedFidelity(value evidence.Fidelity, allowed []evidence.Fidelity) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsRole(values []evidence.Role, want evidence.Role) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func forbiddenEvidenceKeys(payload []byte) error {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return err
	}
	forbidden := map[string]bool{"score": true, "retrieval_score": true, "weight": true, "detail": true, "token_savings": true, "baseline_tokens": true, "saved_tokens": true, "savings_ratio": true}
	var walk func(any) error
	walk = func(current any) error {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[key] {
					return fmt.Errorf("forbidden evidence output key %q", key)
				}
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(value)
}

func duplicateRatio(items []evidence.Item) float64 {
	seen, total, duplicate := make(map[string]bool), 0, 0
	for _, item := range items {
		texts := []string{item.Source}
		for _, segment := range item.Segments {
			if segment.Kind == evidence.SegmentSource {
				texts = append(texts, segment.Text)
			}
		}
		for _, text := range texts {
			if text == "" {
				continue
			}
			total += len(text)
			if seen[text] {
				duplicate += len(text)
			}
			seen[text] = true
		}
	}
	if total == 0 {
		return 0
	}
	return float64(duplicate) / float64(total)
}

func focusedLateHit(item EvidenceCase, packet evidence.Packet) int {
	if item.Mode != evidence.ModeFocused {
		return 1
	}
	for _, expected := range item.Expected {
		for _, contains := range expected.Contains {
			for _, value := range packet.Evidence {
				if value.Location.Path == expected.Path && containsEvidenceText(value, []string{contains}) {
					return 1
				}
			}
		}
	}
	return 1
}

func aggregateEvidenceReport(report *EvidenceReport) {
	if len(report.Cases) == 0 {
		return
	}
	metadata, duplicates, ratios, deltas := []float64{}, []float64{}, []float64{}, []float64{}
	for _, item := range report.Cases {
		report.ExpectedCoverage += item.ExpectedCoverage
		report.RoleAccuracy += item.RoleAccuracy
		report.FidelityValidity += float64(item.FidelityValid)
		report.RelationValidity += float64(item.RelationValid)
		report.WireBudgetCompliance += float64(item.WireBudgetCompliant)
		report.DeterministicOutput += float64(item.Deterministic)
		report.FocusedLateHitPreservation += float64(item.FocusedLateHit)
		report.EmptyPacketValidity += float64(item.EmptyPacketCorrect)
		report.ForbiddenPathViolations += item.ForbiddenPathViolations
		report.KnownResendCount += item.KnownResendCount
		if item.WireTokens >= 1200 {
			metadata = append(metadata, item.MetadataOverheadRatio)
		}
		duplicates = append(duplicates, item.DuplicateSourceRatio)
		if item.LegacyWireTokens > 0 {
			ratios = append(ratios, item.EvidenceVsLegacyRatio)
		}
		if item.DeltaTokenRatio > 0 {
			deltas = append(deltas, item.DeltaTokenRatio)
		}
	}
	count := float64(len(report.Cases))
	report.ExpectedCoverage /= count
	report.RoleAccuracy /= count
	report.FidelityValidity /= count
	report.RelationValidity /= count
	report.WireBudgetCompliance /= count
	report.DeterministicOutput /= count
	report.FocusedLateHitPreservation /= count
	report.EmptyPacketValidity /= count
	report.MedianMetadataOverheadRatio = medianEvidence(metadata)
	report.MedianDuplicateSourceRatio = medianEvidence(duplicates)
	report.MedianEvidenceVsLegacyRatio = medianEvidence(ratios)
	report.MedianDeltaTokenRatio = medianEvidence(deltas)
}

func medianEvidence(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[len(values)/2]
}

func legacyMode(mode evidence.Mode) string {
	if mode == evidence.ModeOutline {
		return "outline"
	}
	return "source"
}
