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

func TestLinkerRecognizesRustAndPythonModulePaths(t *testing.T) {
	tests := []struct {
		name, importer, candidate, target string
	}{
		{name: "rust crate use", importer: "src/http/middleware.rs", candidate: "src/auth/token_service.rs", target: "crate::auth::token_service::TokenService"},
		{name: "python absolute import", importer: "tests/test_token_service.py", candidate: "src/auth/token_service.py", target: "src.auth.token_service"},
		{name: "python relative import", importer: "src/http/middleware.py", candidate: "src/auth/token_service.py", target: "..auth.token_service"},
		{name: "python imported symbol", importer: "src/http/middleware.py", candidate: "src/auth/token_service.py", target: "..auth.token_service.TokenService"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !linkerPathMatch(test.importer, test.candidate, test.target) {
				t.Fatalf("linkerPathMatch(%q, %q, %q)=false", test.importer, test.candidate, test.target)
			}
		})
	}
}

func TestLinkerResolvesQualifiedRustPathToNamedSymbol(t *testing.T) {
	s := openLinkerStore(t)
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/auth/token_service.rs", Language: "rust", SHA256: "target"}, model.Extraction{
		Symbols: []model.Symbol{
			{Handle: "target-module", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "crate_module", Name: "src/auth/token_service.rs", StartLine: 1, EndLine: 1, Confidence: 1},
			{Handle: "target-service", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "struct", Name: "TokenService", QualifiedName: "crate::TokenService", StartLine: 2, EndLine: 2, Confidence: 1},
		},
		Chunks: []model.Chunk{
			{Handle: "target-module-chunk", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "module-outline", SymbolHandle: "target-module", SymbolName: "src/auth/token_service.rs", StartLine: 1, EndLine: 1, Content: "crate module", ContentHash: "target-module"},
			{Handle: "target-service-chunk", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "struct-outline", SymbolHandle: "target-service", SymbolName: "TokenService", StartLine: 2, EndLine: 2, Content: "struct TokenService", ContentHash: "target-service"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/http/middleware.rs", Language: "rust", SHA256: "importer"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "importer", FilePath: "src/http/middleware.rs", Language: "rust", Kind: "crate_module", Name: "src/http/middleware.rs", StartLine: 1, EndLine: 1, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "importer-chunk", FilePath: "src/http/middleware.rs", Language: "rust", Kind: "module-outline", SymbolHandle: "importer", SymbolName: "src/http/middleware.rs", StartLine: 1, EndLine: 1, Content: "crate module", ContentHash: "importer"}},
		Relations: []model.Relation{{FromHandle: "importer", UnresolvedTo: "crate::auth::token_service::TokenService", Kind: "imports", Confidence: .9, Source: "rust-use"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		if relation.FromHandle == "importer" && relation.Kind == "imports" {
			if relation.ToHandle != "target-service" || relation.UnresolvedTo != "" {
				t.Fatalf("relation=%+v, want target-service", relation)
			}
			return
		}
	}
	t.Fatal("qualified Rust import relation missing")
}

func TestLinkerResolvesQualifiedPythonPathToNamedSymbol(t *testing.T) {
	s := openLinkerStore(t)
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/auth/token_service.py", Language: "python", SHA256: "target"}, model.Extraction{
		Symbols: []model.Symbol{
			{Handle: "target-module", FilePath: "src/auth/token_service.py", Language: "python", Kind: "module", Name: "token_service.py", StartLine: 1, EndLine: 1, Confidence: 1},
			{Handle: "target-service", FilePath: "src/auth/token_service.py", Language: "python", Kind: "class", Name: "TokenService", QualifiedName: "TokenService", StartLine: 2, EndLine: 2, Confidence: 1},
		},
		Chunks: []model.Chunk{
			{Handle: "target-module-chunk", FilePath: "src/auth/token_service.py", Language: "python", Kind: "module-outline", SymbolHandle: "target-module", SymbolName: "token_service.py", StartLine: 1, EndLine: 1, Content: "module token_service.py", ContentHash: "target-module"},
			{Handle: "target-service-chunk", FilePath: "src/auth/token_service.py", Language: "python", Kind: "class", SymbolHandle: "target-service", SymbolName: "TokenService", StartLine: 2, EndLine: 2, Content: "class TokenService", ContentHash: "target-service"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/http/middleware.py", Language: "python", SHA256: "importer"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "importer", FilePath: "src/http/middleware.py", Language: "python", Kind: "module", Name: "middleware.py", StartLine: 1, EndLine: 1, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "importer-chunk", FilePath: "src/http/middleware.py", Language: "python", Kind: "module-outline", SymbolHandle: "importer", SymbolName: "middleware.py", StartLine: 1, EndLine: 1, Content: "module middleware.py", ContentHash: "importer"}},
		Relations: []model.Relation{{FromHandle: "importer", UnresolvedTo: "..auth.token_service.TokenService", Kind: "imports", Confidence: .9, Source: "python-import"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := (&Linker{Store: s}).Link(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		if relation.FromHandle == "importer" && relation.Kind == "imports" {
			if relation.ToHandle != "target-service" || relation.UnresolvedTo != "" {
				t.Fatalf("relation=%+v, want target-service", relation)
			}
			return
		}
	}
	t.Fatal("qualified Python import relation missing")
}

func TestLinkerScopedLinkingLeavesUnrelatedRelationsUntouched(t *testing.T) {
	s := openLinkerStore(t)
	defer s.Close()
	seedLinkerFile(t, s, "src/target.go", "target", "ValidateToken", "func ValidateToken", nil)
	seedLinkerFile(t, s, "src/changed.go", "changed", "Changed", "func Changed", []model.Relation{{FromHandle: "changed", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .9, Source: "test"}})
	seedLinkerFile(t, s, "src/unrelated.go", "unrelated", "Unrelated", "func Unrelated", []model.Relation{{FromHandle: "unrelated", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .9, Source: "test"}})
	if err := (&Linker{Store: s}).LinkWithScope(context.Background(), nil, store.LinkScope{ChangedPaths: []string{"src/changed.go"}}, nil); err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		switch relation.FromHandle {
		case "changed":
			if relation.ToHandle != "target" || relation.UnresolvedTo != "" {
				t.Fatalf("changed relation=%+v, want linked", relation)
			}
		case "unrelated":
			if relation.ToHandle != "" || relation.UnresolvedTo != "ValidateToken" {
				t.Fatalf("unrelated relation=%+v, want unresolved", relation)
			}
		}
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
