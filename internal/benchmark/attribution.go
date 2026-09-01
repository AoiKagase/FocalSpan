package benchmark

import (
	"encoding/json"
	"fmt"
	"strings"
)

const AttributionSchemaV1 = "focalspan.benchmark-attribution.v1"

const (
	StageLabelNotIndexed   = "label_not_indexed"
	StageRetrievalMissing  = "retrieval_missing"
	StageLinkingUnresolved = "linking_unresolved"
	StageRankingDropped    = "ranking_dropped"
	StagePackingDropped    = "packing_dropped"
	StagePacked            = "packed"
)

type AttributionExpectation struct {
	Expectation string
	Path        string
	Symbol      string
	Kind        string
	Relation    string
}

type AttributionIdentity struct {
	Path   string
	Symbol string
	Kind   string
}

type AttributionObservation struct {
	AttributionIdentity
	Retriever        string
	Position         int
	Relation         string
	RelationResolved bool
}

type AttributionInput struct {
	Indexed   []AttributionIdentity
	Retrieved []AttributionObservation
	Ranked    []AttributionIdentity
	Packed    []AttributionIdentity
}

type AttributionHit struct {
	Retriever     string `json:"retriever"`
	Position      int    `json:"position"`
	RelationState string `json:"relation_state"`
}

type AttributionLabel struct {
	Expectation    string           `json:"expectation"`
	Path           string           `json:"path"`
	Symbol         string           `json:"symbol,omitempty"`
	Kind           string           `json:"kind,omitempty"`
	Relation       string           `json:"relation,omitempty"`
	TerminalStage  string           `json:"terminal_stage"`
	ReasonCode     string           `json:"reason_code"`
	RetrieverHits  []AttributionHit `json:"retriever_hits,omitempty"`
	RankedPosition int              `json:"ranked_position,omitempty"`
	PackedPosition int              `json:"packed_position,omitempty"`
}

type AttributionResult struct {
	Schema       string             `json:"schema"`
	CaseID       string             `json:"case_id"`
	RepositoryID string             `json:"repository_id"`
	Profile      string             `json:"profile"`
	Budget       int                `json:"budget"`
	Labels       []AttributionLabel `json:"labels"`
}

