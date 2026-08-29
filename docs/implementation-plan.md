# FocalSpan Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a CGO-free Go CLI and MCP stdio server that returns deterministic, budget-compliant code context from a repository.

**Architecture:** Keep repository scanning, extraction, storage, retrieval, ranking, packing, rendering, CLI, and MCP behind small package boundaries. Parse workers return immutable results to one SQLite writer; CLI and MCP call the same application service.

**Tech Stack:** Go 1.26+, `database/sql`, `modernc.org/sqlite v1.57.0` with real FTS5, `github.com/modelcontextprotocol/go-sdk v1.7.0`, `github.com/pelletier/go-toml/v2 v2.2.4` for validation-only parsing, standard-library AST/parser, embedded SQL migrations, `log/slog`.

**Spec:** `docs/design.md`

## Global Constraints

- The module must declare Go 1.26 or newer and pass Go 1.27 verification.
- `CGO_ENABLED=0` must build the binary for Windows amd64, Linux amd64, and macOS arm64.
- No network, external LLM, repository-code execution, build, test, package restore, Tree-sitter, SCIP, embeddings, HTTP MCP, or copied CRG code enters the product path.
- SQLite writes are single-writer transactional operations and FTS5 is real `MATCH` plus `bm25`.
- Internal paths are slash-normalized repository-relative paths; symlink escape and absolute MCP paths are rejected.
- Default secret-shaped files and files over 2 MiB are excluded; source content is stored in `.focalspan/index.db`.
- Every source item has a path and one-based line range; final rendered output must be within 256..64000 tokens.
- CLI normal output is stdout, errors/logs are stderr; MCP stdout is protocol-only.
- Every vertical slice begins with a failing test and ends with a targeted test plus `git diff --check`.

## File Map

- Create `go.mod`, `.gitignore`, `AGENTS.md`, and `cmd/focalspan/main.go` for the executable boundary.
- Create `internal/model`, `internal/config`, `internal/repository`, and `internal/gitx` for safe input and domain values.
- Create `internal/extract`, `internal/extract/goast`, `internal/extract/php`,
  `internal/extract/cpp`, `internal/extract/csharp`, `internal/extract/jsts`,
  `internal/extract/sourceutil`, and `internal/extract/generic` for extraction.
- Create `internal/store/migrations`, `internal/store`, and `internal/indexer` for SQLite and incremental indexing.
- Create `internal/search`, `internal/rank`, `internal/budget`, and `internal/render` for context selection.
- Create `internal/cli`, `internal/mcpserver`, and `internal/eval` for adapters and measurement.
- Create `testdata/repos/authsample`, `testdata/eval/cases.jsonl`, and focused package/integration tests.
- Keep `docs/design.md`, `docs/implementation-plan.md`, and `docs/evaluation.md` current with verified behavior.

### Task 1: Bootstrap module and dependency spike

**Files:**
- Create: `go.mod`, `.gitignore`, `cmd/focalspan/main.go`
- Create: `internal/store/sqlite_spike_test.go`
- Modify: `docs/design.md`

**Interfaces:**
- Produces module `github.com/focalspan/focalspan` and a `main` package that dispatches to `internal/cli.Run`.
- Produces a driver test that opens `:memory:`, creates FTS5, runs `MATCH`, and reads `bm25`.

- [ ] Write `sqlite_spike_test.go` first with `TestSQLiteFTS5Driver` that imports the driver, creates `chunk_fts`, inserts one row, asserts one `MATCH` row, and scans a `bm25` value.
- [ ] Run `GOMODCACHE=H:\sourcecode\FocalSpan\.gomodcache go test ./internal/store -run TestSQLiteFTS5Driver -v`; confirm the new test fails because the module and driver do not exist.
- [ ] Add `go.mod` with `go 1.26`, `modernc.org/sqlite v1.57.0`, and `github.com/modelcontextprotocol/go-sdk v1.7.0`; use the repository-local module cache for downloads.
- [ ] Add a minimal `main` that calls `cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)` and returns its error code.
- [ ] Implement the smallest SQLite spike setup and run the targeted test plus `CGO_ENABLED=0 go test ./internal/store -run TestSQLiteFTS5Driver -v`.
- [ ] Record the actual driver/SDK versions and FTS5 result in `docs/design.md` and commit the bootstrap slice.

