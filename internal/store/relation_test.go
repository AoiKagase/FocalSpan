package store

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestRelatedCandidatesRespectRelationAndHandles(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	extraction := model.Extraction{
		Symbols:   []model.Symbol{{Handle: "source", FilePath: "a.go", Language: "go", Kind: "function", Name: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}, {Handle: "test", FilePath: "a_test.go", Language: "go", Kind: "test", Name: "TestValidateToken", Signature: "func TestValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "chunk-source", FilePath: "a.go", Language: "go", Kind: "function", SymbolHandle: "source", SymbolName: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Content: "return nil", ContentHash: "source"}, {Handle: "chunk-test", FilePath: "a_test.go", Language: "go", Kind: "test", SymbolHandle: "test", SymbolName: "TestValidateToken", Signature: "func TestValidateToken", StartLine: 1, EndLine: 2, Content: "ValidateToken", ContentHash: "test"}},
		Relations: []model.Relation{{FromHandle: "test", ToHandle: "source", Kind: "tests", Confidence: .8, Source: "test"}},
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "a.go", Language: "go", SHA256: "a"}, model.Extraction{Symbols: extraction.Symbols[:1], Chunks: extraction.Chunks[:1]}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "a_test.go", Language: "go", SHA256: "b"}, model.Extraction{Symbols: extraction.Symbols[1:], Chunks: extraction.Chunks[1:], Relations: extraction.Relations}); err != nil {
		t.Fatal(err)
	}
	got, err := s.RelatedCandidates(context.Background(), []string{"test"}, "tests")
	if err != nil || len(got) != 1 || got[0].Symbol != "ValidateToken" {
		t.Fatalf("related=%+v err=%v", got, err)
	}
	missing, err := s.RelatedCandidates(context.Background(), []string{"test"}, "does-not-exist")
	if err != nil || len(missing) != 0 {
		t.Fatalf("unsupported relation=%+v err=%v", missing, err)
	}
}

func TestRelatedCandidatesResolveUnresolvedCallByTargetSymbolName(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "auth.go", Language: "go", SHA256: "auth"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "target", FilePath: "auth.go", Language: "go", Kind: "method", Name: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: "target-chunk", FilePath: "auth.go", Language: "go", Kind: "method", SymbolHandle: "target", SymbolName: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Content: "return nil", ContentHash: "target"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "http.go", Language: "go", SHA256: "caller"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "caller", FilePath: "http.go", Language: "go", Kind: "function", Name: "Authenticate", Signature: "func Authenticate", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "caller-chunk", FilePath: "http.go", Language: "go", Kind: "function", SymbolHandle: "caller", SymbolName: "Authenticate", Signature: "func Authenticate", StartLine: 1, EndLine: 2, Content: "service.ValidateToken()", ContentHash: "caller"}},
		Relations: []model.Relation{{FromHandle: "caller", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .3, Source: "go-ast"}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.RelatedCandidates(context.Background(), []string{"target-chunk"}, "callers")
	if err != nil || len(got) != 1 || got[0].Symbol != "Authenticate" {
		t.Fatalf("callers=%+v err=%v", got, err)
	}
}

func TestRelatedCandidatesResolveCamelCaseGoTestName(t *testing.T) {
	s := openTestStore(t)
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "auth.go", Language: "go", SHA256: "auth"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "target", FilePath: "auth.go", Language: "go", Kind: "method", Name: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: "target-chunk", FilePath: "auth.go", Language: "go", Kind: "method", SymbolHandle: "target", SymbolName: "ValidateToken", Signature: "func ValidateToken", StartLine: 1, EndLine: 2, Content: "return nil", ContentHash: "target"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "auth_test.go", Language: "go", SHA256: "test"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "test", FilePath: "auth_test.go", Language: "go", Kind: "test", Name: "TestValidateExpiredToken", Signature: "func TestValidateExpiredToken", StartLine: 1, EndLine: 2, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "test-chunk", FilePath: "auth_test.go", Language: "go", Kind: "test", SymbolHandle: "test", SymbolName: "TestValidateExpiredToken", Signature: "func TestValidateExpiredToken", StartLine: 1, EndLine: 2, Content: "ValidateToken", ContentHash: "test"}},
		Relations: []model.Relation{{FromHandle: "test", UnresolvedTo: "ValidateExpiredToken", Kind: "tests", Confidence: .4, Source: "go-ast"}},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := s.RelatedCandidates(context.Background(), []string{"target-chunk"}, "tests")
	if err != nil || len(got) != 1 || got[0].Symbol != "TestValidateExpiredToken" {
		t.Fatalf("tests=%+v err=%v", got, err)
	}
}

