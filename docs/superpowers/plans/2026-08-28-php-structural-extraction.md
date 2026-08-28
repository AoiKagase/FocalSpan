# PHP Structural Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic, pure-Go first-class PHP and content-aware `.inc` extraction to FocalSpan while preserving existing contracts and Go evaluation behavior.

**Architecture:** Keep language detection in `internal/repository`, add a stateful lexer and tolerant declaration parser under `internal/extract/php`, and convert parser nodes into existing `model` symbols/chunks/relations. Use the existing SQLite tables and extend only store-side unresolved relation lookup so local handles remain resolved while canonical cross-file names and safe include paths remain searchable.

**Tech Stack:** Go 1.26+, standard library byte scanning and context cancellation, existing `model.HandleAllocator`, `modernc.org/sqlite`, existing FTS5/search/rank/budget/eval packages, and no new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-28-php-structural-extraction-design.md`

## Global Constraints

- Use pure Go; keep `CGO_ENABLED=0` working.
- Never invoke PHP runtime, Composer, repository PHP code, external parser processes, package installation, network access, Tree-sitter, or a new parser dependency.
- Do not change the SQLite schema or CLI/MCP tool names and payload contracts.
- Preserve existing user changes in `README.md` and `.serena/`; stage explicit task files only.
- Keep production code under `cmd/focalspan` or `internal/*`; keep fixtures under `testdata/`.
- Begin every behavior change with a deterministic failing test and verify the failure before production code.
- PHP parse problems remain indexable; only context cancellation is returned as an extraction error.
- Use half-open UTF-8 byte offsets and one-based inclusive line ranges; never use line numbers in stable symbol identities.
- Keep CLI output on stdout, diagnostics/logging on stderr, and MCP stdout protocol-only.
- Sort symbols, chunks, relations, and deterministic candidate output before returning them.
- Run targeted tests after each task, then `git diff --check`; create narrow commits containing only that task's files.

## File Map

- Modify `internal/repository/scanner.go` and `internal/repository/scanner_test.go` for extension and content-aware detection.
- Create `internal/extract/php/token.go` for token kinds and spans.
- Create `internal/extract/php/lexer.go` for PHP/HTML stateful scanning and bounded lexer diagnostics.
- Create `internal/extract/php/parser.go` for tolerant scope/declaration parsing and internal parse nodes.
- Create `internal/extract/php/builder.go` for model conversion, canonical names, chunks, relations, and coverage windows.
- Create `internal/extract/php/extractor.go` for the public extractor interface and cancellation orchestration.
- Create `internal/extract/php/lexer_test.go` for token stream, state, span, malformed-input, and cancellation tests.
- Create `internal/extract/php/extractor_test.go` for symbols, chunks, relations, error recovery, and deterministic output tests.
- Modify `internal/extract/extract_test.go` and `internal/app/service.go`/tests for registry ordering and PHP selection.
- Modify `internal/store/store.go` and `internal/store/relation_test.go` for unresolved canonical-name/path relation lookup without migrations.
- Add `testdata/repos/phpsample/` PHP, `.inc`, `.phtml`, README, and large unrelated fixture files.
- Add `testdata/eval/php-cases.jsonl` and extend `docs/design.md`, `docs/implementation-plan.md`, `docs/evaluation.md`, and the existing user-modified `README.md` without replacing its Japanese section.

---

### Task 1: Content-aware language detection

**Files:**

- Modify: `internal/repository/scanner.go`
- Test: `internal/repository/scanner_test.go`

**Interfaces:**

- Produces `DetectLanguageContent(path string, content []byte) string`.
- Preserves `DetectLanguage(path string) string` and makes PHP-family extension matching case-insensitive.
- Makes `Scanner.Scan` call `DetectLanguageContent` after BOM removal and UTF-8 validation.

- [ ] **Step 1: Write the failing tests**

Add this table test:

```go
func TestDetectLanguageContentPHPAndIncRules(t *testing.T) {
    tests := []struct{ name, path, content, want string }{
        {"php", "index.php", "", "php"},
        {"phtml", "template.phtml", "", "php"},
        {"uppercase-family", "legacy.PHP5", "", "php"},
        {"inc-php", "auth.inc", "<?php echo 1;", "php"},
        {"inc-short-echo", "view.inc", "<?= $title ?>", "php"},
        {"inc-short-tag", "short.inc", "<? echo 1;", "php"},
        {"inc-xml", "xml.inc", "<?xml version=\"1.0\"?>", "text"},
        {"inc-plain", "plain.inc", "plain text", "text"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := DetectLanguageContent(tt.path, []byte(tt.content)); got != tt.want {
                t.Fatalf("DetectLanguageContent(%q)=%q, want %q", tt.path, got, tt.want)
            }
        })
    }
}
```

Add a scanner test that writes `auth.inc` containing `<?PHP`, scans it, and asserts its `SourceFile.Language` is `php`.

- [ ] **Step 2: Run the focused tests and verify RED**

Run `go test ./internal/repository -run 'TestDetectLanguageContentPHPAndIncRules|TestScannerDetectsPHPIncContent' -v`.

Expected: compile failure because `DetectLanguageContent` and the scanner call do not exist.

- [ ] **Step 3: Implement the minimal detection change**

Keep `DetectLanguage` path-only. Add `.phtml`, `.php3`, `.php4`, `.php5`, `.php7`, `.php8`, and `.phps`. For `.inc`, lower-case the content and recognize `<?php`, `<?=`, or `<?` unless the following text is `xml` case-insensitively. Return `text` for `.inc` without a recognized tag. Make the scanner pass validated, BOM-stripped content to the new function.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run:

```text
go test ./internal/repository -run 'TestDetectLanguageContentPHPAndIncRules|TestScannerDetectsPHPIncContent' -v
go test ./internal/repository
```

Expected: all repository tests pass and existing `.go`/secret/binary behavior is unchanged.

- [ ] **Step 5: Check and commit**

Run `git diff --check`, stage only `internal/repository/scanner.go` and `internal/repository/scanner_test.go`, and commit with `feat: detect PHP inc files from content`.

### Task 2: PHP token model and stateful lexer

**Files:**

- Create: `internal/extract/php/token.go`
- Create: `internal/extract/php/lexer.go`
- Test: `internal/extract/php/lexer_test.go`

**Interfaces:**

- Produces `type Token struct { Kind Kind; Text string; StartByte, EndByte, StartLine, EndLine int }`.
- Produces `func Lex(ctx context.Context, content []byte) ([]Token, []model.Diagnostic, error)`.
- Exposes stable token kinds for tags, identifiers, variables, keywords, punctuation, operators, whitespace, comments, strings, heredoc/nowdoc, and inline HTML.

- [ ] **Step 1: Write lexer RED tests**

Test a mixed CRLF input containing `<div>`, `<?php`, doc/block/line/hash comments, escaped single/double strings with braces, `#[Attribute]`, heredoc, nowdoc, `?>`, a second HTML block, and UTF-8 text. Assert expected token kind/text, first start `(0, 1)`, final `EndByte == len(content)`, and correct one-based lines. Add cases for multiple PHP blocks, CRLF, malformed unclosed string/comment/heredoc diagnostics, unknown bytes, and braces inside strings/comments.

Add cancellation:

```go
func TestLexHonorsCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if _, _, err := Lex(ctx, []byte("<?php "+strings.Repeat("$x = 1; ", 1000))); !errors.Is(err, context.Canceled) {
        t.Fatalf("err=%v, want context cancellation", err)
    }
}
```

- [ ] **Step 2: Run lexer tests and verify RED**

Run `go test ./internal/extract/php -run 'TestLex' -v`.

Expected: compile failure because the PHP package and `Lex` do not exist.

- [ ] **Step 3: Implement token kinds and byte/line accounting**

Define `Kind` as a string-backed type. Scan the original byte slice, update line state only on `\n`, preserve CR in token text, preserve original UTF-8 bytes, and guarantee `0 <= StartByte <= EndByte <= len(content)`.

- [ ] **Step 4: Implement PHP/HTML lexical states**

Start in inline HTML mode. Recognize `<?php` with a word boundary, `<?=`, and `<?` except `<?xml` case-insensitively. In PHP mode scan whitespace, variables, identifiers, a keyword lookup set, longest operators first, and punctuation. Track escapes in quoted strings, consume `//`, `#`, `/* */`, and `/** */`, and produce bounded diagnostics at EOF. Match heredoc/nowdoc terminators at line start as one token. Emit `?>` and return to HTML mode. Emit unknown bytes and continue.

- [ ] **Step 5: Run lexer tests and verify GREEN**

Run:

```text
gofmt -w internal/extract/php/token.go internal/extract/php/lexer.go internal/extract/php/lexer_test.go
go test ./internal/extract/php -run 'TestLex' -v
```

Expected: all lexer tests pass, including malformed-input recovery and cancellation.

- [ ] **Step 6: Check and commit**

Run `git diff --check`, stage only the three PHP lexer files, and commit with `feat: add stateful PHP lexer`.

### Task 3: Tolerant declarations, symbols, and outline/body chunks

**Files:**

- Create: `internal/extract/php/parser.go`
- Create: `internal/extract/php/builder.go`
- Create: `internal/extract/php/extractor.go`
- Test: `internal/extract/php/extractor_test.go`
- Modify: `internal/extract/extract_test.go`

**Interfaces:**

- Produces `php.Extractor`, `php.NewExtractor() php.Extractor`, `Name() string`, `Supports(path, language string) bool`, and `Extract(context.Context, model.SourceFile) (model.Extraction, error)`.
- Produces internal parse nodes carrying token ranges, namespace, modifiers, attributes, doc comments, class parent, and signature ranges.
- Produces canonical symbols, stable handles, class/member `contains` relations, exact spans, and non-overlapping outline/body chunks.

- [ ] **Step 1: Write extraction RED tests**

Use a source with `namespace App\\Auth`, class/interface/trait/enum declarations, `readonly class`, `extends`, `implements`, property promotion, class constant, `#[Test]`, an abstract method, `&validateToken(?string): bool`, a named global function, anonymous function, and arrow function. Assert symbols for `App\\Auth\\TokenService`, `App\\Auth\\TokenService::validateToken`, the test method, and `App\\Auth\\validate_token`; assert anonymous/arrow functions are absent; assert class-to-method/property/constant `contains`; assert `class-outline`, `method`, `test`, and `function` chunks exist; assert method body text is absent from the class outline.

