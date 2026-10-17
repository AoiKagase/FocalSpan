package linker

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/projectmeta"
	"github.com/focalspan/focalspan/internal/store"
	"testing"
)

func TestF15ExactAndWildcardAliases(t *testing.T) {
	fact := projectmeta.Fact{SourcePath: "tsconfig.json", Kind: "path-alias", Name: "@cfg", Target: "src/config.ts"}
	if got := factCandidatePaths(fact, "@cfg"); len(got) != 1 || got[0] != "src/config.ts" {
		t.Fatalf("positive alias=%v", got)
	}
	if got := factCandidatePaths(fact, "@cfgExtra"); len(got) != 0 {
		t.Fatalf("nonexistent @cfgExtra maps through exact alias: %v", got)
	}
}

func TestF16UnrelatedManifestDoesNotResolve(t *testing.T) {
	fact := projectmeta.Fact{SourcePath: "package.json", Kind: "main", Target: "src/main.js"}
	if got := factCandidatePaths(fact, "missing-package"); len(got) != 0 {
		t.Fatalf("unrelated missing-package import maps to main: %v", got)
	}
}

func fsAuditCoreSeed(t *testing.T, s *store.Store, path, handle, name string, relations []model.Relation) {
	t.Helper()
	symbol := model.Symbol{Handle: handle, FilePath: path, Language: "typescript", Kind: "module", Name: name, QualifiedName: name, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 1, Confidence: 1}
	chunk := model.Chunk{Handle: handle + "-chunk", FilePath: path, Language: "typescript", Kind: "module-outline", SymbolHandle: handle, SymbolName: name, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 1, Content: "x", ContentHash: handle}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: "typescript", SHA256: handle, SizeBytes: 1}, model.Extraction{Symbols: []model.Symbol{symbol}, Chunks: []model.Chunk{chunk}, Relations: relations}); err != nil {
		t.Fatal(err)
	}
}

func TestF17ProjectedAndScopedLinkAgree(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fsAuditCoreSeed(t, s, "src/dep.ts", "dep", "dep", nil)
	fsAuditCoreSeed(t, s, "src/main.ts", "main", "main", []model.Relation{{FromHandle: "main", UnresolvedTo: "./dep.ts", Kind: "imports", Confidence: 1, Source: "audit"}})
	l := &Linker{Store: s}
	if err := l.Link(ctx, nil); err != nil {
		t.Fatal(err)
	}
	before, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("relations=%+v", before)
	}
	fullTarget := before[0].ToHandle
	if err := l.LinkWithScope(ctx, nil, store.LinkScope{ChangedPaths: []string{"src/main.ts"}}, nil); err != nil {
		t.Fatal(err)
	}
	after, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].ToHandle != "dep" {
		t.Fatalf("scoped positive control not linked: %+v", after)
	}
	if fullTarget != after[0].ToHandle {
		t.Fatalf("full link target=%q, scoped target=%q; projection removed relative import", fullTarget, after[0].ToHandle)
	}
}

func TestF18ExtensionlessRelativeImport(t *testing.T) {
	if !linkerPathMatch("src/main.ts", "src/dep.ts", "./dep.ts") {
		t.Fatal("extensionful positive control failed")
	}
	if !linkerPathMatch("src/main.ts", "src/dep.ts", "./dep") {
		t.Fatal("extensionless relative import ./dep does not match src/dep.ts")
	}
}
