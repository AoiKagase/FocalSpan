package benchmark

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAttributeLabelsUsesEarliestTerminalStage(t *testing.T) {
	expectations := []AttributionExpectation{
		{Expectation: "required_symbol", Path: "missing.go", Symbol: "NotIndexed"},
		{Expectation: "required_symbol", Path: "retrieval.go", Symbol: "NeverRetrieved"},
		{Expectation: "required_symbol", Path: "link.go", Symbol: "LinkedOnlyLexically"},
		{Expectation: "required_symbol", Path: "rank.go", Symbol: "DroppedBeforeRank"},
		{Expectation: "required_symbol", Path: "pack.go", Symbol: "DroppedByPacker"},
		{Expectation: "required_symbol", Path: "packet.go", Symbol: "Packed"},
	}
	identities := []AttributionIdentity{
		{Path: "retrieval.go", Symbol: "NeverRetrieved"},
		{Path: "link.go", Symbol: "LinkedOnlyLexically"},
		{Path: "rank.go", Symbol: "DroppedBeforeRank"},
		{Path: "pack.go", Symbol: "DroppedByPacker"},
		{Path: "packet.go", Symbol: "Packed"},
	}
	input := AttributionInput{
		Indexed: identities,
		Retrieved: []AttributionObservation{
			{AttributionIdentity: identities[1], Retriever: "relation", Position: 4, Relation: "callers", RelationResolved: false},
			{AttributionIdentity: identities[2], Retriever: "symbol-exact", Position: 2},
			{AttributionIdentity: identities[3], Retriever: "fts", Position: 9},
			{AttributionIdentity: identities[4], Retriever: "qualified-symbol", Position: 1},
		},
		Ranked: []AttributionIdentity{identities[3], identities[4]},
		Packed: []AttributionIdentity{identities[4]},
	}

	got, err := AttributeLabels(expectations, input)
	if err != nil {
		t.Fatal(err)
	}
	wantStages := []string{StageLabelNotIndexed, StageRetrievalMissing, StageLinkingUnresolved, StageRankingDropped, StagePackingDropped, StagePacked}
	wantReasons := []string{"label_not_indexed", "no_retriever_match", "relation_unresolved", "removed_before_rank", "omitted_by_packer", "selected_in_packet"}
	for index := range wantStages {
		if got[index].TerminalStage != wantStages[index] || got[index].ReasonCode != wantReasons[index] {
			t.Fatalf("label %d = %+v, want stage=%s reason=%s", index, got[index], wantStages[index], wantReasons[index])
		}
	}
	if got[2].RetrieverHits[0].Position != 4 || got[4].RankedPosition != 1 || got[5].PackedPosition != 1 {
		t.Fatalf("positions were not preserved: %+v", got)
	}
}

func TestAttributeLabelsMatchesPathSymbolAndOptionalKindExactly(t *testing.T) {
	expectations := []AttributionExpectation{
		{Expectation: "required_path", Path: "same.go"},
		{Expectation: "required_symbol", Path: "same.go", Symbol: "Target", Kind: "function"},
	}
	input := AttributionInput{
		Indexed: []AttributionIdentity{{Path: "same.go", Symbol: "Other", Kind: "function"}, {Path: "same.go", Symbol: "Target", Kind: "method"}},
		Packed:  []AttributionIdentity{{Path: "same.go", Symbol: "Other", Kind: "function"}, {Path: "same.go", Symbol: "Target", Kind: "method"}},
	}
	got, err := AttributeLabels(expectations, input)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].TerminalStage != StagePacked || got[1].TerminalStage != StageLabelNotIndexed {
		t.Fatalf("exact matching = %+v", got)
	}
}

func TestAttributeLabelsMatchesExpansionAnchorAndConstrainsLinkingStage(t *testing.T) {
	expectations := []AttributionExpectation{
		{Expectation: "expansion_anchor", Path: "anchor.go", Symbol: "Target", Kind: "function", Relation: "callers"},
		{Expectation: "required_symbol", Path: "mixed.go", Symbol: "Target"},
		{Expectation: "required_symbol", Path: "resolved.go", Symbol: "Target"},
	}
	identities := []AttributionIdentity{
		{Path: "anchor.go", Symbol: "Target", Kind: "function"},
		{Path: "mixed.go", Symbol: "Target"},
		{Path: "resolved.go", Symbol: "Target"},
	}
	input := AttributionInput{
		Indexed: identities,
		Retrieved: []AttributionObservation{
			{AttributionIdentity: identities[0], Retriever: "relation", Position: 3, Relation: "callers"},
			{AttributionIdentity: identities[1], Retriever: "relation", Position: 4, Relation: "callers"},
			{AttributionIdentity: identities[1], Retriever: "symbol-exact", Position: 1},
			{AttributionIdentity: identities[2], Retriever: "relation", Position: 2, Relation: "callers", RelationResolved: true},
		},
	}

	got, err := AttributeLabels(expectations, input)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].TerminalStage != StageLinkingUnresolved {
		t.Fatalf("expansion anchor = %+v", got[0])
	}
	if got[1].TerminalStage != StageRankingDropped || got[2].TerminalStage != StageRankingDropped {
		t.Fatalf("ordinary/resolved retrieval must not be linking failures: %+v", got)
	}
}