Add table tests for semicolon/braced namespace, interface, trait, enum, readonly class, abstract/interface semicolon methods, property/constant, multiline signatures, stable handles after a leading line insertion, exact byte/line ranges, and repeated extraction equality. Assert each method `ParentHandle` equals its class handle.

- [ ] **Step 2: Run extraction tests and verify RED**

Run `go test ./internal/extract/php ./internal/extract -run 'TestPHP|TestRegistry' -v`.

Expected: compile failures for the new package and extractor methods.

- [ ] **Step 3: Implement parser token utilities and scope recovery**

Implement a cursor over non-trivia tokens while retaining trivia indexes. Add delimiter matching for `()`, `[]`, and `{}` that uses lexer token kinds, so delimiters inside strings/comments/heredocs are ignored. Parse namespace statements, attributes, modifiers, class-like declarations, functions, methods, properties, constants, and semicolon-terminated methods. On malformed declarations emit `php_malformed_declaration`, advance to the next safe semicolon or declaration keyword, and retain earlier nodes. Close open scopes at EOF and emit `php_unbalanced_brace` for nonzero depth.

- [ ] **Step 4: Implement canonical names, handles, and symbols**

Remove a leading `\` from canonical names, join namespace segments with `\`, and use `::` for class members. Allocate handles with `allocator.Allocate("sym", file.Path, "php", kind, qualifiedName, model.NormalizeSignature(signature))`. Use declaration token spans for source locations and confidence `1` for complete nodes, lower confidence for recovered nodes.

- [ ] **Step 5: Implement chunks and useful-content coverage**

Create `class-outline`, `interface-outline`, `trait-outline`, and `enum-outline` chunks from attributes/doc/modifiers/header through the opening brace. Keep property/constant outline chunks separate with exact declaration spans. Create `function`, `method`, and `test` chunks through a matching body brace or semicolon. Create `procedural` chunks for top-level PHP tokens not owned by declarations and `template` chunks for inline HTML outside declarations. Generate fallback windows for uncovered useful intervals with 80-line size, 10-line overlap, and 160-line maximum. Never add an unconditional full-file chunk beside named chunks.

- [ ] **Step 6: Run extraction tests and verify GREEN**

Run:

```text
gofmt -w internal/extract/php internal/extract/extract_test.go
go test ./internal/extract/php ./internal/extract -v
```

Expected: PHP declaration/chunk tests pass and existing extractor tests remain green.

- [ ] **Step 7: Check and commit**

Run `git diff --check`, stage only the PHP parser/builder/extractor files and focused registry test changes, and commit with `feat: extract PHP declarations and chunks`.

### Task 4: Aliases, relations, includes, calls, PHPUnit, and recovery

**Files:**

- Modify: `internal/extract/php/parser.go`, `internal/extract/php/builder.go`, `internal/extract/php/extractor.go`
- Test: `internal/extract/php/extractor_test.go`

**Interfaces:**

- Produces `contains`, `imports`, `references`, `calls`, and `tests` relations in existing `model.Relation` fields.
- Preserves unresolved targets as short canonical names or safe repository-relative paths in `UnresolvedTo`.
- Produces bounded PHP diagnostics while returning partial extraction for malformed source.

- [ ] **Step 1: Write relation and recovery RED tests**

Use a source with namespace imports, grouped imports, class alias, function/constant imports, `use SomeTrait`, named/local/static/receiver calls, `new TokenService()`, static class references, `include __DIR__ . '/bootstrap.inc'`, `require_once dirname(__FILE__) . '/auth.php'`, and `require $dynamicPath`. Assert local calls have `ToHandle`; unknown receiver and dynamic include have terminal `UnresolvedTo`; trait use is `references`, imports are `imports`, and static include is not linked outside the repository. Assert relation tuples are unique.

Add PHPUnit tests for imported `TestCase`, `#[Test]`, `test...` methods, and `tests` relations from test methods. Add malformed input with a valid declaration before an unclosed string/comment/brace; assert earlier symbols/chunks, a fallback chunk, `php_partial_extraction`, the specific diagnostic, and cancellation error.

