package store

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestCPP20_ReviewMixedLanguage(t *testing.T) {
	for _, langs := range [][2]string{{"c", "cpp"}, {"cpp", "c"}, {"cpp", "cpp"}} {
		t.Run(langs[0]+"_to_"+langs[1], func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
			for i, name := range []string{"caller", "target"} {
				l := langs[i]
				path := name + "." + l
				x := model.Extraction{Symbols: []model.Symbol{{Handle: name, FilePath: path, Language: l, Kind: "function", Name: name, QualifiedName: name, StartLine: 1, EndLine: 1}}, Chunks: []model.Chunk{{Handle: name + "-chunk", FilePath: path, Language: l, Kind: "function", SymbolHandle: name, SymbolName: name, StartLine: 1, EndLine: 1, Content: "void " + name + "(){}", ContentHash: name}}}
				if i == 0 {
					x.Relations = []model.Relation{{FromHandle: name, UnresolvedTo: "target", Kind: "calls", Confidence: .25, Source: "cpp-call"}}
				}
				if e := s.ReplaceFile(ctx, model.SourceFile{Path: path, Language: l, SHA256: name}, x); e != nil {
					t.Fatal(e)
				}
			}
			for _, q := range []struct{ h, kind string }{{"caller", "callees"}, {"caller-chunk", "callees"}, {"target", "callers"}, {"target-chunk", "callers"}} {
				hits, e := s.RelatedCandidateHits(ctx, []string{q.h}, q.kind)
				if e != nil {
					t.Fatal(e)
				}
				if len(hits) != 1 {
					t.Errorf("%s %s got %d hits, expected 1", q.h, q.kind, len(hits))
				}
			}
		})
	}
}