### Task 2: Domain model, configuration, root safety, and scanner

**Files:**
- Create: `internal/model/model.go`, `internal/model/handles.go`
- Create: `internal/config/config.go`, `internal/config/config_test.go`
- Create: `internal/repository/root.go`, `internal/repository/scanner.go`, `internal/repository/scanner_test.go`

**Interfaces:**
- Produces `model.SourceFile`, `Symbol`, `Chunk`, `Relation`, `Diagnostic`, `ScoreReason`, `RankedCandidate`, `ContextBundle`, and `ContextItem`.
- Produces `config.Load(root, path, force) (Config, []string, error)` and `repository.Scanner.Scan(context.Context) ([]SourceFile, []Diagnostic, error)`.

- [ ] Write tests for default config, unknown-key warning, wrong type, root-relative paths, slash normalization, binary/NUL detection, invalid UTF-8, BOM/CRLF line counting, max size, secret exclusion, and symlink escape.
- [ ] Run `go test ./internal/config ./internal/repository`; confirm failures identify missing types/functions.
- [ ] Implement bounded config validation and a SHA-256 config hash; ensure explicit include patterns are required for secret-shaped paths.
- [ ] Implement Git-root detection fallback to current directory, canonical containment, safe walk, language/profile detection, and source file reading.
- [ ] Implement deterministic handle generation from normalized identity fields and collision extension helpers; test line-movement stability and collision lengthening.
- [ ] Run focused tests, `go test ./internal/config ./internal/repository ./internal/model`, and `git diff --check`.

### Task 3: Git listing and diff hunk parsing

**Files:**
- Create: `internal/gitx/git.go`, `internal/gitx/git_test.go`, `internal/gitx/diff.go`, `internal/gitx/diff_test.go`

**Interfaces:**
- Produces `GitClient.ListFiles(ctx)` and `GitClient.Diff(ctx, DiffRequest)` using separated `exec.CommandContext` arguments.
- Produces `ParseUnifiedZeroDiff([]byte) ([]ChangedFile, error)` with changed line ranges and file state.

- [ ] Write tests using a temporary Git repository for tracked/untracked non-ignored listing and an argument-observation test that rejects shell metacharacter execution.
- [ ] Write table tests for unstaged, staged, base..head, rename, delete, binary, empty, and CRLF zero-context hunks.
- [ ] Run the two focused test files and confirm the parser/listing tests initially fail.
- [ ] Implement Git availability/error classification, NUL record decoding, diff mode selection, and hunk range conversion without a shell.
- [ ] Run `go test ./internal/gitx -v` and `git diff --check`.

### Task 4: Extraction registry and Go AST extractor

**Files:**
- Create: `internal/extract/extract.go`, `internal/extract/registry.go`, `internal/extract/extract_test.go`
- Create: `internal/extract/goast/go.go`, `internal/extract/goast/go_test.go`

**Interfaces:**
- Produces `extract.Extractor` and `extract.Registry.Extract(context.Context, model.SourceFile)`.
- Produces Go symbols/chunks and lexical relations for package/import/function/method/type/const/var/calls/references/tests.

- [ ] Write fixture-free AST tests for functions, methods/receivers, doc comments, imports, unresolved calls, selector expressions, test/benchmark/example names, line/byte spans, and CRLF.
- [ ] Run `go test ./internal/extract/...`; confirm failures before implementation.
- [ ] Implement AST traversal with `go/parser.ParseComments`, `token.FileSet`, normalized signatures, stable handles, and conservative unresolved relation confidence.
- [ ] Implement registry selection by extension/language and extraction diagnostics for parse failures; do not invoke `go list`, type checking, or repository code.
- [ ] Run extractor tests twice and compare serialized results for determinism.