- [ ] **Step 2: Run relation tests and verify RED**

Run `go test ./internal/extract/php -run 'TestPHP(Relations|Includes|Calls|PHPUnit|Recovery|Cancellation)' -v`.

Expected: assertions fail because alias tables, relation building, include folding, and diagnostics are not implemented.

- [ ] **Step 3: Implement namespace imports and alias tables**

Parse namespace-level `use`, split grouped imports, classify `class`/`function`/`const`, apply `as` aliases, and scope aliases to the namespace block. Treat class-scope `use` as a trait reference. Resolve a target to a local handle only when its canonical name or alias matches exactly one same-file declaration; otherwise preserve the canonical unresolved target.

- [ ] **Step 4: Implement type/reference and call relations**

Inspect signature ranges for `extends`, `implements`, parameter/return/property types, attributes, `new`, and static class references and emit `references`. Recognize named functions, `$this->method`, `self::method`, `static::method`, `parent::method`, `Class::method`, `$object->method`, and `new Class`. Resolve only unambiguous local cases; store unresolved calls by terminal name and put only a short lexical expression in `Relation.Source`.

- [ ] **Step 5: Implement safe include folding and procedural ownership**

Fold quoted strings, `__DIR__`, `__FILE__`, `dirname(__FILE__)`, and `.` concatenation into a slash-normalized path relative to the source file directory. Reject absolute paths and normalized `../` escapes. Store rejected/dynamic expressions as bounded unresolved values. Attach include relations to the containing function/method or file-scope procedural owner.

