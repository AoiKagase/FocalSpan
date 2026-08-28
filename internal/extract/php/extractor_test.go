package php

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestPHPExtractorCreatesCanonicalSymbolsAndSeparateChunks(t *testing.T) {
	content := `<?php
namespace App\Auth;

#[Service]
readonly class TokenService extends BaseService implements TokenValidator
{
    private string|int $token;
    public const TTL = 60;

    #[Test]
    public function &validateToken(?string $token): ?bool
    {
        $anonymous = function () use ($token) { return true; };
        $arrow = fn(string $value): string => $value;
        return true;
    }

    abstract public function contract(?string $value): bool;
}

function validate_token(string|int $token): bool { return true; }
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/Auth/TokenService.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	classSymbol := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService", "class")
	methodSymbol := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::validateToken", "test")
	requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::contract", "method")
	requireSymbol(t, got.Symbols, "App\\Auth\\validate_token", "function")
	if _, ok := findSymbol(got.Symbols, "anonymous"); ok {
		t.Fatal("anonymous function became a named symbol")
	}
	if _, ok := findSymbol(got.Symbols, "arrow"); ok {
		t.Fatal("arrow function became a named symbol")
	}
	if methodSymbol.ParentHandle != classSymbol.Handle {
		t.Fatalf("method parent=%q, want class handle %q", methodSymbol.ParentHandle, classSymbol.Handle)
	}
	property := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::token", "property")
	constant := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::TTL", "constant")
	if !hasRelation(got.Relations, classSymbol.Handle, methodSymbol.Handle, "contains") ||
		!hasRelation(got.Relations, classSymbol.Handle, property.Handle, "contains") ||
		!hasRelation(got.Relations, classSymbol.Handle, constant.Handle, "contains") {
		t.Fatalf("member containment missing: %+v", got.Relations)
	}
	outline := requireChunk(t, got.Chunks, "class-outline", classSymbol.Handle)
	if strings.Contains(outline.Content, "return true") || strings.Contains(outline.Content, "$anonymous") {
		t.Fatalf("class outline contains method body: %q", outline.Content)
	}
	if !hasChunkKind(got.Chunks, "test") || !hasChunkKind(got.Chunks, "method") || !hasChunkKind(got.Chunks, "function") {
		t.Fatalf("chunks=%+v", got.Chunks)
	}
	if methodSymbol.StartByte < 0 || methodSymbol.EndByte > len(content) || methodSymbol.StartByte >= methodSymbol.EndByte || methodSymbol.StartLine < 1 || methodSymbol.EndLine < methodSymbol.StartLine {
		t.Fatalf("invalid method span=%+v", methodSymbol)
	}
	if got.Chunks[0].FilePath != "src/Auth/TokenService.php" {
		t.Fatalf("chunk path=%q", got.Chunks[0].FilePath)
	}
}

func TestPHPExtractorSupportsPHPOnly(t *testing.T) {
	extractor := NewExtractor()
	if extractor.Name() != "php-structural" {
		t.Fatalf("name=%q", extractor.Name())
	}
	if !extractor.Supports("view.PHP", "php") || extractor.Supports("view.php", "text") {
		t.Fatal("unexpected Supports result")
	}
}

func TestPHPExtractorHandlesNamespaceFormsAndStableHandles(t *testing.T) {
	for _, namespace := range []string{"namespace App\\Auth;", "namespace App\\Auth {"} {
		content := "<?php\n" + namespace + "\nfunction validate_token(): bool { return true; }\n" + func() string {
			if strings.HasSuffix(namespace, "{") {
				return "}\n"
			}
			return ""
		}()
		got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth.php", Language: "php", Content: []byte(content)})
		if err != nil {
			t.Fatal(err)
		}
		requireSymbol(t, got.Symbols, "App\\Auth\\validate_token", "function")
	}

	base := "<?php\nnamespace App;\nfunction run(): void { return; }\n"
	shifted := "<?php\n// inserted\nnamespace App;\nfunction run(): void { return; }\n"
	first, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth.php", Language: "php", Content: []byte(base)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "auth.php", Language: "php", Content: []byte(shifted)})
	if err != nil {
		t.Fatal(err)
	}
	if requireSymbol(t, first.Symbols, "App\\run", "function").Handle != requireSymbol(t, second.Symbols, "App\\run", "function").Handle {
		t.Fatal("line movement changed function handle")
	}
}

func TestPHPExtractorCreatesAllClassLikeKindsAndAbstractMethods(t *testing.T) {
	content := `<?php
namespace App;

/** Contract documentation. */
interface Contract { public function validate(?string $value): bool; }
trait HasValidation { public function helper(): void {} }
enum Status { case Expired; }
abstract class Service implements Contract {
    /** method docs */
    abstract public function validate(?string $value): bool;
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "types.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	requireSymbol(t, got.Symbols, "App\\Contract", "interface")
	requireSymbol(t, got.Symbols, "App\\HasValidation", "trait")
	requireSymbol(t, got.Symbols, "App\\Status", "enum")
	service := requireSymbol(t, got.Symbols, "App\\Service", "class")
	method := requireSymbol(t, got.Symbols, "App\\Service::validate", "method")
	if method.ParentHandle != service.Handle || method.EndByte <= method.StartByte {
		t.Fatalf("method=%+v service=%+v", method, service)
	}
	if !strings.Contains(requireChunk(t, got.Chunks, "class-outline", service.Handle).Content, "Service") {
		t.Fatal("class outline lost declaration header")
	}
	if !strings.Contains(requireChunk(t, got.Chunks, "interface-outline", requireSymbol(t, got.Symbols, "App\\Contract", "interface").Handle).Content, "Contract documentation") {
		t.Fatal("interface doc comment was not retained")
	}
}

func TestPHPExtractorIsDeterministic(t *testing.T) {
	file := model.SourceFile{Path: "auth.php", Language: "php", Content: []byte("<?php\nclass Service { public function run(): void {} }\n")}
	first, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("extractions differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestPHPExtractorBuildsAliasesCallsReferencesAndIncludes(t *testing.T) {
	content := `<?php
namespace App\Auth;
use App\Auth\TokenService as AuthService;
use App\Repository\{TokenRepository, UserRepository};
use function App\Support\normalize_token;
use const App\Config\TOKEN_TTL;

class TokenService {
    public function validateToken(): bool {
        $this->helper();
        self::helper();
        static::helper();
        TokenService::helper();
        $service->validateToken();
        new TokenService();
    }
    private function helper(): void {}
}

class Caller {
    use SomeTrait;
    public function run(): void {
        AuthService::validateToken();
        include __DIR__ . '/bootstrap.inc';
        require_once dirname(__FILE__) . '/auth.php';
        require $dynamicPath;
    }
}

function normalize_token(string $value): string { return $value; }
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/Auth/Calls.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	service := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService", "class")
	validate := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::validateToken", "method")
	helper := requireSymbol(t, got.Symbols, "App\\Auth\\TokenService::helper", "method")
	caller := requireSymbol(t, got.Symbols, "App\\Auth\\Caller::run", "method")
	callerClass := requireSymbol(t, got.Symbols, "App\\Auth\\Caller", "class")
	if !hasRelation(got.Relations, service.Handle, validate.Handle, "contains") || !hasRelation(got.Relations, service.Handle, helper.Handle, "contains") {
		t.Fatalf("member relations missing: %+v", got.Relations)
	}
	if !hasRelation(got.Relations, validate.Handle, helper.Handle, "calls") || !hasRelation(got.Relations, caller.Handle, validate.Handle, "calls") {
		t.Fatalf("resolved calls missing: %+v", got.Relations)
	}
	if !hasUnresolvedRelation(got.Relations, validate.Handle, "validateToken", "calls") || !hasUnresolvedRelation(got.Relations, caller.Handle, "$dynamicPath", "imports") {
		t.Fatalf("unresolved relations missing: %+v", got.Relations)
	}
	if !hasUnresolvedRelation(got.Relations, callerClass.Handle, "SomeTrait", "references") {
		t.Fatalf("trait reference missing: %+v", got.Relations)
	}
	if (!hasUnresolvedTarget(got.Relations, "App\\Auth\\TokenService", "imports") && !hasResolvedTarget(got.Relations, service.Handle, "imports")) || !hasUnresolvedRelation(got.Relations, caller.Handle, "src/Auth/bootstrap.inc", "imports") {
		t.Fatalf("imports missing: %+v", got.Relations)
	}
	if hasUnresolvedRelation(got.Relations, caller.Handle, "../", "imports") {
		t.Fatal("escaping include was resolved")
	}
	if !relationsUnique(got.Relations) {
		t.Fatalf("duplicate relations: %+v", got.Relations)
	}
}

func TestPHPExtractorBuildsTypeAndAttributeReferences(t *testing.T) {
	content := `<?php
namespace App;

class Token {}
class Base {}
interface Contract {}
class Result {}

#[Entity(Token::class)]
class Handler extends Base implements Contract {
    private ?Token $token;

    public function run(Token $input): Result {
        return new Result();
    }
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "handler.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	token := requireSymbol(t, got.Symbols, "App\\Token", "class")
	base := requireSymbol(t, got.Symbols, "App\\Base", "class")
	contract := requireSymbol(t, got.Symbols, "App\\Contract", "interface")
	result := requireSymbol(t, got.Symbols, "App\\Result", "class")
	handler := requireSymbol(t, got.Symbols, "App\\Handler", "class")
	property := requireSymbol(t, got.Symbols, "App\\Handler::token", "property")
	method := requireSymbol(t, got.Symbols, "App\\Handler::run", "method")
	for _, relation := range []struct {
		from, to model.Symbol
		name     string
	}{
		{handler, token, "class attribute"},
		{handler, base, "extends"},
		{handler, contract, "implements"},
		{property, token, "property type"},
		{method, token, "parameter type"},
		{method, result, "return type"},
	} {
		if !hasRelation(got.Relations, relation.from.Handle, relation.to.Handle, "references") {
			t.Fatalf("missing %s reference: %+v", relation.name, got.Relations)
		}
	}
}

func TestPHPExtractorRecognizesPHPUnitTestsAndTestRelations(t *testing.T) {
	content := `<?php
namespace App\Auth;
use PHPUnit\Framework\TestCase;

function validateToken(): bool { return true; }

class TokenServiceTest extends TestCase {
    public function testExpiredTokenIsRejected(): void { validateToken(); }
    #[Test]
    public function expired_token_is_rejected(): void { validateToken(); }
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "tests/TokenServiceTest.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	testClass := requireSymbol(t, got.Symbols, "App\\Auth\\TokenServiceTest", "class")
	first := requireSymbol(t, got.Symbols, "App\\Auth\\TokenServiceTest::testExpiredTokenIsRejected", "test")
	second := requireSymbol(t, got.Symbols, "App\\Auth\\TokenServiceTest::expired_token_is_rejected", "test")
	validate := requireSymbol(t, got.Symbols, "App\\Auth\\validateToken", "function")
	if !hasRelation(got.Relations, testClass.Handle, first.Handle, "contains") || !hasRelation(got.Relations, testClass.Handle, second.Handle, "contains") {
		t.Fatalf("test containment missing: %+v", got.Relations)
	}
	if !hasRelation(got.Relations, first.Handle, validate.Handle, "tests") || !hasRelation(got.Relations, second.Handle, validate.Handle, "tests") {
		t.Fatalf("test relations missing: %+v", got.Relations)
	}
}

func TestPHPExtractorKeepsPartialExtractionAfterMalformedInput(t *testing.T) {
	content := `<?php
class Good { public function ok(): void {} }
$value = /* unclosed comment
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "broken.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	requireSymbol(t, got.Symbols, "Good", "class")
	if !hasChunkKind(got.Chunks, "procedural") || !hasDiagnosticCode(got.Diagnostics, "php_unclosed_comment") || !hasDiagnosticCode(got.Diagnostics, "php_partial_extraction") {
		t.Fatalf("partial extraction=%+v", got)
	}
}

func TestPHPBuilderReturnsCancellationAfterPartialBuild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := buildExtraction(ctx, model.SourceFile{Path: "empty.php", Language: "php", Content: []byte("<?php")}, parseResult{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context.Canceled", err)
	}
}

func hasUnresolvedRelation(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasUnresolvedTarget(relations []model.Relation, target, kind string) bool {
	for _, relation := range relations {
		if relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasResolvedTarget(relations []model.Relation, target, kind string) bool {
	for _, relation := range relations {
		if relation.ToHandle == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func relationsUnique(relations []model.Relation) bool {
	seen := make(map[string]bool)
	for _, relation := range relations {
		key := relation.FromHandle + "\x00" + relation.ToHandle + "\x00" + relation.UnresolvedTo + "\x00" + relation.Kind
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func requireSymbol(t *testing.T, symbols []model.Symbol, qualified, kind string) model.Symbol {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.QualifiedName == qualified && symbol.Kind == kind {
			return symbol
		}
	}
	t.Fatalf("symbol %q kind %q not found in %+v", qualified, kind, symbols)
	return model.Symbol{}
}

func findSymbol(symbols []model.Symbol, name string) (model.Symbol, bool) {
	for _, symbol := range symbols {
		if symbol.Name == name {
			return symbol, true
		}
	}
	return model.Symbol{}, false
}

func requireChunk(t *testing.T, chunks []model.Chunk, kind, symbolHandle string) model.Chunk {
	t.Helper()
	for _, chunk := range chunks {
		if chunk.Kind == kind && chunk.SymbolHandle == symbolHandle {
			return chunk
		}
	}
	t.Fatalf("chunk kind %q symbol %q not found in %+v", kind, symbolHandle, chunks)
	return model.Chunk{}
}

func hasChunkKind(chunks []model.Chunk, kind string) bool {
	for _, chunk := range chunks {
		if chunk.Kind == kind {
			return true
		}
	}
	return false
}

func hasRelation(relations []model.Relation, from, to, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.ToHandle == to && relation.Kind == kind {
			return true
		}
	}
	return false
}
