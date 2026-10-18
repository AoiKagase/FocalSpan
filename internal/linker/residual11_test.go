package linker

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
	"testing"
)

func TestR05ModuleOwnerResolution(t *testing.T) {
	for _, tc := range []struct {
		name, target, source string
		owners               int
		want                 string
	}{
		{"module", "./dep.ts", "jsts:import", 1, "module"}, {"named", "work", "jsts:import-symbol", 1, "work"},
		{"ambiguous", "dep", "jsts:import", 2, ""}, {"no-owner", "./dep.ts", "jsts:import", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, e := store.Open(t.TempDir(), ".focalspan")
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			seed := func(path string, syms []model.Symbol, rels []model.Relation) {
				t.Helper()
				for i := range syms {
					syms[i].FilePath = path
					syms[i].Language = "typescript"
					syms[i].StartLine = 1
					syms[i].EndLine = 1
				}
				if e := s.ReplaceFile(ctx, model.SourceFile{Path: path, Language: "typescript", SHA256: path}, model.Extraction{Symbols: syms, Relations: rels}); e != nil {
					t.Fatal(e)
				}
			}
			syms := []model.Symbol{{Handle: "work", Name: "work", QualifiedName: "work", Kind: "function"}}
			if tc.owners > 0 {
				syms = append(syms, model.Symbol{Handle: "module", Name: "dep.ts", Kind: "module"})
			}
			seed("dep.ts", syms, nil)
			if tc.owners == 2 {
				seed("other/dep.ts", []model.Symbol{{Handle: "other", Name: "other/dep.ts", Kind: "module"}}, nil)
			}
			seed("main.ts", []model.Symbol{{Handle: "main", Name: "main.ts", Kind: "module"}}, []model.Relation{{FromHandle: "main", UnresolvedTo: tc.target, Kind: "imports", Source: tc.source, Confidence: .75}})
			if e := (&Linker{Store: s}).Link(ctx, nil); e != nil {
				t.Fatal(e)
			}
			rels, e := s.Relations(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if len(rels) != 1 || rels[0].ToHandle != tc.want {
				t.Fatalf("relations=%+v want %q", rels, tc.want)
			}
		})
	}
}