- [ ] **Step 6: Implement PHPUnit classification and test relations**

Resolve class `extends` against aliases. Mark methods beginning with `test` or carrying a `Test` attribute as kind `test`. For each test-owned call emit a `tests` relation to the resolved handle or unresolved terminal name. Deduplicate by `(FromHandle, ToHandle, UnresolvedTo, Kind)`.

- [ ] **Step 7: Implement diagnostics and fallback**

Keep lexer diagnostics, add bounded parser diagnostics, return partial extraction for malformed PHP, and build fallback windows from useful tokens not covered by declaration/member chunks. Check `ctx.Err()` in token and declaration loops and return it immediately.

- [ ] **Step 8: Run relation/recovery tests and commit**

Run:

```text
gofmt -w internal/extract/php
go test ./internal/extract/php -v
go test ./internal/extract/...
git diff --check
```

Stage only PHP relation/recovery source and tests and commit with `feat: add PHP relations and recovery`.

### Task 5: Registry, store lookup, and application integration

**Files:**

- Modify: `internal/app/service.go`
- Modify: `internal/extract/extract_test.go`
- Modify: `internal/store/store.go`
- Test: `internal/store/relation_test.go`
- Test: `internal/app/service_test.go`

**Interfaces:**

- Registry order becomes `extract.NewRegistry(goast.NewExtractor(), php.NewExtractor(), generic.NewExtractor())`.
- Existing `Store.RelatedCandidates` and `Store.SearchFTS` signatures remain unchanged.
- Unresolved `calls`, `tests`, `imports`, and `references` can match same-name symbols, qualified names, or safe file paths without a schema migration.