func AttributeLabels(expectations []AttributionExpectation, input AttributionInput) ([]AttributionLabel, error) {
	labels := make([]AttributionLabel, 0, len(expectations))
	for _, expectation := range expectations {
		if err := validateAttributionExpectation(expectation); err != nil {
			return nil, err
		}
		label := AttributionLabel{
			Expectation: expectation.Expectation,
			Path:        expectation.Path,
			Symbol:      expectation.Symbol,
			Kind:        expectation.Kind,
			Relation:    expectation.Relation,
		}

		if !identityPresent(expectation, input.Indexed) {
			label.TerminalStage, label.ReasonCode = StageLabelNotIndexed, "label_not_indexed"
			labels = append(labels, label)
			continue
		}

		for _, observation := range input.Retrieved {
			if !identityMatches(expectation, observation.AttributionIdentity) {
				continue
			}
			state := "none"
			if observation.Relation != "" {
				if observation.RelationResolved {
					state = "resolved"
				} else {
					state = "unresolved"
				}
			}
			label.RetrieverHits = append(label.RetrieverHits, AttributionHit{
				Retriever: observation.Retriever, Position: observation.Position, RelationState: state,
			})
		}

		label.RankedPosition = identityPosition(expectation, input.Ranked)
		label.PackedPosition = identityPosition(expectation, input.Packed)
		switch {
		case label.PackedPosition > 0:
			label.TerminalStage, label.ReasonCode = StagePacked, "selected_in_packet"
		case label.RankedPosition > 0:
			label.TerminalStage, label.ReasonCode = StagePackingDropped, "omitted_by_packer"
		case hasOnlyUnresolvedRelation(label.RetrieverHits):
			label.TerminalStage, label.ReasonCode = StageLinkingUnresolved, "relation_unresolved"
		case len(label.RetrieverHits) > 0:
			label.TerminalStage, label.ReasonCode = StageRankingDropped, "removed_before_rank"
		default:
			label.TerminalStage, label.ReasonCode = StageRetrievalMissing, "no_retriever_match"
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func MarshalAttribution(results []AttributionResult) ([]byte, error) {
	if err := validateAttributionResults(results); err != nil {
		return nil, err
	}
	return json.MarshalIndent(results, "", "  ")
}

func RenderAttributionMarkdown(results []AttributionResult) (string, error) {
	if err := validateAttributionResults(results); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# FocalSpan benchmark attribution\n\n")
	for _, result := range results {
		fmt.Fprintf(&b, "## %s / %s / %d\n\n", escapeTable(result.CaseID), escapeTable(result.Profile), result.Budget)
		fmt.Fprintf(&b, "- Schema: `%s`\n- Repository: `%s`\n\n", result.Schema, escapeTable(result.RepositoryID))
		b.WriteString("| Expectation | Path | Symbol | Kind | Relation | Terminal stage | Reason | Retriever hits | Ranked | Packed |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|---:|---:|\n")
		for _, label := range result.Labels {
			hits := make([]string, 0, len(label.RetrieverHits))
			for _, hit := range label.RetrieverHits {
				hits = append(hits, fmt.Sprintf("%s:%d:%s", hit.Retriever, hit.Position, hit.RelationState))
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %d | %d |\n",
				escapeTable(label.Expectation), escapeTable(label.Path), escapeTable(label.Symbol), escapeTable(label.Kind),
				escapeTable(label.Relation), label.TerminalStage, label.ReasonCode, escapeTable(strings.Join(hits, ", ")),
				label.RankedPosition, label.PackedPosition)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func identityPresent(expectation AttributionExpectation, identities []AttributionIdentity) bool {
	return identityPosition(expectation, identities) > 0
}

func identityPosition(expectation AttributionExpectation, identities []AttributionIdentity) int {
	for index, identity := range identities {
		if identityMatches(expectation, identity) {
			return index + 1
		}
	}
	return 0
}

func identityMatches(expectation AttributionExpectation, identity AttributionIdentity) bool {
	if expectation.Path != identity.Path {
		return false
	}
	if expectation.Expectation == "required_path" {
		return true
	}
	return expectation.Symbol == identity.Symbol && (expectation.Kind == "" || expectation.Kind == identity.Kind)
}

func hasOnlyUnresolvedRelation(hits []AttributionHit) bool {
	if len(hits) == 0 {
		return false
	}
	for _, hit := range hits {
		if hit.RelationState != "unresolved" {
			return false
		}
	}
	return true
}

func validateAttributionExpectation(value AttributionExpectation) error {
	if value.Expectation != "required_path" && value.Expectation != "required_symbol" && value.Expectation != "expansion_anchor" {
		return fmt.Errorf("unsupported attribution expectation %q", value.Expectation)
	}
	if issue := pathIssue(value.Path); issue != "" {
		return fmt.Errorf("attribution path: %s", issue)
	}
	if value.Expectation != "required_path" && value.Symbol == "" {
		return fmt.Errorf("attribution symbol must not be empty")
	}
	if !safeAttributionText(value.Symbol) || !safeAttributionText(value.Kind) {
		return fmt.Errorf("attribution identity contains control characters")
	}
	if value.Relation != "" && !supportedRelation(value.Relation) {
		return fmt.Errorf("unsupported attribution relation %q", value.Relation)
	}
	return nil
}

func validateAttributionResults(results []AttributionResult) error {
	for resultIndex, result := range results {
		if result.Schema != AttributionSchemaV1 {
			return fmt.Errorf("attribution result %d: unsupported schema %q", resultIndex, result.Schema)
		}
		if result.CaseID == "" || result.RepositoryID == "" || result.Profile == "" || result.Budget <= 0 {
			return fmt.Errorf("attribution result %d: identifiers and positive budget are required", resultIndex)
		}
		if !safeAttributionText(result.CaseID) || !safeAttributionText(result.RepositoryID) || !safeAttributionText(result.Profile) {
			return fmt.Errorf("attribution result %d: identifiers contain control characters", resultIndex)
		}
		for labelIndex, label := range result.Labels {
			expectation := AttributionExpectation{Expectation: label.Expectation, Path: label.Path, Symbol: label.Symbol, Kind: label.Kind, Relation: label.Relation}
			if err := validateAttributionExpectation(expectation); err != nil {
				return fmt.Errorf("attribution result %d label %d: %w", resultIndex, labelIndex, err)
			}
			if !validStageReason(label.TerminalStage, label.ReasonCode) {
				return fmt.Errorf("attribution result %d label %d: unsupported stage/reason %q/%q", resultIndex, labelIndex, label.TerminalStage, label.ReasonCode)
			}
			if label.RankedPosition < 0 || label.PackedPosition < 0 {
				return fmt.Errorf("attribution result %d label %d: positions must not be negative", resultIndex, labelIndex)
			}
			for hitIndex, hit := range label.RetrieverHits {
				if !validRetriever(hit.Retriever) || hit.Position <= 0 || !validRelationState(hit.RelationState) {
					return fmt.Errorf("attribution result %d label %d hit %d: invalid sanitized hit", resultIndex, labelIndex, hitIndex)
				}
			}
		}
	}
	return nil
}

func validStageReason(stage, reason string) bool {
	want := map[string]string{
		StageLabelNotIndexed: "label_not_indexed", StageRetrievalMissing: "no_retriever_match",
		StageLinkingUnresolved: "relation_unresolved", StageRankingDropped: "removed_before_rank",
		StagePackingDropped: "omitted_by_packer", StagePacked: "selected_in_packet",
	}
	return want[stage] == reason
}

func validRetriever(value string) bool {
	switch value {
	case "qualified-symbol", "symbol-exact", "symbol-prefix", "identity-bridge", "fts", "path", "relation":
		return true
	default:
		return false
	}
}

func validRelationState(value string) bool {
	return value == "none" || value == "resolved" || value == "unresolved"
}

func safeAttributionText(value string) bool {
	for _, character := range value {
		if character < ' ' || character == 0x7f {
			return false
		}
	}
	return true
}
