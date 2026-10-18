package benchmark

import (
	"context"
	"fmt"
	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"testing"
)

func TestR10AttributionIdentityLookup(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(t.TempDir(), ".focalspan")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	updates := []store.FileUpdate{}
	for n := 0; n < 502; n++ {
		p := fmt.Sprintf("%04d.go", n)
		h := fmt.Sprintf("symbol%04d", n)
		sy := model.Symbol{Handle: h, FilePath: p, Language: "go", Kind: "function", Name: "Target", QualifiedName: "Target", StartLine: 1, EndLine: 1, EndByte: 1, Confidence: 1}
		c := model.Chunk{Handle: h + "chunk", FilePath: p, Language: "go", Kind: "function", SymbolHandle: h, SymbolName: "Target", StartLine: 1, EndLine: 1, EndByte: 1, Content: "x", ContentHash: h}
		chunks := []model.Chunk{c}
		if n == 0 {
			extra := c
			extra.Handle += "more"
			chunks = append(chunks, extra)
		}
		updates = append(updates, store.FileUpdate{File: model.SourceFile{Path: p, Language: "go", SHA256: h}, Extraction: model.Extraction{Symbols: []model.Symbol{sy}, Chunks: chunks}})
	}
	updates = append(updates, store.FileUpdate{File: model.SourceFile{Path: "empty.go", Language: "go", SHA256: "empty"}})
	if e = s.ApplyIndex(ctx, nil, updates, nil, model.IndexRun{}); e != nil {
		t.Fatal(e)
	}
	engine := appEngine{service: &app.Service{Store: s}}
	for _, tc := range []struct {
		name         string
		expectations []AttributionExpectation
		want         int
	}{
		{"late-symbol", []AttributionExpectation{{Expectation: "required_symbol", Path: "0501.go", Symbol: "Target", Kind: "function"}}, 1},
		{"late-path", []AttributionExpectation{{Expectation: "required_path", Path: "0501.go"}}, 1},
		{"empty-path", []AttributionExpectation{{Expectation: "required_path", Path: "empty.go"}}, 1},
		{"kind-mismatch", []AttributionExpectation{{Expectation: "required_symbol", Path: "0501.go", Symbol: "Target", Kind: "method"}}, 0},
		{"case-mismatch", []AttributionExpectation{{Expectation: "required_symbol", Path: "0501.go", Symbol: "target", Kind: "function"}}, 0},
		{"unindexed", []AttributionExpectation{{Expectation: "required_path", Path: "missing.go"}}, 0},
		{"duplicate-chunks", []AttributionExpectation{{Expectation: "required_symbol", Path: "0000.go", Symbol: "Target", Kind: "function"}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := engine.AttributionIdentities(ctx, tc.expectations)
			if e != nil {
				t.Fatal(e)
			}
			if len(got) != tc.want {
				t.Errorf("identities=%+v want count %d", got, tc.want)
			}
			labels, e := AttributeLabels(tc.expectations, AttributionInput{Indexed: got})
			if e != nil {
				t.Fatal(e)
			}
			want := StageLabelNotIndexed
			if tc.want > 0 {
				want = StageRetrievalMissing
			}
			if len(labels) != 1 || labels[0].TerminalStage != want {
				t.Errorf("labels=%+v want %s", labels, want)
			}
		})
	}
}