func TestMarshalAttributionRejectsUnsafeIdentityAndReason(t *testing.T) {
	tests := []AttributionResult{
		{Schema: AttributionSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: []AttributionLabel{{Expectation: "required_path", Path: `C:\Users\private\source.go`, TerminalStage: StageRetrievalMissing, ReasonCode: "no_retriever_match"}}},
		{Schema: AttributionSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: []AttributionLabel{{Expectation: "required_path", Path: "safe.go", TerminalStage: StageRetrievalMissing, ReasonCode: "TOP SECRET SOURCE BODY"}}},
	}
	for _, result := range tests {
		if _, err := MarshalAttribution([]AttributionResult{result}); err == nil {
			t.Fatalf("unsafe attribution accepted: %+v", result)
		}
	}
}

func TestAttributionOmitsUnmatchedCandidateSentinel(t *testing.T) {
	expectation := AttributionExpectation{Expectation: "required_symbol", Path: "safe.go", Symbol: "Target"}
	labels, err := AttributeLabels([]AttributionExpectation{expectation}, AttributionInput{
		Indexed: []AttributionIdentity{
			{Path: "safe.go", Symbol: "Target"},
			{Path: "private.go", Symbol: "TOP SECRET SOURCE BODY"},
		},
		Retrieved: []AttributionObservation{{
			AttributionIdentity: AttributionIdentity{Path: "private.go", Symbol: "TOP SECRET SOURCE BODY"},
			Retriever:           "fts",
			Position:            1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := MarshalAttribution([]AttributionResult{{Schema: AttributionSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: labels}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("TOP SECRET SOURCE BODY")) || bytes.Contains(encoded, []byte("private.go")) {
		t.Fatalf("unmatched candidate data leaked: %s", encoded)
	}
}

func TestAttributionJSONAndMarkdownAreDeterministicAndSourceFree(t *testing.T) {
	result := AttributionResult{Schema: AttributionSchemaV1, CaseID: "case-a", RepositoryID: "self", Profile: "full-evidence-focused", Budget: 1024, Labels: []AttributionLabel{{Expectation: "required_symbol", Path: "internal/app/evidence.go", Symbol: "QueryEvidence", Kind: "function", TerminalStage: StagePackingDropped, ReasonCode: "omitted_by_packer", RetrieverHits: []AttributionHit{{Retriever: "symbol-exact", Position: 1, RelationState: "none"}}, RankedPosition: 2}}}
	first, err := MarshalAttribution([]AttributionResult{result})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := MarshalAttribution([]AttributionResult{result})
	markdown, err := RenderAttributionMarkdown([]AttributionResult{result})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || bytes.Contains(first, []byte("source")) || strings.Contains(markdown, "source") {
		t.Fatalf("unsafe or nondeterministic output\njson=%s\nmarkdown=%s", first, markdown)
	}
	for _, want := range []string{"focalspan.benchmark-attribution.v1", "packing_dropped", "omitted_by_packer", "internal/app/evidence.go"} {
		if !bytes.Contains(first, []byte(want)) || !strings.Contains(markdown, want) {
			t.Fatalf("output missing %q\njson=%s\nmarkdown=%s", want, first, markdown)
		}
	}
	if bytes.Contains(first, []byte{'\r'}) || strings.Contains(markdown, "\r") {
		t.Fatalf("output must use LF line endings")
	}
	var roundTrip []AttributionResult
	if err := json.Unmarshal(first, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, []AttributionResult{result}) {
		t.Fatalf("round trip = %+v, want %+v", roundTrip, result)
	}
}

func TestMarshalAttributionRejectsControlCharactersAndInvalidPositions(t *testing.T) {
	makeResult := func() AttributionResult {
		return AttributionResult{Schema: AttributionSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: []AttributionLabel{{Expectation: "required_path", Path: "safe.go", TerminalStage: StageRankingDropped, ReasonCode: "removed_before_rank", RetrieverHits: []AttributionHit{{Retriever: "fts", Position: 1, RelationState: "none"}}}}}
	}
	tests := []AttributionResult{makeResult(), makeResult(), makeResult()}
	tests[0].CaseID = "case\nsecret"
	tests[1].Labels[0].Symbol = "Symbol\rBody"
	tests[2].Labels[0].RetrieverHits[0].Position = 0
	for _, result := range tests {
		if _, err := MarshalAttribution([]AttributionResult{result}); err == nil {
			t.Fatalf("unsafe attribution accepted: %+v", result)
		}
	}
}