### Task 5: Generic extractors and fallback windows

**Files:**
- Create: `internal/extract/generic/lexer.go`, `internal/extract/generic/brace.go`, `internal/extract/generic/indent.go`, `internal/extract/generic/markdown.go`, `internal/extract/generic/window.go`, `internal/extract/generic/generic_test.go`

**Interfaces:**
- Produces generic `Extraction` for C-like, Python-like, Markdown, and unknown text profiles with confidence labels.

- [ ] Write tests for braces in strings/comments, top-level C-like declarations, Python class/def indentation, Markdown headings, 80/10 windows, 160-line maximum, and no mid-line cuts.
- [ ] Run `go test ./internal/extract/generic`; confirm failures.
- [ ] Implement the stateful lexer and balanced declaration scanner, indentation boundaries, heading parser, and boundary-aware fallback windows.
- [ ] Generate parent/subchunks for oversized declarations and stable content hashes; keep generic extraction approximate in diagnostics.
- [ ] Run generic tests and `go test ./internal/extract/...`.

### Task 6: SQLite migrations, store, and FTS synchronization

**Files:**
- Create: `internal/store/migrations/001_initial.sql`, `internal/store/store.go`, `internal/store/store_test.go`
- Create: `internal/store/fts_test.go`, `internal/store/status.go`

**Interfaces:**
- Produces `store.Open(root, indexDir)`, `Store.ReplaceFile`, `Store.DeleteFile`, `Store.SearchFTS`, `Store.GetStatus`, `Store.GetRevision`, and transactional index-run methods.
- Persists all required tables, source content, FTS5 rows, config metadata, and diagnostics.

- [ ] Write migration/status tests and FTS insert/update/delete tests; assert `MATCH`, `bm25`, foreign keys, WAL/busy timeout, and schema mismatch behavior.
- [ ] Run `go test ./internal/store`; confirm failure before schema/store code exists.
- [ ] Implement embedded migration transaction, SQLite pragmas, parameter-bound statements, explicit FTS synchronization, and read-only status queries.
- [ ] Implement corrupt-DB error wrapping with a rebuild instruction and revision/meta accessors.
- [ ] Run store tests under `CGO_ENABLED=0` and a repeated `go test ./internal/store` determinism check.

### Task 7: Incremental indexer and application service

**Files:**
- Create: `internal/indexer/indexer.go`, `internal/indexer/indexer_test.go`
- Create: `internal/app/service.go`, `internal/app/service_test.go`

**Interfaces:**
- Produces `Indexer.Index(context.Context, IndexRequest) (IndexResult, error)` and application operations for index/update/status/query/expand/impact.
- Uses scanner, extractor registry, store, and Git diff service without leaking CLI/MCP concerns.

- [ ] Write tests for first full index, unchanged SHA-256 skip, one-file reparse, stale deletion, parse failure isolation, worker cap, cancellation rollback, and run stats.
- [ ] Run `go test ./internal/indexer ./internal/app`; confirm failures.
- [ ] Implement bounded workers capped by `min(GOMAXPROCS, 8)`, result channel, single writer transaction, SHA-256 comparison, stale cleanup, FTS consistency check, and successful revision publication.
- [ ] Implement auto-index-on-query and `--no-update` service behavior plus root-bound path filters.
- [ ] Run focused tests, `go test ./internal/indexer ./internal/app`, and `git diff --check`.

### Task 8: Query normalization, retrieval, ranking, and relation expansion

**Files:**
- Create: `internal/search/query.go`, `internal/search/search.go`, `internal/search/search_test.go`
- Create: `internal/rank/rank.go`, `internal/rank/rank_test.go`

**Interfaces:**
- Produces `NormalizeQuery(string) QueryTerms`, `Search.Search(context.Context, SearchRequest) ([]model.RankedCandidate, error)`, and `Ranker.Rank`.
- Supports one-hop relation kinds and changed-line boosts without narrowing normal search to changed files.