func TestRelatedCandidatesResolveUnresolvedPHPRelations(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/Auth/TokenService.php", Language: "php", SHA256: "target"}, model.Extraction{
		Symbols: []model.Symbol{
			{Handle: "php-class", FilePath: "src/Auth/TokenService.php", Language: "php", Kind: "class", Name: "TokenService", QualifiedName: "App\\Auth\\TokenService", StartLine: 1, EndLine: 8, Confidence: 1},
			{Handle: "php-method", FilePath: "src/Auth/TokenService.php", Language: "php", Kind: "method", Name: "validateToken", QualifiedName: "App\\Auth\\TokenService::validateToken", StartLine: 2, EndLine: 7, Confidence: 1},
		},
		Chunks: []model.Chunk{
			{Handle: "php-class-chunk", FilePath: "src/Auth/TokenService.php", Language: "php", Kind: "class-outline", SymbolHandle: "php-class", SymbolName: "TokenService", StartLine: 1, EndLine: 1, Content: "class TokenService", ContentHash: "php-class"},
			{Handle: "php-method-chunk", FilePath: "src/Auth/TokenService.php", Language: "php", Kind: "method", SymbolHandle: "php-method", SymbolName: "validateToken", StartLine: 2, EndLine: 7, Content: "validateToken", ContentHash: "php-method"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/Http/AuthMiddleware.php", Language: "php", SHA256: "caller"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "php-caller", FilePath: "src/Http/AuthMiddleware.php", Language: "php", Kind: "method", Name: "handle", QualifiedName: "App\\Http\\AuthMiddleware::handle", StartLine: 1, EndLine: 5, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: "php-caller-chunk", FilePath: "src/Http/AuthMiddleware.php", Language: "php", Kind: "method", SymbolHandle: "php-caller", SymbolName: "handle", StartLine: 1, EndLine: 5, Content: "TokenService::validateToken", ContentHash: "php-caller"}},
		Relations: []model.Relation{
			{FromHandle: "php-caller", UnresolvedTo: "App\\Auth\\TokenService::validateToken", Kind: "calls", Confidence: .3, Source: "php"},
			{FromHandle: "php-caller", UnresolvedTo: "App\\Auth\\TokenService", Kind: "references", Confidence: .35, Source: "php"},
			{FromHandle: "php-caller", UnresolvedTo: "includes/bootstrap.inc", Kind: "imports", Confidence: .8, Source: "php"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "tests/TokenServiceTest.php", Language: "php", SHA256: "test"}, model.Extraction{
		Symbols:   []model.Symbol{{Handle: "php-test", FilePath: "tests/TokenServiceTest.php", Language: "php", Kind: "test", Name: "testExpiredTokenIsRejected", QualifiedName: "App\\Auth\\TokenServiceTest::testExpiredTokenIsRejected", StartLine: 1, EndLine: 5, Confidence: 1}},
		Chunks:    []model.Chunk{{Handle: "php-test-chunk", FilePath: "tests/TokenServiceTest.php", Language: "php", Kind: "test", SymbolHandle: "php-test", SymbolName: "testExpiredTokenIsRejected", StartLine: 1, EndLine: 5, Content: "validateToken", ContentHash: "php-test"}},
		Relations: []model.Relation{{FromHandle: "php-test", UnresolvedTo: "App\\Auth\\TokenService::validateToken", Kind: "tests", Confidence: .3, Source: "php"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "includes/bootstrap.inc", Language: "php", SHA256: "include"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "php-include", FilePath: "includes/bootstrap.inc", Language: "php", Kind: "file", Name: "includes/bootstrap.inc", QualifiedName: "includes/bootstrap.inc", StartLine: 1, EndLine: 3, Confidence: .7}},
		Chunks:  []model.Chunk{{Handle: "php-include-chunk", FilePath: "includes/bootstrap.inc", Language: "php", Kind: "procedural", SymbolHandle: "php-include", SymbolName: "includes/bootstrap.inc", StartLine: 1, EndLine: 3, Content: "bootstrap", ContentHash: "php-include"}},
	}); err != nil {
		t.Fatal(err)
	}

	callers, err := s.RelatedCandidates(context.Background(), []string{"php-method-chunk"}, "callers")
	if err != nil || !hasRelatedPath(callers, "src/Http/AuthMiddleware.php") {
		t.Fatalf("PHP callers=%+v err=%v", callers, err)
	}
	tests, err := s.RelatedCandidates(context.Background(), []string{"php-method-chunk"}, "tests")
	if err != nil || !hasRelatedPath(tests, "tests/TokenServiceTest.php") {
		t.Fatalf("PHP tests=%+v err=%v", tests, err)
	}
	imports, err := s.RelatedCandidates(context.Background(), []string{"php-include-chunk"}, "imports")
	if err != nil || !hasRelatedPath(imports, "src/Http/AuthMiddleware.php") {
		t.Fatalf("PHP imports=%+v err=%v", imports, err)
	}
	references, err := s.RelatedCandidates(context.Background(), []string{"php-class-chunk"}, "references")
	if err != nil || !hasRelatedPath(references, "src/Http/AuthMiddleware.php") {
		t.Fatalf("PHP references=%+v err=%v", references, err)
	}
}

func hasRelatedPath(candidates []model.RankedCandidate, path string) bool {
	for _, candidate := range candidates {
		if candidate.Path == path {
			return true
		}
	}
	return false
}

func TestRelatedCandidatesFilterImportAndReferenceKinds(t *testing.T) {
	s, err := Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	symbols := []model.Symbol{
		{Handle: "source", FilePath: "source.php", Language: "php", Kind: "function", Name: "source", StartLine: 1, EndLine: 1, Confidence: 1},
		{Handle: "import", FilePath: "import.php", Language: "php", Kind: "class", Name: "Imported", StartLine: 1, EndLine: 1, Confidence: 1},
		{Handle: "reference", FilePath: "reference.php", Language: "php", Kind: "class", Name: "Referenced", StartLine: 1, EndLine: 1, Confidence: 1},
	}
	chunks := []model.Chunk{
		{Handle: "source-chunk", FilePath: "source.php", Language: "php", Kind: "function", SymbolHandle: "source", SymbolName: "source", StartLine: 1, EndLine: 1, Content: "function source() {}"},
		{Handle: "import-chunk", FilePath: "import.php", Language: "php", Kind: "class", SymbolHandle: "import", SymbolName: "Imported", StartLine: 1, EndLine: 1, Content: "class Imported {}"},
		{Handle: "reference-chunk", FilePath: "reference.php", Language: "php", Kind: "class", SymbolHandle: "reference", SymbolName: "Referenced", StartLine: 1, EndLine: 1, Content: "class Referenced {}"},
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "import.php", Language: "php", SHA256: "import"}, model.Extraction{Symbols: symbols[1:2], Chunks: chunks[1:2]}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "reference.php", Language: "php", SHA256: "reference"}, model.Extraction{Symbols: symbols[2:], Chunks: chunks[2:]}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "source.php", Language: "php", SHA256: "source"}, model.Extraction{
		Symbols: symbols[:1], Chunks: chunks[:1],
		Relations: []model.Relation{
			{FromHandle: "source", ToHandle: "import", Kind: "imports", Confidence: 1},
			{FromHandle: "source", ToHandle: "reference", Kind: "references", Confidence: 1},
		},
	}); err != nil {
		t.Fatal(err)
	}

	imports, err := s.RelatedCandidates(context.Background(), []string{"source"}, "imports")
	if err != nil || len(imports) != 1 || imports[0].Symbol != "Imported" {
		t.Fatalf("imports=%+v err=%v", imports, err)
	}
	references, err := s.RelatedCandidates(context.Background(), []string{"source"}, "references")
	if err != nil || len(references) != 1 || references[0].Symbol != "Referenced" {
		t.Fatalf("references=%+v err=%v", references, err)
	}
}

