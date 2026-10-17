package evidence

import (
	"context"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
	"testing"
)

func TestActualModuleOutlineNeverClaimsSourceFidelity(t *testing.T) {
	extracted, err := jsts.NewExtractor().Extract(context.Background(), model.SourceFile{Path: "module.js", Language: "javascript", Content: []byte("export function Run() { return 1; }\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range extracted.Chunks {
		if chunk.Kind == "module-outline" {
			got, err := NewCompiler(nil).Compile(CompileRequest{Plan: query.PlanQuery("module"), Mode: ModeSource, TokenBudget: 1200, Candidates: []model.RankedCandidate{{Handle: chunk.Handle, Path: chunk.FilePath, Kind: chunk.Kind, Signature: chunk.Signature, StartLine: chunk.StartLine, EndLine: chunk.EndLine, Content: chunk.Content, ContentHash: chunk.ContentHash}}})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Packet.Evidence) != 1 || got.Packet.Evidence[0].Fidelity != FidelitySynthetic {
				t.Fatalf("actual generated outline: %+v", got.Packet)
			}
			return
		}
	}
	t.Fatal("module outline fixture missing")
}

func TestSyntheticNeverClaimsSourceFidelity(t *testing.T) {
	for _, mode := range []Mode{ModeSource, ModeFocused, ModeOutline} {
		for _, limit := range []int{256, 512, 1200, 4000} {
			result, err := NewCompiler(nil).Compile(CompileRequest{Plan: query.PlanQuery("module"), TokenBudget: limit, Mode: mode, Candidates: []model.RankedCandidate{{Handle: "outline", Path: "module.js", Kind: "module-outline", Symbol: "module.js", Signature: "synthetic outline (not source slice)", StartLine: 1, EndLine: 1, Content: "functions: Run", ContentHash: "synthetic"}}})
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range result.Packet.Evidence {
				if item.Fidelity == FidelityVerbatim || item.Fidelity == FidelityExcerpt {
					t.Fatalf("mode=%s budget=%d synthetic labeled source: %+v", mode, limit, item)
				}
			}
			if err := Validate(result.Packet); err != nil {
				t.Fatal(err)
			}
			if MeasureModelVisible(result.Packet, nil) > limit {
				t.Fatal("wire budget exceeded")
			}
			if _, err := EncodeContextV2(result.Packet, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}