- [ ] Write tests for quoted/special FTS syntax, snake/camel/Pascal/namespace/receiver tokens, exact symbols, BM25 order, relation candidates, changed-line overlap, score reasons, tie breaks, and deduplication.
- [ ] Run `go test ./internal/search ./internal/rank`; confirm failures.
- [ ] Implement escaped FTS query construction, staged retrieval caps, named weights, stable sorting, relation expansion, path filtering, and overlap/content-hash deduplication.
- [ ] Verify the same indexed fixture query serializes identically across repeated runs.
- [ ] Run focused tests and `go test ./internal/search ./internal/rank`.

### Task 9: Token estimator, packer, and renderers

**Files:**
- Create: `internal/budget/estimate.go`, `internal/budget/pack.go`, `internal/budget/budget_test.go`
- Create: `internal/render/compact.go`, `internal/render/json.go`, `internal/render/render_test.go`

**Interfaces:**
- Produces `budget.TokenEstimator`, `budget.Packer.Pack(PackRequest) ContextBundle`, and final rendered human/JSON payload functions.
- Packer guarantees final estimated output is at or below the clamped budget, with outline fallback and explicit elision.

- [ ] Write tests for ASCII/non-ASCII/identifier/punctuation/header estimates, large-symbol elision, 40% item cap, exact-symbol exception, UTF-8/line-safe cuts, final render budget, file diversity, and low-utility omission.
- [ ] Run `go test ./internal/budget ./internal/render`; confirm failures.
- [ ] Implement deterministic estimator with 12% margin, query-window elision, source/outline modes, compact reasons, and JSON diagnostics.
- [ ] Implement render-estimate-drop-re-render loop and assert no source body is duplicated in structured plus text MCP representations.
- [ ] Run focused tests and `go test ./internal/budget ./internal/render`.

### Task 10: CLI commands and doctor

**Files:**
- Create: `internal/cli/run.go`, `internal/cli/flags.go`, `internal/cli/commands.go`, `internal/cli/cli_test.go`
- Modify: `cmd/focalspan/main.go`, `.gitignore`

**Interfaces:**
- Produces `focalspan init`, `index`, `update`, `status`, `query`, `expand`, `impact`, `eval`, `doctor`, and `serve` dispatch with documented flags.
- Uses stdout for compact/JSON command output and stderr for errors; `update --if-repo --quiet` is a zero-output no-op outside Git.

- [ ] Write command-boundary tests for init safety/force, no-update, JSON status/query, budget/mode validation, quiet if-repo, path rejection, and doctor checks.
- [ ] Run CLI tests and the built binary against a temporary root; confirm failures.
- [ ] Implement standard-library flag parsing, root/config resolution, command exit codes, init `.focalspan.json` and `.gitignore` handling, and doctor checks including FTS5/MCP initialization.
- [ ] Run `go test ./internal/cli`, build `go build ./cmd/focalspan`, and execute the fixture command sequence.

### Task 11: MCP server and integration tests

**Files:**
- Create: `internal/mcpserver/server.go`, `internal/mcpserver/tools.go`, `internal/mcpserver/mcp_test.go`
- Modify: `internal/cli/commands.go`, `README.md`

**Interfaces:**
- Produces official SDK stdio server with `code_context`, `code_expand`,
  `code_impact`, `code_restart`, and `code_status`; `code_restart` reloads the
  service in-process without terminating the MCP transport.
- Typed inputs are `CodeContextInput`, `CodeExpandInput`, and `CodeImpactInput`; invalid/blank inputs return typed validation errors without stack traces.

- [ ] Write in-memory/client integration tests for server initialization, tools list, status, context, validation error, cancellation, concurrent calls, and stdout cleanliness.
- [ ] Run `go test ./internal/mcpserver`; confirm failures.
- [ ] Implement startup-root binding, auto-update policy, structured output with one canonical source copy, stderr slog handler, signal cancellation, and root/path validation.
- [ ] Run MCP tests and a subprocess smoke test that proves stdout contains only protocol messages.

