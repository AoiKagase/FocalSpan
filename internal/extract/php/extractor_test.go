package php

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/model"
)

func TestPHPExtractorCreatesCanonicalSymbolsAndSeparateChunks(t *testing.T) {
	content := `<?php
namespace App\Auth;
use PHPUnit\Framework\TestCase;

#[Service]
readonly class TokenService extends TestCase implements TokenValidator
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

func TestPHPExtractorAssignsDistinctHandlesToRepeatedFallbackChunks(t *testing.T) {
	content := "<?php\nclass First {}\nclass Second {}\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "repeated.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, chunk := range got.Chunks {
		if seen[chunk.Handle] {
			t.Fatalf("duplicate chunk handle %q in %+v", chunk.Handle, got.Chunks)
		}
		seen[chunk.Handle] = true
	}
}

func TestPHPExtractorDoesNotUseFilePathAsFallbackSymbol(t *testing.T) {
	content := "<?php\nclass First {}\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/First.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	file := requireSymbol(t, got.Symbols, "src/First.php", "file")
	for _, chunk := range got.Chunks {
		if chunk.SymbolHandle == file.Handle && chunk.SymbolName != "" {
			t.Fatalf("file-owned fallback chunk has symbol name %q: %+v", chunk.SymbolName, chunk)
		}
	}
}

func TestPHPExtractorUsesBoundedOverlappingFallbackWindows(t *testing.T) {
	content := "<?php\n" + strings.Repeat("echo 1;\n", 170)
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "procedural.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	procedural := make([]model.Chunk, 0)
	for _, chunk := range got.Chunks {
		if chunk.Kind == "procedural" {
			procedural = append(procedural, chunk)
		}
	}
	if len(procedural) < 3 {
		t.Fatalf("procedural fallback windows=%+v", procedural)
	}
	for index, chunk := range procedural {
		if lines := chunk.EndLine - chunk.StartLine + 1; lines > 80 {
			t.Fatalf("fallback window %d has %d lines: %+v", index, lines, chunk)
		}
		if index > 0 && procedural[index].StartLine > procedural[index-1].EndLine-10 {
			t.Fatalf("fallback windows do not overlap by ten lines: previous=%+v current=%+v", procedural[index-1], chunk)
		}
	}
}

func TestPHPFallbackMakesProgressPastAnOversizedToken(t *testing.T) {
	content := strings.Repeat("heredoc body\n", 200)
	builder := &phpBuilder{
		ctx:    context.Background(),
		file:   model.SourceFile{Path: "oversized.php", Language: "php", Content: []byte(content)},
		parsed: parseResult{Tokens: []Token{{Kind: KindHeredoc, Text: content, StartByte: 0, EndByte: len(content), StartLine: 1, EndLine: 200}}},
	}
	done := make(chan struct{})
	go func() {
		builder.appendFallbackRange(0, 0, "procedural")
		close(done)
	}()
	select {
	case <-done:
		if len(builder.result.Chunks) != 1 {
			t.Fatalf("oversized token chunks=%+v", builder.result.Chunks)
		}
	case <-time.After(time.Second):
		t.Fatal("fallback range did not make progress past oversized token")
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

func TestPHPExtractorMakesRelativeIncludesSourceRelativeAndRejectsAbsolutePaths(t *testing.T) {
	content := `<?php
namespace App\Auth;

final class Loader {
    public function run(): void {
        include 'bootstrap.inc';
        require 'C:\\secrets\\bootstrap.inc';
        require '\\\\server\\share\\bootstrap.inc';
    }
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/Auth/Loader.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	run := requireSymbol(t, got.Symbols, "App\\Auth\\Loader::run", "method")
	if !hasUnresolvedRelation(got.Relations, run.Handle, "src/Auth/bootstrap.inc", "imports") {
		t.Fatalf("relative include missing: %+v", got.Relations)
	}
	for _, relation := range got.Relations {
		if relation.FromHandle != run.Handle || relation.Kind != "imports" {
			continue
		}
		target := strings.ReplaceAll(relation.UnresolvedTo, "\\", "/")
		if strings.HasPrefix(target, "c:/") || strings.HasPrefix(target, "//") {
			t.Fatalf("absolute include was accepted: %+v", relation)
		}
	}
}

func TestPHPExtractorRecordsTopLevelIncludesOnFileOwner(t *testing.T) {
	content := `<?php
include 'includes/bootstrap.inc';
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "src/Entry.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	file := requireSymbol(t, got.Symbols, "src/Entry.php", "file")
	if !hasUnresolvedRelation(got.Relations, file.Handle, "src/includes/bootstrap.inc", "imports") {
		t.Fatalf("top-level include missing: %+v", got.Relations)
	}
}

func TestPHPExtractorBuildsTypeAndAttributeReferences(t *testing.T) {
	content := `<?php
namespace App;

class Token {}
class Base {}
interface Contract {}
class Result {}
class Entity {}

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
	entity := requireSymbol(t, got.Symbols, "App\\Entity", "class")
	handler := requireSymbol(t, got.Symbols, "App\\Handler", "class")
	property := requireSymbol(t, got.Symbols, "App\\Handler::token", "property")
	method := requireSymbol(t, got.Symbols, "App\\Handler::run", "method")
	for _, relation := range []struct {
		from, to model.Symbol
		name     string
	}{
		{handler, token, "class attribute"},
		{handler, entity, "attribute name"},
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

func TestPHPExtractorLimitsTestClassificationToPHPUnitClassesAndExactAttributes(t *testing.T) {
	content := `<?php
namespace App\Tests;
use PHPUnit\Framework\TestCase;

class PlainClass {
    public function testLooksLikeATest(): void {}
    #[Test]
    public function attributeLooksLikeATest(): void {}
}

	class RealTest extends TestCase {
	    public function testByName(): void {}
	    #[NotATest]
	    public function namedWithWrongAttribute(): void {}
	    #[Test]
	    public function namedByAttribute(): void {}
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "tests/classification.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"App\\Tests\\PlainClass::testLooksLikeATest",
		"App\\Tests\\PlainClass::attributeLooksLikeATest",
		"App\\Tests\\RealTest::namedWithWrongAttribute",
	} {
		requireSymbol(t, got.Symbols, name, "method")
	}
	for _, name := range []string{
		"App\\Tests\\RealTest::testByName",
		"App\\Tests\\RealTest::namedByAttribute",
	} {
		requireSymbol(t, got.Symbols, name, "test")
	}
}

func TestPHPExtractorResolvesNamespacedAndAbsoluteCallsWithoutAmbiguity(t *testing.T) {
	content := `<?php
namespace App\Other;
function helper(): void {}
final class Utility {
    public static function ping(): void {}
}

namespace App\Feature;
function helper(): void {}
final class Utility {
    public static function ping(): void {}
}
function caller(): void {
    helper();
    \App\Feature\helper();
    App\Feature\Utility::ping();
    \App\Feature\Utility::ping();
}
function absoluteCaller(): void {
    \App\Other\helper();
    \App\Other\Utility::ping();
}
`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "calls.php", Language: "php", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	caller := requireSymbol(t, got.Symbols, "App\\Feature\\caller", "function")
	absoluteCaller := requireSymbol(t, got.Symbols, "App\\Feature\\absoluteCaller", "function")
	featureHelper := requireSymbol(t, got.Symbols, "App\\Feature\\helper", "function")
	utilityPing := requireSymbol(t, got.Symbols, "App\\Feature\\Utility::ping", "method")
	otherHelper := requireSymbol(t, got.Symbols, "App\\Other\\helper", "function")
	otherUtilityPing := requireSymbol(t, got.Symbols, "App\\Other\\Utility::ping", "method")
	if !hasRelation(got.Relations, caller.Handle, featureHelper.Handle, "calls") {
		t.Fatalf("namespaced calls missing: %+v", got.Relations)
	}
	if !hasRelation(got.Relations, caller.Handle, utilityPing.Handle, "calls") {
		t.Fatalf("qualified static calls missing: %+v", got.Relations)
	}
	if !hasRelation(got.Relations, absoluteCaller.Handle, otherHelper.Handle, "calls") || !hasRelation(got.Relations, absoluteCaller.Handle, otherUtilityPing.Handle, "calls") {
		t.Fatalf("absolute calls missing: %+v", got.Relations)
	}
	if hasRelation(got.Relations, caller.Handle, otherHelper.Handle, "calls") || hasRelation(got.Relations, caller.Handle, otherUtilityPing.Handle, "calls") {
		t.Fatalf("qualified calls leaked into bare caller: %+v", got.Relations)
	}
	if hasRelation(got.Relations, caller.Handle, otherHelper.Handle, "calls") {
		t.Fatalf("ambiguous bare call resolved to wrong namespace: %+v", got.Relations)
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