func TestRelatedCandidatesMatchQualifiedModuleImportToFileAnchor(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "src/auth/token_service.rs", Language: "rust", SHA256: "target"}, model.Extraction{
		Symbols: []model.Symbol{{Handle: "target", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "crate_module", Name: "token_service", StartLine: 1, EndLine: 1, Confidence: 1}},
		Chunks:  []model.Chunk{{Handle: "target-chunk", FilePath: "src/auth/token_service.rs", Language: "rust", Kind: "module-outline", SymbolHandle: "target", SymbolName: "token_service", StartLine: 1, EndLine: 1, Content: "crate module", ContentHash: "target"}},
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
	got, err := s.RelatedCandidates(context.Background(), []string{"target"}, "imports")
	if err != nil || len(got) != 1 || got[0].Path != "src/http/middleware.rs" {
		t.Fatalf("qualified module imports=%+v err=%v", got, err)
	}
}

func TestRelatedCandidateHitsPreserveDirectionResolutionAndSource(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()
	ctx := context.Background()

	files := []struct {
		path      string
		handle    string
		chunk     string
		name      string
		relations []model.Relation
	}{
		{path: "auth/service.go", handle: "target", chunk: "target-chunk", name: "ValidateToken"},
		{path: "http/middleware.go", handle: "caller", chunk: "caller-chunk", name: "Authenticate", relations: []model.Relation{
			{FromHandle: "caller", ToHandle: "target", Kind: "calls", Confidence: .95, Source: "go-ast"},
		}},
		{path: "legacy/handler.go", handle: "lexical-caller", chunk: "lexical-caller-chunk", name: "LegacyAuthenticate", relations: []model.Relation{
			{FromHandle: "lexical-caller", UnresolvedTo: "ValidateToken", Kind: "calls", Confidence: .35, Source: "generic"},
		}},
		{path: "auth/child.go", handle: "child", chunk: "child-chunk", name: "ParseToken"},
		{path: "config/importer.go", handle: "importer", chunk: "importer-chunk", name: "LoadAuth", relations: []model.Relation{
			{FromHandle: "importer", ToHandle: "target", Kind: "imports", Confidence: .9, Source: "go-import"},
		}},
	}
	for _, file := range files {
		relations := append([]model.Relation(nil), file.relations...)
		if file.handle == "target" {
			relations = append(relations, model.Relation{FromHandle: "target", ToHandle: "child", Kind: "contains", Confidence: 1, Source: "go-ast"})
		}
		err := s.ReplaceFile(ctx, model.SourceFile{Path: file.path, Language: "go", SHA256: file.handle}, model.Extraction{
			Symbols:   []model.Symbol{{Handle: file.handle, FilePath: file.path, Language: "go", Kind: "function", Name: file.name, QualifiedName: file.name, StartLine: 1, EndLine: 2, Confidence: 1}},
			Chunks:    []model.Chunk{{Handle: file.chunk, FilePath: file.path, Language: "go", Kind: "function", SymbolHandle: file.handle, SymbolName: file.name, Signature: "func " + file.name, StartLine: 1, EndLine: 2, Content: file.name, ContentHash: file.handle}},
			Relations: relations,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name       string
		anchor     string
		relation   string
		candidate  string
		kind       string
		direction  model.RelationDirection
		resolved   bool
		confidence float64
		source     string
	}{
		{name: "caller to anchor", anchor: "target-chunk", relation: "callers", candidate: "caller-chunk", kind: "callers", direction: model.RelationIncoming, resolved: true, confidence: .95, source: "go-ast"},
		{name: "lexical caller", anchor: "target-chunk", relation: "callers", candidate: "lexical-caller-chunk", kind: "callers", direction: model.RelationIncoming, resolved: false, confidence: .35, source: "generic"},
		{name: "anchor to callee", anchor: "caller-chunk", relation: "callees", candidate: "target-chunk", kind: "callees", direction: model.RelationOutgoing, resolved: true, confidence: .95, source: "go-ast"},
		{name: "parent to child", anchor: "target-chunk", relation: "children", candidate: "child-chunk", kind: "children", direction: model.RelationOutgoing, resolved: true, confidence: 1, source: "go-ast"},
		{name: "child to parent", anchor: "child-chunk", relation: "parent", candidate: "target-chunk", kind: "parent", direction: model.RelationIncoming, resolved: true, confidence: 1, source: "go-ast"},
		{name: "importer to target", anchor: "importer-chunk", relation: "imports", candidate: "target-chunk", kind: "imports", direction: model.RelationOutgoing, resolved: true, confidence: .9, source: "go-import"},
		{name: "imported target to importer", anchor: "target-chunk", relation: "imports", candidate: "importer-chunk", kind: "imports", direction: model.RelationIncoming, resolved: true, confidence: .9, source: "go-import"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits, err := s.RelatedCandidateHits(ctx, []string{tt.anchor}, tt.relation)
			if err != nil {
				t.Fatal(err)
			}
			var found *model.RelationHit
			for i := range hits {
				if hits[i].Candidate.Handle == tt.candidate {
					found = &hits[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("candidate %q absent from hits: %+v", tt.candidate, hits)
			}
			want := model.RelationContext{AnchorHandle: tt.anchor, Kind: tt.kind, Direction: tt.direction, Confidence: tt.confidence, Source: tt.source, Resolved: tt.resolved}
			if found.Context != want {
				t.Fatalf("context = %+v, want %+v", found.Context, want)
			}
		})
	}
}
