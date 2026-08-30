package linker

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/projectmeta"
	"github.com/focalspan/focalspan/internal/store"
)

func TestLinkerResolvesUniqueRelationAndPreservesAmbiguity(t *testing.T) {
	s := openLinkerStore(t)
	seedLinkerFile(t, s, "src/auth.go", "source", "ValidateToken", "func ValidateToken", nil)
	seedLinkerFile(t, s, "src/http.go", "caller", "Authenticate", "func Authenticate", []model.Relation{{FromHandle: "caller", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .3, Source: "test"}})
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].ToHandle != "source" || relations[0].UnresolvedTo != "" {
		t.Fatalf("unique relation=%+v", relations)
	}

	seedLinkerFile(t, s, "src/other.go", "other", "ValidateToken", "func ValidateToken", nil)
	seedLinkerFile(t, s, "src/ambiguous.go", "ambiguous", "CallOther", "func CallOther", []model.Relation{{FromHandle: "ambiguous", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .3, Source: "test"}})
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	relations, err = s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		if relation.FromHandle == "ambiguous" && (relation.ToHandle != "" || relation.UnresolvedTo != "ValidateToken") {
			t.Fatalf("ambiguous relation was resolved: %+v", relation)
		}
	}
}

func TestLinkerUsesManifestFactsForScopedPackageResolution(t *testing.T) {
	tests := []struct {
		name       string
		manifest   string
		fact       projectmeta.Fact
		targetPath string
		target     string
	}{
		{
			name:       "go module",
			manifest:   "go.mod",
			fact:       projectmeta.Fact{SourcePath: "go.mod", Kind: "module", Target: "example.com/app"},
			targetPath: "auth/token_service.go",
			target:     "example.com/app/auth",
		},
		{
			name:       "composer psr4",
			manifest:   "composer.json",
			fact:       projectmeta.Fact{SourcePath: "composer.json", Kind: "psr-4", Name: `Demo\`, Target: "src/"},
			targetPath: "src/Auth/TokenService.php",
			target:     `Demo\Auth\TokenService`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := openLinkerStore(t)
			seedLinkerFile(t, s, test.targetPath, "target", "TokenService", "target", nil)
			seedLinkerFile(t, s, "cmd/main.go", "caller", "Main", "caller", []model.Relation{{FromHandle: "caller", UnresolvedTo: test.target, Kind: "imports", Confidence: .4, Source: "test"}})
			if err := (&Linker{Store: s}).Link(context.Background(), []projectmeta.Fact{test.fact}); err != nil {
				t.Fatal(err)
			}
			relations, err := s.Relations(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(relations) != 1 || relations[0].ToHandle != "target" || relations[0].UnresolvedTo != "" {
				t.Fatalf("relations=%+v", relations)
			}
		})
	}
}

func openLinkerStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedLinkerFile(t *testing.T, s *store.Store, path, handle, name, signature string, relations []model.Relation) {
	t.Helper()
	content := signature + " {}"
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: "go", SHA256: handle}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: handle, FilePath: path, Language: "go", Kind: "function", Name: name, QualifiedName: name, Signature: signature, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: len(content), Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: handle + "-chunk", FilePath: path, Language: "go", Kind: "function", SymbolHandle: handle, SymbolName: name, Signature: signature, StartLine: 1, EndLine: 1, StartByte: 0, EndByte: len(content), Content: content, ContentHash: handle}},
		Relations: relations,
	}); err != nil {
		t.Fatal(err)
	}
}
