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
