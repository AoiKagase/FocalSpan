package benchmark

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCompileDiagnosisLabelsUsesFixedPriority(t *testing.T) {
	tests := []struct {
		name       string
		input      AttributionInput
		label      AttributionLabel
		wantStage  string
		wantReason string
	}{
		{name: "not indexed", label: diagnosisAttributionLabel(StageLabelNotIndexed, "label_not_indexed"), wantStage: DiagnosisLabelNotIndexed, wantReason: "label_not_indexed"},
		{name: "packed beats every earlier observation", input: diagnosisInput(true, true), label: diagnosisAttributionLabel(StagePacked, "selected_in_packet"), wantStage: DiagnosisPacked, wantReason: "selected_in_packet"},
		{name: "ranked beats retrieval observations", input: diagnosisInput(true, false), label: diagnosisAttributionLabel(StagePackingDropped, "omitted_by_packer"), wantStage: DiagnosisPackingDropped, wantReason: "omitted_by_packer"},
		{name: "unresolved relation beats exact hits", input: diagnosisRetrievedInput("target.go", "Target"), label: diagnosisAttributionLabel(StageLinkingUnresolved, "relation_unresolved"), wantStage: DiagnosisLinkingUnresolved, wantReason: "relation_unresolved"},
		{name: "exact retriever hit beats path-only hit", input: diagnosisRetrievedInput("target.go", "Target"), label: diagnosisAttributionLabel(StageRankingDropped, "removed_before_rank"), wantStage: DiagnosisRankingDropped, wantReason: "removed_before_rank"},
		{name: "same path without exact identity", input: diagnosisRetrievedInput("target.go", "Other"), label: diagnosisAttributionLabel(StageRetrievalMissing, "no_retriever_match"), wantStage: DiagnosisSymbolMatchMissing, wantReason: "expected_path_retrieved_identity_missing"},
		{name: "path absent", input: diagnosisRetrievedInput("other.go", "Other"), label: diagnosisAttributionLabel(StageRetrievalMissing, "no_retriever_match"), wantStage: DiagnosisPathScopeMissing, wantReason: "expected_path_not_retrieved"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CompileDiagnosisLabels(test.input, []AttributionLabel{test.label})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].DiagnosticStage != test.wantStage || got[0].ReasonCode != test.wantReason {
				t.Fatalf("diagnosis = %+v, want %s/%s", got, test.wantStage, test.wantReason)
			}
		})
	}
}

func TestCompileDiagnosisLabelsRequiredPathCannotBeSymbolMatchMissing(t *testing.T) {
	label := diagnosisAttributionLabel(StageRetrievalMissing, "no_retriever_match")
	label.Expectation = "required_path"
	label.Symbol = ""
	got, err := CompileDiagnosisLabels(diagnosisRetrievedInput("target.go", "Other"), []AttributionLabel{label})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DiagnosticStage == DiagnosisSymbolMatchMissing {
		t.Fatalf("required path was classified as symbol-match missing: %+v", got[0])
	}
}

func TestCompileDiagnosisLabelsGroupsSamePathHitsInFirstObservedOrder(t *testing.T) {
	label := diagnosisAttributionLabel(StageRetrievalMissing, "no_retriever_match")
	input := AttributionInput{Retrieved: []AttributionObservation{
		{AttributionIdentity: AttributionIdentity{Path: "target.go", Symbol: "PRIVATE ONE"}, Retriever: "fts", Position: 7},
		{AttributionIdentity: AttributionIdentity{Path: "other.go", Symbol: "Target"}, Retriever: "symbol-exact", Position: 1},
		{AttributionIdentity: AttributionIdentity{Path: "target.go", Symbol: "PRIVATE TWO"}, Retriever: "path", Position: 3},
		{AttributionIdentity: AttributionIdentity{Path: "target.go", Symbol: "PRIVATE THREE"}, Retriever: "fts", Position: 9},
	}}

	got, err := CompileDiagnosisLabels(input, []AttributionLabel{label})
	if err != nil {
		t.Fatal(err)
	}
	want := []DiagnosisPathHit{{Retriever: "fts", FirstPosition: 7, Count: 2}, {Retriever: "path", FirstPosition: 3, Count: 1}}
	if !reflect.DeepEqual(got[0].PathHits, want) {
		t.Fatalf("path hits = %+v, want %+v", got[0].PathHits, want)
	}
	result := DiagnosisResult{Schema: DiagnosisSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: got}
	encoded, err := MarshalDiagnosis([]DiagnosisResult{result})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range [][]byte{[]byte("PRIVATE ONE"), []byte("PRIVATE TWO"), []byte("PRIVATE THREE"), []byte("other.go")} {
		if bytes.Contains(encoded, secret) {
			t.Fatalf("unmatched identity leaked in diagnosis: %s", encoded)
		}
	}
}