### Task 12: Fixture, evaluation, README, acceptance, and cross-build

**Files:**
- Create: `testdata/repos/authsample/auth/service.go`, `auth/service_test.go`, `http/middleware.go`, `config/config.go`, `unrelated/report.go`, `README.md`
- Create: `testdata/eval/cases.jsonl`, `internal/eval/eval.go`, `internal/eval/eval_test.go`
- Create: `internal/extract/cpp`, `internal/extract/csharp`,
  `internal/extract/jsts`, `internal/extract/sourceutil`,
  `testdata/repos/cppsample`, `testdata/repos/csharpsample`,
  `testdata/repos/jstssample`, and their matching evaluation JSONL files.
- Modify: `docs/evaluation.md`, `README.md`, `AGENTS.md`

**Interfaces:**
- Produces JSON/JSONL evaluation with hit@1/3/5, symbol/path recall, forbidden violations, budget compliance, median estimate, reduction ratio, and repeated-run determinism. The same evaluator covers Go, PHP, C/C++, C#, and JavaScript/TypeScript fixtures.
- Documents all CLI/MCP/config/security/platform limitations and the exact acceptance workflow.

- [ ] Write fixture/eval tests first and assert `ValidateToken` top 5, related test top 5, no forbidden report path, 100% budget compliance, and deterministic serialization.
- [ ] Run `go test ./internal/eval ./...`; confirm any missing integration is visible.
- [ ] Add natural fixture code and implement evaluation baseline as estimated tokens for candidate files returned in the query, not a hard-coded symbol special case.
- [ ] Run `gofmt` on every Go file, `go test ./...`, `go test -race ./...`, and `go vet ./...`.
- [ ] Run `CGO_ENABLED=0 go build ./cmd/focalspan`, plus Windows amd64, Linux amd64, and Darwin arm64 cross-builds using explicit output paths under a temporary build directory.
- [ ] Run the fixture sequence: `index`, `status --json`, budgeted JSON `query`, `update`, `impact --json`, `doctor`, and bounded `serve` smoke test.
- [ ] Record measured metrics and any environment-blocked verification in `docs/evaluation.md` and README; run `git diff --check`.

### Task 13: Codex MCP registration integration

**Files:**
- Create: `internal/integration/codex/*.go`, focused project/user adapter tests
- Modify: `internal/cli/run.go`, `README.md`, `docs/design.md`

**Interfaces:**
- Produces `focalspan install|uninstall` global MCP shortcuts and
  `focalspan mcp install|status|uninstall|print [codex]` with project
  scope as the default and user scope delegated to the official Codex CLI.
- Project scope uses a validated, atomic, marked TOML block and preserves all
  unmanaged content; user scope uses an injectable no-shell command runner.

- [x] Resolve FocalSpan executables canonically and reject temporary `go-build`
  binaries for permanent registration.
- [x] Add idempotent managed-block install/uninstall, collision detection,
  read-only status, dry-run output, and separated JSON command/args.
- [x] Add deterministic tests for path escaping, project preservation, user
  argv ordering, force/remove safety, cancellation, and CLI dispatch.
- [ ] Run the full test, vet, race, cross-build, and temporary-repository
  acceptance checklist before marking the integration complete.

### Task 14: Opaque double-curly template tags

**Files:**
- Modify: `internal/repository/scanner.go`, `internal/extract/template/`
- Modify: `README.md`, `docs/design.md`

**Interfaces:**
- Treat `{{...}}` as an opaque template tag so `.tpl` files using a
  non-Smarty double-curly syntax remain `template` files unless they also
  contain an actual Smarty marker.
- Preserve double-curly tags in searchable source and never create Smarty
  structural symbols from their contents; quoted `}}` is not a boundary.

- [x] Add scanner, language-detection, and extraction regression tests.
- [x] Keep malformed double-curly input bounded and report a diagnostic.
- [ ] Run the full test, vet, race, cross-build, and fixture acceptance
  checklist before marking the integration complete.