- [ ] **Step 1: Write integration RED tests**

Add:

```go
func TestRegistryPrefersPHPExtractorOverGeneric(t *testing.T) {
    r := NewRegistry(goast.NewExtractor(), php.NewExtractor(), generic.NewExtractor())
    got, ok := r.For("view.PHP", "php")
    if !ok || got.Name() != "php-structural" {
        t.Fatalf("extractor=%v ok=%v", got, ok)
    }
}
```

Add store fixtures with symbols `App\\Auth\\TokenService`, `TokenService::validateToken`, and a `.inc` file path, plus unresolved `calls`, `tests`, and `imports` relations. Assert `RelatedCandidates` returns targets for `callers`, `tests`, and `imports`, while the existing Go unresolved-call test still returns `Authenticate`. Add an app test that indexes a temp `.php` and `.inc` repository and asserts PHP language, symbols, and chunks are stored.

- [ ] **Step 2: Run focused tests and verify RED**

Run `go test ./internal/extract ./internal/store ./internal/app -run 'TestRegistryPrefersPHPExtractorOverGeneric|TestRelatedCandidates.*PHP|TestAppIndexesPHP' -v`.

Expected: registry selects generic or the PHP import is missing, and unresolved qualified/path relation assertions fail.

- [ ] **Step 3: Register PHP before generic**

Import `internal/extract/php` and construct the registry in exactly this order:

```go
registry := extract.NewRegistry(
    goast.NewExtractor(),
    php.NewExtractor(),
    generic.NewExtractor(),
)
```

- [ ] **Step 4: Extend store-side unresolved matching without schema changes**

Keep existing exact-handle branches. For `callers` and `tests`, add SQL predicates comparing relation `unresolved_to` against target symbol `name` and `qualified_name`. For `imports`/`references`, compare `unresolved_to` against symbols and `files.path` while retaining handle-based neighbor behavior. Bind all values as SQL parameters, preserve deterministic ordering, and do not alter `migrations/001_initial.sql`.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run:

