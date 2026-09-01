package benchmark

import (
	"encoding/json"
	"fmt"
	"strings"
)

const DiagnosisSchemaV1 = "focalspan.benchmark-diagnosis.v1"

const (
	DiagnosisLabelNotIndexed    = "label_not_indexed"
	DiagnosisPacked             = "packed"
	DiagnosisPackingDropped     = "packing_dropped"
	DiagnosisLinkingUnresolved  = "linking_unresolved"
	DiagnosisRankingDropped     = "ranking_dropped"
	DiagnosisSymbolMatchMissing = "symbol_match_missing"
	DiagnosisPathScopeMissing   = "path_scope_missing"
)

type DiagnosisPathHit struct {
	Retriever     string `json:"retriever"`
	FirstPosition int    `json:"first_position"`
	Count         int    `json:"count"`
}

type DiagnosisLabel struct {
	Expectation      string             `json:"expectation"`
	Path             string             `json:"path"`
	Symbol           string             `json:"symbol,omitempty"`
	Kind             string             `json:"kind,omitempty"`
	Relation         string             `json:"relation,omitempty"`
	AttributionStage string             `json:"attribution_stage"`
	DiagnosticStage  string             `json:"diagnostic_stage"`
	ReasonCode       string             `json:"reason_code"`
	PathHits         []DiagnosisPathHit `json:"path_hits,omitempty"`
	RetrieverHits    []AttributionHit   `json:"retriever_hits,omitempty"`
	RankedPosition   int                `json:"ranked_position,omitempty"`
	PackedPosition   int                `json:"packed_position,omitempty"`
}

type DiagnosisResult struct {
	Schema       string           `json:"schema"`
	CaseID       string           `json:"case_id"`
	RepositoryID string           `json:"repository_id"`
	Profile      string           `json:"profile"`
	Budget       int              `json:"budget"`
	Labels       []DiagnosisLabel `json:"labels"`
}