func TestDiagnosisJSONAndMarkdownAreDeterministicAndValidated(t *testing.T) {
	result := DiagnosisResult{Schema: DiagnosisSchemaV1, CaseID: "case-a", RepositoryID: "self", Profile: "full-evidence-focused", Budget: 2048, Labels: []DiagnosisLabel{{
		Expectation: "required_symbol", Path: "internal/app/evidence.go", Symbol: "QueryEvidence", Kind: "function",
		AttributionStage: StageRetrievalMissing, DiagnosticStage: DiagnosisSymbolMatchMissing, ReasonCode: "expected_path_retrieved_identity_missing",
		PathHits: []DiagnosisPathHit{{Retriever: "fts", FirstPosition: 2, Count: 3}},
	}}}
	first, err := MarshalDiagnosis([]DiagnosisResult{result})
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalDiagnosis([]DiagnosisResult{result})
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := RenderDiagnosisMarkdown([]DiagnosisResult{result})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || bytes.Contains(first, []byte{13}) || strings.Contains(markdown, "\r") {
		t.Fatalf("diagnosis output is nondeterministic or not LF-only\njson=%s\nmarkdown=%s", first, markdown)
	}
	for _, want := range []string{DiagnosisSchemaV1, DiagnosisSymbolMatchMissing, "expected_path_retrieved_identity_missing", "internal/app/evidence.go", "fts:2:3"} {
		if !bytes.Contains(first, []byte(want)) && !strings.Contains(markdown, want) {
			t.Fatalf("diagnosis output missing %q\njson=%s\nmarkdown=%s", want, first, markdown)
		}
	}
	var roundTrip []DiagnosisResult
	if err := json.Unmarshal(first, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, []DiagnosisResult{result}) {
		t.Fatalf("round trip = %+v, want %+v", roundTrip, result)
	}
}

func TestMarshalDiagnosisRejectsUnsafeOrInconsistentResults(t *testing.T) {
	valid := func() DiagnosisResult {
		return DiagnosisResult{Schema: DiagnosisSchemaV1, CaseID: "case", RepositoryID: "self", Profile: "p", Budget: 1024, Labels: []DiagnosisLabel{{
			Expectation: "required_symbol", Path: "safe.go", Symbol: "Target", AttributionStage: StageRetrievalMissing,
			DiagnosticStage: DiagnosisPathScopeMissing, ReasonCode: "expected_path_not_retrieved",
		}}}
	}
	tests := []DiagnosisResult{valid(), valid(), valid(), valid(), valid(), valid(), valid(), valid()}
	tests[0].Schema = AttributionSchemaV1
	tests[1].Labels[0].Path = `C:\Users\private\source.go`
	tests[2].CaseID = "case\nsecret"
	tests[3].Labels[0].ReasonCode = "wrong"
	tests[4].Labels[0].RankedPosition = -1
	tests[5].Labels[0].PathHits = []DiagnosisPathHit{{Retriever: "unknown", FirstPosition: 1, Count: 1}}
	tests[6].Labels[0].PathHits = []DiagnosisPathHit{{Retriever: "fts", FirstPosition: 0, Count: 1}}
	tests[7].Labels[0].RetrieverHits = []AttributionHit{{Retriever: "fts", Position: 1, RelationState: "secret"}}
	for _, result := range tests {
		if _, err := MarshalDiagnosis([]DiagnosisResult{result}); err == nil {
			t.Fatalf("unsafe diagnosis accepted: %+v", result)
		}
	}
}

func diagnosisAttributionLabel(stage, reason string) AttributionLabel {
	label := AttributionLabel{Expectation: "required_symbol", Path: "target.go", Symbol: "Target", TerminalStage: stage, ReasonCode: reason}
	if stage == StageRankingDropped {
		label.RetrieverHits = []AttributionHit{{Retriever: "symbol-exact", Position: 1, RelationState: "none"}}
	}
	if stage == StageLinkingUnresolved {
		label.RetrieverHits = []AttributionHit{{Retriever: "relation", Position: 1, RelationState: "unresolved"}}
	}
	if stage == StagePackingDropped {
		label.RankedPosition = 1
	}
	if stage == StagePacked {
		label.RankedPosition = 1
		label.PackedPosition = 1
	}
	return label
}

func diagnosisInput(ranked, packed bool) AttributionInput {
	input := diagnosisRetrievedInput("target.go", "Target")
	if ranked {
		input.Ranked = []AttributionIdentity{{Path: "target.go", Symbol: "Target"}}
	}
	if packed {
		input.Packed = []AttributionIdentity{{Path: "target.go", Symbol: "Target"}}
	}
	return input
}

func diagnosisRetrievedInput(path, symbol string) AttributionInput {
	return AttributionInput{Retrieved: []AttributionObservation{{
		AttributionIdentity: AttributionIdentity{Path: path, Symbol: symbol}, Retriever: "fts", Position: 1,
	}}}
}