```text
gofmt -w internal/app/service.go internal/extract/extract_test.go internal/store/store.go internal/store/relation_test.go internal/app/service_test.go
go test ./internal/extract ./internal/store ./internal/app -v
```

Expected: PHP registry/store/app tests pass and all existing Go tests pass.

- [ ] **Step 6: Check and commit**

Run `git diff --check`, stage only the registry/store/app files for this task, and commit with `feat: integrate PHP extraction with indexing`.

### Task 6: PHP fixture, evaluation cases, and documentation

**Files:**

- Create: `testdata/repos/phpsample/composer.json`
- Create: `testdata/repos/phpsample/src/Auth/TokenService.php`
- Create: `testdata/repos/phpsample/src/Auth/TokenValidator.php`
- Create: `testdata/repos/phpsample/src/Http/AuthMiddleware.php`
- Create: `testdata/repos/phpsample/includes/bootstrap.inc`
- Create: `testdata/repos/phpsample/tests/TokenServiceTest.php`
- Create: `testdata/repos/phpsample/templates/login.phtml`
- Create: `testdata/repos/phpsample/unrelated/Report.php`
- Create: `testdata/repos/phpsample/README.md`
- Create: `testdata/eval/php-cases.jsonl`
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/implementation-plan.md`
- Modify: `docs/evaluation.md`

**Interfaces:**

- Produces a fixture with namespace/use alias, expired-token branch, middleware caller, PHPUnit test, `.inc` include, mixed HTML/PHP, heredoc/comment braces, and an unrelated large PHP file.
- Produces eval cases with the existing JSONL schema and no production fixture-name branches.
- Documents PHP as first-class structural extraction and records actual measured metrics only after evaluation runs.

- [ ] **Step 1: Write fixture/eval acceptance tests**

Add a Go integration test under `internal/eval` or `internal/app` that opens `testdata/repos/phpsample`, runs `Index` and the four PHP queries, and asserts every returned item has an existing path and valid `1 <= StartLine <= EndLine` against the fixture. Assert `unrelated/Report.php` is absent from all four bundles and each expected symbol/path appears by rank five.

- [ ] **Step 2: Run fixture tests and verify RED**

Run `go test ./internal/eval ./internal/app -run 'TestPHPFixture' -v`.

Expected: fixture path/file or PHP eval case is missing.

- [ ] **Step 3: Add the fixture contents**

Use canonical application names `App\\Auth\\TokenService` and `App\\Http\\AuthMiddleware`. Include:

```php
if ($token->isExpired()) {
    throw new ExpiredTokenException('expired token');
}
```

Make middleware call `TokenService::validateToken`, make the test extend `PHPUnit\\Framework\\TestCase` with `testExpiredTokenIsRejected`, make `bootstrap.inc` contain a bootstrap function, and make `login.phtml` contain both HTML and a short echo tag. Put braces in a heredoc and comment in `TokenService.php`. Make `Report.php` large enough to test bounded retrieval but unrelated in names/content.

- [ ] **Step 4: Add PHP eval cases**

Create `testdata/eval/php-cases.jsonl` with:

```jsonl
{"name":"php-expired-token","query":"where is an expired PHP authentication token rejected?","token_budget":1200,"expected_symbols":["validateToken"],"expected_paths":["src/Auth/TokenService.php"],"forbidden_paths":["unrelated/Report.php"]}
{"name":"php-token-callers","query":"what calls TokenService validateToken?","token_budget":1200,"expected_symbols":["validateToken"],"expected_paths":["src/Http/AuthMiddleware.php"],"forbidden_paths":["unrelated/Report.php"]}
{"name":"php-expired-token-tests","query":"what tests cover expired PHP tokens?","token_budget":1200,"expected_symbols":["testExpiredTokenIsRejected"],"expected_paths":["tests/TokenServiceTest.php"],"forbidden_paths":["unrelated/Report.php"]}
{"name":"php-bootstrap-include","query":"which include file bootstraps authentication?","token_budget":1200,"expected_symbols":["bootstrap"],"expected_paths":["includes/bootstrap.inc"],"forbidden_paths":["unrelated/Report.php"]}
```

- [ ] **Step 5: Update documentation without erasing user changes**

Update the existing Japanese and English README sections to state: PHP is first-class structural extraction; `.inc` uses content-aware detection; namespace/use/class/function/method/include are extracted; complete type inference is not provided; dynamic dispatch and service-container resolution are unsupported; Composer and PHP runtime are never executed; mixed HTML/PHP is searchable; PHP source is stored in SQLite. In `docs/design.md`, replace PHP-as-generic-only wording with the package and relation design. In `docs/implementation-plan.md`, add a completed PHP workstream with tests and verification commands. In `docs/evaluation.md`, add PHP metrics and fixture reproduction commands while keeping Go metrics separate.

- [ ] **Step 6: Run fixture tests and eval, then record evidence**

Run:

```text
gofmt -w internal/eval internal/app
go test ./internal/eval ./internal/app -run 'TestPHPFixture' -v
go run ./cmd/focalspan index --root testdata/repos/phpsample --quiet
go run ./cmd/focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
```

Copy only measured hit@5, budget compliance, forbidden violations, determinism, and median reduction results into `docs/evaluation.md`. If a metric fails, fix extraction/ranking and keep the acceptance test failing until it passes.

- [ ] **Step 7: Check and commit**

Run `git diff --check`, stage only fixture/eval/docs files, and commit with `test: add PHP fixture evaluation`.

### Task 7: Full regression, cross-build, and final acceptance

**Files:**

- Test: all changed Go packages and existing repository tests
- Modify: `docs/evaluation.md` only if final measured verification differs from Task 6

**Interfaces:**

- Preserves all existing CLI/MCP entry points, schema version, Go fixture metrics, and deterministic output.
- Provides final evidence for PHP and Go evaluation, race/vet status, and four CGO-free targets.

- [ ] **Step 1: Run formatting and all unit/integration tests**

Run:

```text
gofmt -w .
go test ./...
```

Expected: all tests pass. If failures occur, return to the owning behavior, add a RED regression test, and then change code.

- [ ] **Step 2: Run race and vet checks**

Run:

```text
go test -race ./...
go vet ./...
```

Record the exact toolchain error and affected scope if Windows cannot run the race detector; do not report race success in that case.

- [ ] **Step 3: Run both fixture evaluations**

Run:

```text
go run ./cmd/focalspan index --root testdata/repos/authsample --quiet
go run ./cmd/focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
go run ./cmd/focalspan index --root testdata/repos/phpsample --quiet
go run ./cmd/focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
```

Confirm existing Go hit@5 `1.0`, budget compliance `1.0`, forbidden violations `0`, determinism `1.0`, and median reduction `<= 0.25`; confirm the same PHP thresholds. Confirm every item path exists and line range is valid.

- [ ] **Step 4: Run CGO-free builds**

Run:

```text
go build ./cmd/focalspan
CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan
```

Record each exit status. Do not add runtime PHP/Composer checks because the product must not execute them.

- [ ] **Step 5: Inspect final diff and repository state**

Run:

```text
git diff --check
git status --short
git diff --stat
git log -5 --oneline
```

Verify no migration file changed, no external dependency was added, `README.md` retains the pre-existing Japanese change, `.serena/` remains untouched, and no fixture-specific production condition exists.

- [ ] **Step 6: Report acceptance accurately**

Report detection rules, lexer/parser design, symbols/relations, recovery, changed files, unit/integration/eval outputs, PHP metrics, Go regression metrics, cross-build statuses, and known semantic limits. Mark any unexecuted or failed check as unverified rather than complete.