## Retrieval Quality v0.2 completion

The v0.2 retrieval work is implemented in the current checkout. A single
deterministic planner in `internal/query` normalizes Unicode and mixed
Japanese/code queries, preserves identifiers, selects intent and relation
profiles, and feeds independent structural, FTS, path, and relation
retrievers. Weighted reciprocal-rank fusion combines their ranked lists before
the existing intent-aware ranking, deduplication, and token packer. Evaluation
2.0 measures intent, relation, kind, hit@N, budget, forbidden paths, reduction,
and repeated-run determinism, with `full`, `fts-only`, and `no-relations`
ablations. `focalspan explain` is a CLI-only, source-free retrieval trace.

The v0.2 implementation does not add translation, embeddings, compiler-grade
cross-file resolution, a schema migration, or an MCP debug tool. The current
checkout retains its existing `code_restart` MCP extension, so its verified
MCP surface is five tools rather than the four-tool surface described by the
original MVP plan.

Measured acceptance results and environment-specific verification status are
recorded in `docs/evaluation.md`. The next-stage roadmap remains design-only:
semantic zoom and evidence spans (v0.3), repository linking (v0.4), and
optional semantic facts (v0.5).

## Verification checklist

### Completed PHP structural extraction workstream

- [x] Detect `.php`, `.phtml`, `.php3` through `.php8`, `.phps`, and
  content-aware PHP `.inc` files without executing PHP or Composer.
- [x] Add stateful PHP lexing, namespace-aware declarations/members, stable
  symbol and span-aware chunk handles, mixed HTML/PHP fallback chunks, and
  bounded malformed-source recovery.
- [x] Add `contains`, `imports`, `references`, `calls`, and `tests` relations,
  local alias resolution, safe include folding, PHPUnit classification, and
  unresolved store matching without a schema migration.
- [x] Register PHP between Go AST and generic extraction; add fixture/eval
  coverage for PHP, `.inc`, `.phtml`, relations, and deterministic packing.
- [x] Verify with `go test ./internal/extract/php ./internal/extract`,
  `go test ./internal/extract ./internal/store ./internal/app`, and the PHP
  fixture evaluation under `testdata/repos/phpsample`.

### Completed C/C++, C#, and JavaScript/TypeScript structural workstream

- [x] Detect C and common C/C++ header/module extensions, C# `.cs`, and
  JavaScript/TypeScript `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, and `.cjs` files.
- [x] Add pure-Go stateful lexers and bounded parsers with exact UTF-8-safe
  byte/line spans, namespace/module/type hierarchy, stable handles, and
  malformed-source diagnostics and prefix recovery.
- [x] Add method/function-oriented chunks, container outlines, include/import/
  export relations, caller/callee candidates, type/reference candidates, and
  test relations without class/body duplication.
- [x] Add unresolved relation matching and relation-aware search anchors while
  retaining confidence labels instead of claiming compiler-grade resolution.
- [x] Add C/C++, C#, and JavaScript/TypeScript fixtures and evaluations covering
  C, C++ headers/includes, C# interfaces/partial classes, ESM/CommonJS,
  JSX/TSX, tests, forbidden paths, budget compliance, and determinism.
- [x] Verify focused extractor/ranking/search/packing tests and the three
  fixture evaluations; measured hit@5 is 1.00 for all three profiles and
  median reduction is 0.1667, 0.1215, and 0.2020 respectively.

- [x] `go test ./...`
- [ ] `go test -race ./...` (環境未検証: PATH上に`gcc`がなく、`C:\cygwin64\bin\gcc.exe`もnative Windows向けにはGoが拒否)
- [x] `go vet ./...`
- [x] `CGO_ENABLED=0 go build ./cmd/focalspan`
- [x] `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan`
- [x] `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan`
- [x] `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan`
- [x] fixture evaluation meets the acceptance thresholds; PHP and Go results are recorded in `docs/evaluation.md`.