func CompileDiagnosisLabels(input AttributionInput, attribution []AttributionLabel) ([]DiagnosisLabel, error) {
	labels := make([]DiagnosisLabel, 0, len(attribution))
	for _, source := range attribution {
		expectation := AttributionExpectation{Expectation: source.Expectation, Path: source.Path, Symbol: source.Symbol, Kind: source.Kind, Relation: source.Relation}
		if err := validateAttributionExpectation(expectation); err != nil {
			return nil, err
		}
		label := DiagnosisLabel{
			Expectation: source.Expectation, Path: source.Path, Symbol: source.Symbol, Kind: source.Kind, Relation: source.Relation,
			AttributionStage: source.TerminalStage, RetrieverHits: append([]AttributionHit(nil), source.RetrieverHits...),
			RankedPosition: source.RankedPosition, PackedPosition: source.PackedPosition,
			PathHits: diagnosisPathHits(source.Path, input.Retrieved),
		}
		switch {
		case source.TerminalStage == StageLabelNotIndexed:
			label.DiagnosticStage, label.ReasonCode = DiagnosisLabelNotIndexed, "label_not_indexed"
		case source.PackedPosition > 0:
			label.DiagnosticStage, label.ReasonCode = DiagnosisPacked, "selected_in_packet"
		case source.RankedPosition > 0:
			label.DiagnosticStage, label.ReasonCode = DiagnosisPackingDropped, "omitted_by_packer"
		case source.TerminalStage == StageLinkingUnresolved:
			label.DiagnosticStage, label.ReasonCode = DiagnosisLinkingUnresolved, "relation_unresolved"
		case len(source.RetrieverHits) > 0:
			label.DiagnosticStage, label.ReasonCode = DiagnosisRankingDropped, "removed_before_rank"
		case source.TerminalStage == StageRetrievalMissing && source.Expectation != "required_path" && len(label.PathHits) > 0:
			label.DiagnosticStage, label.ReasonCode = DiagnosisSymbolMatchMissing, "expected_path_retrieved_identity_missing"
		default:
			label.DiagnosticStage, label.ReasonCode = DiagnosisPathScopeMissing, "expected_path_not_retrieved"
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func diagnosisPathHits(path string, observations []AttributionObservation) []DiagnosisPathHit {
	var hits []DiagnosisPathHit
	indexes := make(map[string]int)
	for _, observation := range observations {
		if observation.Path != path || !validRetriever(observation.Retriever) || observation.Position <= 0 {
			continue
		}
		index, exists := indexes[observation.Retriever]
		if !exists {
			indexes[observation.Retriever] = len(hits)
			hits = append(hits, DiagnosisPathHit{Retriever: observation.Retriever, FirstPosition: observation.Position, Count: 1})
			continue
		}
		hits[index].Count++
	}
	return hits
}

func MarshalDiagnosis(results []DiagnosisResult) ([]byte, error) {
	if err := validateDiagnosisResults(results); err != nil {
		return nil, err
	}
	return json.MarshalIndent(results, "", "  ")
}

func RenderDiagnosisMarkdown(results []DiagnosisResult) (string, error) {
	if err := validateDiagnosisResults(results); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# FocalSpan benchmark diagnosis\n\n")
	for _, result := range results {
		fmt.Fprintf(&b, "## %s / %s / %d\n\n", escapeTable(result.CaseID), escapeTable(result.Profile), result.Budget)
		fmt.Fprintf(&b, "- Schema: `%s`\n- Repository: `%s`\n\n", result.Schema, escapeTable(result.RepositoryID))
		b.WriteString("| Expectation | Path | Symbol | Kind | Relation | Attribution stage | Diagnostic stage | Reason | Path hits | Retriever hits | Ranked | Packed |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|---|---|---:|---:|\n")
		for _, label := range result.Labels {
			pathHits := make([]string, 0, len(label.PathHits))
			for _, hit := range label.PathHits {
				pathHits = append(pathHits, fmt.Sprintf("%s:%d:%d", hit.Retriever, hit.FirstPosition, hit.Count))
			}
			retrieverHits := make([]string, 0, len(label.RetrieverHits))
			for _, hit := range label.RetrieverHits {
				retrieverHits = append(retrieverHits, fmt.Sprintf("%s:%d:%s", hit.Retriever, hit.Position, hit.RelationState))
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %d | %d |\n",
				escapeTable(label.Expectation), escapeTable(label.Path), escapeTable(label.Symbol), escapeTable(label.Kind), escapeTable(label.Relation),
				label.AttributionStage, label.DiagnosticStage, label.ReasonCode, escapeTable(strings.Join(pathHits, ", ")),
				escapeTable(strings.Join(retrieverHits, ", ")), label.RankedPosition, label.PackedPosition)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func validateDiagnosisResults(results []DiagnosisResult) error {
	for resultIndex, result := range results {
		if result.Schema != DiagnosisSchemaV1 {
			return fmt.Errorf("diagnosis result %d: unsupported schema %q", resultIndex, result.Schema)
		}
		if result.CaseID == "" || result.RepositoryID == "" || result.Profile == "" || result.Budget <= 0 {
			return fmt.Errorf("diagnosis result %d: identifiers and positive budget are required", resultIndex)
		}
		if !safeAttributionText(result.CaseID) || !safeAttributionText(result.RepositoryID) || !safeAttributionText(result.Profile) {
			return fmt.Errorf("diagnosis result %d: identifiers contain control characters", resultIndex)
		}
		for labelIndex, label := range result.Labels {
			expectation := AttributionExpectation{Expectation: label.Expectation, Path: label.Path, Symbol: label.Symbol, Kind: label.Kind, Relation: label.Relation}
			if err := validateAttributionExpectation(expectation); err != nil {
				return fmt.Errorf("diagnosis result %d label %d: %w", resultIndex, labelIndex, err)
			}
			if !validDiagnosisStageReason(label.DiagnosticStage, label.ReasonCode) || !validDiagnosisAttributionPair(label.AttributionStage, label.DiagnosticStage) {
				return fmt.Errorf("diagnosis result %d label %d: unsupported attribution/diagnosis/reason %q/%q/%q", resultIndex, labelIndex, label.AttributionStage, label.DiagnosticStage, label.ReasonCode)
			}
			if label.Expectation == "required_path" && label.DiagnosticStage == DiagnosisSymbolMatchMissing {
				return fmt.Errorf("diagnosis result %d label %d: required path cannot be symbol-match missing", resultIndex, labelIndex)
			}
			if label.RankedPosition < 0 || label.PackedPosition < 0 {
				return fmt.Errorf("diagnosis result %d label %d: positions must not be negative", resultIndex, labelIndex)
			}
			seenPathRetrievers := make(map[string]struct{}, len(label.PathHits))
			for hitIndex, hit := range label.PathHits {
				if !validRetriever(hit.Retriever) || hit.FirstPosition <= 0 || hit.Count <= 0 {
					return fmt.Errorf("diagnosis result %d label %d path hit %d: invalid sanitized hit", resultIndex, labelIndex, hitIndex)
				}
				if _, exists := seenPathRetrievers[hit.Retriever]; exists {
					return fmt.Errorf("diagnosis result %d label %d path hit %d: duplicate retriever", resultIndex, labelIndex, hitIndex)
				}
				seenPathRetrievers[hit.Retriever] = struct{}{}
			}
			for hitIndex, hit := range label.RetrieverHits {
				if !validRetriever(hit.Retriever) || hit.Position <= 0 || !validRelationState(hit.RelationState) {
					return fmt.Errorf("diagnosis result %d label %d retriever hit %d: invalid sanitized hit", resultIndex, labelIndex, hitIndex)
				}
			}
		}
	}
	return nil
}

func validDiagnosisStageReason(stage, reason string) bool {
	want := map[string]string{
		DiagnosisLabelNotIndexed: "label_not_indexed", DiagnosisPacked: "selected_in_packet",
		DiagnosisPackingDropped: "omitted_by_packer", DiagnosisLinkingUnresolved: "relation_unresolved",
		DiagnosisRankingDropped: "removed_before_rank", DiagnosisSymbolMatchMissing: "expected_path_retrieved_identity_missing",
		DiagnosisPathScopeMissing: "expected_path_not_retrieved",
	}
	return want[stage] == reason
}

func validDiagnosisAttributionPair(attributionStage, diagnosisStage string) bool {
	switch attributionStage {
	case StageLabelNotIndexed:
		return diagnosisStage == DiagnosisLabelNotIndexed
	case StagePacked:
		return diagnosisStage == DiagnosisPacked
	case StagePackingDropped:
		return diagnosisStage == DiagnosisPackingDropped
	case StageLinkingUnresolved:
		return diagnosisStage == DiagnosisLinkingUnresolved
	case StageRankingDropped:
		return diagnosisStage == DiagnosisRankingDropped
	case StageRetrievalMissing:
		return diagnosisStage == DiagnosisSymbolMatchMissing || diagnosisStage == DiagnosisPathScopeMissing
	default:
		return false
	}
}
