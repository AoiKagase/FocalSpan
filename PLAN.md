# FocalSpan Path-Scoped Symbol Retrieval v0.7 Implementation Plan

> **For agentic workers:** Execute this plan task-by-task. Every behavior change
> follows RED, explicit RED confirmation, minimal GREEN, focused verification,
> `go test ./... -count=1`, `go vet ./...`, `git diff --check`, and an
> explicit-path commit. Keep this file's Progress, discoveries, decisions, and
> outcomes current while working.

**Goal:** Add one bounded hierarchical retriever that turns likely repository
files into symbol-bearing candidates, so semantically relevant identities such
as `Run`, `Search`, and `codeContext` reach the ranked candidate set and
Evidence Packet without broadening the ordinary path retriever, changing the
query language, or tuning ranking and packing simultaneously.

**Architecture:** Existing qualified, exact-symbol, prefix, FTS, explicit-path,
and relation retrieval remain intact. A new `path-scoped-symbol` retriever
derives at most eight likely file paths from existing FTS/path results plus a
bounded lexical path probe, then asks the store for symbol-associated chunks
within only those files using exact/name-variant and FTS evidence. Its output is
a separately traced RRF list; lexical path probes never emit generic chunks and
never become relation anchors by themselves.

**Tech Stack:** Go 1.27 as currently verified; standard library; existing
`internal/{query,search,store,rank,app,evidence,benchmark,benchcli}` packages;
SQLite/FTS5 through the current store; existing historical benchmark and
source-free attribution trace; local Git CLI through the existing benchmark
runner.

**Spec:** This root plan is the sole active ExecPlan. It is governed by
`PLANS.md`, preserves the completed v0.6 evidence in
`docs/benchmarks/findings-v0.6.md` and
`docs/benchmarks/attribution-v0.6.{json,md}`, and does not alter the
`focalspan.context.v1` wire contract.

**Plan ID:** `v0.7-path-scoped-symbol-retrieval`

**Status:** Active after Task 0 archives the completed v0.6 plan.

**Expected remote baseline:** `987c5d2` (`docs: record v0.6 closure CI`). The
executor must record the actual local `git rev-parse HEAD` before changing
files. A local branch ahead of this commit or a dirty worktree is preserved and
recorded; it is not reset to this expected value.

---

## Global Constraints

- Treat the current checkout as the only source of truth. Preserve all merged
  extractors, metadata resolvers, linker behavior, Query Planner, RRF fusion,
  Evidence Packet behavior, benchmark attribution, Codex MCP registration, and
  CI.
- Do not use `git reset`, `git restore`, `git checkout --`, `git clean`, or
  `git stash`. Do not overwrite or discard pre-existing changes.
- Archive the completed v0.6 `PLAN.md` byte-for-byte before replacing its
  active-plan role. The archive filename is
  `docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md`.
- This milestone has exactly one production hypothesis:
  **bounded file discovery followed by symbol-level retrieval inside those
  files**. Do not add a second production optimization if this hypothesis fails.
- Do not change query normalization, intent lexicons, Query Planner output,
  global exact/prefix behavior, FTS query construction, existing path-search
  semantics, relation SQL, linker behavior, rank profile weights, Evidence
  utility, packer rules, token accounting, wire fields, MCP tool names, or
  server-wide MCP instructions.
- Do not reintroduce v0.6's rejected behavior of feeding all natural-language
  words directly into `SearchPaths` and emitting those generic path chunks.
- Do not make fixture/corpus-specific aliases, special cases for `Run`,
  `codeContext`, `Search`, `indexer.go`, `server.go`, or benchmark case IDs.
- `RetrievalFTSOnly` remains byte-for-byte behaviorally unchanged. It must not
  call file probing or the new retriever.
- The new retriever runs only for `RetrievalFull` and
  `RetrievalNoRelations`.
- Lexical file probing is allowed only when the query plan has no relation
  intents. Relation queries may use paths already present in explicit path and
  FTS results, but may not launch a broad lexical file probe.
- File probing returns repository-relative paths only. It must never return
  source content, chunks, handles, or synthetic candidates.
- Path-scoped symbol retrieval may inspect at most 8 distinct paths, return at
  most 8 candidates per path, and return at most 40 candidates total.
- Query-derived path-probe hints are capped at 8; path-scoped symbol hints are
  capped at 16. All lists are stable and case-insensitively deduplicated.
- The store must use parameter binding. Dynamic `IN` placeholder construction
  may create placeholders only; values never enter SQL text.
- Symbol-scoped FTS requires `c.symbol_handle IS NOT NULL`. Generic windows and
  unowned chunks do not enter the new list.
- Existing explicit path candidates remain in the original `path` retriever.
  The new retriever has the separate ID `path-scoped-symbol`.
- Freeze the new retriever's RRF weight at `1.35` before the first historical
  candidate run. Do not tune it after seeing benchmark output in this
  milestone.
- Keep ordinary CLI, MCP, and Evidence output free of traces, candidate pools,
  scores, and the new internal file scopes.
- Attribution remains source-free and path-relative. Add the new retriever ID
  to sanitized development output, but never expose query source, candidate
  content, absolute paths, environment values, or secrets.
- Do not add network access, external LLM calls, embeddings, Tree-sitter,
  compiler/LSP processes, repository build/test execution, or package-manager
  execution.
- Do not change SQLite schema version. This milestone adds read queries only.
- Do not increase existing global FTS, fused, relation, path, exact, qualified,
  or prefix limits.
- Every production behavior starts with a failing automated test that
  reproduces the intended boundary. Confirm the failure before implementation.
- Run the selected four-case baseline once, the selected four-case candidate
  once, and the full eight-case repeat-3 run at most once after the frozen gate
  passes. Infrastructure failures before a valid report may be retried once
  only after recording the cause in the Decision Log.
- If the frozen candidate gate fails, revert the v0.7 production change,
  retain the negative findings and plan history, run closure verification, and
  finish v0.7 as a negative milestone. Do not try a second retrieval, ranking,
  or packing adjustment.
- Preserve CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds, Linux
  race CI, every checked-in fixture evaluation, Evidence invariants, and the
  v0.5 benchmark comparison.
- Do not call configured but unrun GitHub Actions jobs successful. Record an
  actual run URL and conclusion before claiming remote verification.

---

## Purpose / Big Picture

v0.6 established a source-free pre-packet attribution pipeline and measured
where 95 human-labeled historical expectations terminated. The measured
distribution was:

    retrieval_missing   55
    packing_dropped     35
    packed               5
    linking_unresolved   0
    ranking_dropped      0
    label_not_indexed    0

The selected v0.6 experiment broadened the existing path retriever with lexical
words. It proved that a word such as `index` could expose chunks from
`internal/indexer/indexer.go`, reducing twelve selected retrieval misses to
eight. It still did not retrieve the `Run` identity, did not make an expansion
anchor executable, and did not improve final packet recall. The broad path
chunks also threatened relation-anchor quality until the experiment was
restricted, and the candidate was ultimately reverted.

That result distinguishes two operations that the current search pipeline
conflates:

1. Discovering a likely file.
2. Selecting the relevant symbol-bearing span inside that file.

`SearchPaths` performs the first operation only indirectly: it returns ordinary
chunks from every matching path, ordered by outline penalty, confidence, span
size, and source order. Its bounded results can fill with other chunks before
the relevant function appears. Feeding those chunks directly into fusion also
adds file-level noise.

v0.7 tests a narrower hierarchy:

    query plan
      -> existing qualified/exact/prefix/FTS/explicit-path lists
      -> bounded file-scope discovery
      -> exact/name-variant + FTS search among symbols in those files
      -> path-scoped-symbol ranked list
      -> unchanged RRF, ranking, and Evidence compiler

The new list should give FocalSpan a way to say:

> This file is plausible; now return the few functions, methods, types, or
> declarations in that file that best match the question.

The milestone succeeds only if symbol identities reach actual Evidence packets
and unblock at least one expansion. Merely moving a required path or symbol
from `retrieval_missing` to `packing_dropped` is not sufficient by itself.

---

## Context and Orientation

Relevant current boundaries:

- `internal/query/model.go` defines `Terms` and `Plan`.
- `internal/query/normalize.go` preserves words, identifiers, symbols, paths,
  phrases, and Unicode runs.
- `internal/query/fts.go` creates a safe quoted OR expression for FTS5.
- `internal/search/search.go` defines `CandidateStore`, `SearchRequest`, and
  `Searcher`.
- `internal/search/retrieval.go` calls qualified, exact, prefix, FTS, explicit
  path, and relation retrieval.
- `internal/search/fusion.go` performs weighted reciprocal-rank fusion.
- `internal/search/trace.go` defines source-free retriever and candidate trace
  types.
- `internal/store/store.go` implements symbol, FTS, path, and relation queries
  against SQLite.
- `internal/rank/rank.go` applies intent-aware scoring after fusion. It is
  intentionally unchanged in v0.7.
- `internal/evidence` selects and serializes packet evidence. It is
  intentionally unchanged in v0.7.
- `internal/benchmark/attribution.go` classifies exact labels as not indexed,
  not retrieved, unresolved, dropped by ranking, dropped by packing, or packed.
- `cmd/focalspan-bench` can select cases with repeatable `--case` flags and emit
  source-free attribution alongside quality reports.
- `testdata/benchmark/focalspan-history.json` contains eight historical cases.
- `docs/benchmarks/results-v0.5.json` is the current accepted quality baseline.
- `docs/benchmarks/attribution-v0.6.json` is the current accepted attribution
  baseline.

Current retriever IDs are:

    qualified-symbol
    symbol-exact
    symbol-prefix
    fts
    path
    relation

v0.7 adds:

    path-scoped-symbol

Current RRF weights are:

    qualified-symbol     2.00
    symbol-exact         1.80
    relation             1.60
    symbol-prefix        1.20
    fts                  1.00
    path                 0.90

v0.7 adds this fixed weight:

    path-scoped-symbol   1.35

This value places a file-constrained lexical symbol signal above global prefix
and FTS, but below an exact global symbol or relation fact. Do not modify any
other weight.

---

## Hypothesis and Selected Historical Scope

The hypothesis is:

> When existing evidence or a bounded path probe identifies a likely file,
> searching exact/name-variant and lexical matches only among symbol-owned
> chunks in that file will surface the intended symbol earlier than global FTS
> or generic path retrieval, without flooding relation anchors.

Freeze these four cases before production code changes:

1. `php-extractor-integration`
   - path: `internal/indexer/indexer.go`
   - symbol: `Run`
   - expansion anchor: `Run`, relation `callers`
   - path-probe clue: `index`
2. `project-metadata-indexing`
   - path: `internal/indexer/indexer.go`
   - symbol: `Run`
   - existing FTS already reaches the file and `Run` at a low position
3. `jsts-search-integration`
   - path: `internal/search/search.go`
   - symbol: `Search`
   - expansion anchor: `Search`, relation `callers`
   - natural word `search` is safe only inside a bounded file scope
4. `mcp-evidence-output`
   - path: `internal/mcpserver/server.go`
   - symbol: `codeContext`
   - expansion anchor: `codeContext`, relation `references`
   - identifier `code_context` requires naming-style variants

Selected profiles and budgets:

    full-evidence-focused / 1024
    full-evidence-focused / 2048
    full-evidence-focused / 4096
    no-relations-evidence-focused / 2048

`fts-evidence-focused` is excluded from the candidate gate because the new
retriever must not execute in FTS-only mode.

The selected subset contains 44 label rows:

    php-extractor-integration       3 labels x 4 rows = 12
    project-metadata-indexing       2 labels x 4 rows =  8
    jsts-search-integration         3 labels x 4 rows = 12
    mcp-evidence-output             3 labels x 4 rows = 12

Task 1 records their exact starting stages and positions from a fresh current
checkout run before implementation.

---

## Frozen Candidate Gate

The candidate proceeds to the full eight-case run only when every condition
below passes in the one selected four-case repeat-1 candidate run.

### Hard invariants

- The candidate report is compatible with `docs/benchmarks/results-v0.5.json`
  and reports zero quality regressions for the selected four cases.
- Budget compliance is `1.0`.
- Deterministic output is `1.0`.
- Relation validity is `1.0` wherever relations exist.
- Forbidden violations are `0`.
- Known-handle resend count is `0`.
- No selected label moves to an earlier terminal stage.
- No selected label becomes `label_not_indexed`.
- FTS-only fixture outputs and retriever-call tests remain unchanged.
- Existing Japanese relation-bearing recall remains `1.0` in full mode.
- Source-free attribution privacy and finite-number checks pass.

### Symbol-identity improvements

- Among the 16 v0.6 `retrieval_missing` symbol/anchor rows for
  `php-extractor-integration` and `mcp-evidence-output`, at least 8 advance to
  `ranking_dropped`, `packing_dropped`, or `packed`.
- At least one symbol or anchor row advances beyond retrieval in each of those
  two cases.
- `mcp-evidence-output` / `full-evidence-focused` / budget `2048` must pack
  `internal/mcpserver/server.go::codeContext`.
- `project-metadata-indexing` / `full-evidence-focused` / budget `2048` must
  pack `internal/indexer/indexer.go::Run`, and its ranked position must improve
  from the v0.6 baseline position `20` to `10` or better.
- `php-extractor-integration` / `full-evidence-focused` / budget `2048` must
  retrieve `Run` beyond `retrieval_missing`.
- At least one previously blocked selected expansion anchor must be packed and
  the benchmark must execute its real `code_expand` expectation successfully.
- Required-symbol recall at budget `2048` must improve in at least two of the
  four selected cases.

### Boundedness

- The new retriever returns no more than 40 candidates.
- No path contributes more than 8 candidates.
- No query scopes more than 8 paths.
- The new list contains only candidates with a non-empty symbol handle.
- Lexical file probing never appears as a retriever list and never adds generic
  chunks.
- The normal search result remains deterministic across two calls.
- No corpus-specific string appears in production code.

If any hard invariant or symbol-identity gate fails, Task 7 follows the negative
branch: revert the production candidate, retain source-free evidence, close the
milestone, and skip the full run.

---

## New Interfaces

Extend `internal/search.CandidateStore` with:

```go
SearchFilePaths(
    ctx context.Context,
    hints []string,
    limit int,
) ([]string, error)

SearchSymbolsInPaths(
    ctx context.Context,
    paths []string,
    symbolHints []string,
    ftsQuery string,
    perPathLimit int,
    limit int,
) ([]model.RankedCandidate, error)
```

Add these constants in `internal/search`:

```go
const (
    pathScopeHintLimit    = 8
    pathScopeFileLimit    = 8
    pathScopeSymbolLimit  = 40
    pathScopePerFileLimit = 8
)
```

Add the retriever ID:

```go
const RetrieverPathScopedSymbol RetrieverID = "path-scoped-symbol"
```

Add the fixed RRF weight:

```go
RetrieverPathScopedSymbol: 1.35,
```

Internal helper contracts:

```go
func pathScopeHints(plan query.Plan) []string
func pathScopedSymbolHints(plan query.Plan) []string
func collectScopedPaths(
    plan query.Plan,
    req SearchRequest,
    lists []RankedList,
    probed []string,
) []string
func identifierStyleVariants(value string) []string
```

These helpers remain unexported unless current package tests require an
exported test adapter.

---

## Path-Scope Rules

### File-scope sources, in stable priority order

1. Request path filters from `SearchRequest.Paths`, resolved through
   `SearchFilePaths`.
2. Paths already returned by the ordinary explicit `path` retriever.
3. Distinct paths from the existing FTS list, preserving FTS rank.
4. Lexical file-probe paths, only when `len(plan.Relations) == 0`.

Stop after 8 distinct paths.

Qualified/exact/prefix candidates do not seed a file scope solely because they
already identify a symbol directly; they remain in their original lists.

### Lexical file-probe hints

Build at most 8 case-insensitively unique hints from:

1. explicit query paths;
2. identifiers and symbols;
3. anchors;
4. natural-language words of at least 3 runes.

Normalize backslashes to slashes and trim surrounding punctuation. Do not stem
or use fuzzy edit distance. Substring path matching is the store's existing
bounded behavior.

Exclude these English intent/navigation words:

    where
    what
    which
    who
    why
    how
    is
    are
    was
    were
    does
    do
    did
    before
    after
    adding
    support
    supports
    caller
    callers
    callee
    callees
    test
    tests
    reference
    references
    import
    imports
    export
    exports

Exclude these Japanese navigation fragments when they appear as standalone
normalized terms:

    どこ
    場所
    処理
    流れ
    呼び出し元
    呼び出し先
    テスト
    参照
    定義

This list is a path-probe guard only. It does not modify `query.Normalize`,
`query.Plan`, FTS terms, or ranking.

### Symbol hints

Build at most 16 case-insensitively unique hints from:

1. plan anchors;
2. explicit symbols;
3. identifiers;
4. natural-language words of at least 3 runes that are not intent/navigation
   words.

For each code-shaped value, add stable naming variants:

- original value;
- final segment after `/`, `\`, `.`, `:`, or `::`;
- separator-free lower camel case;
- separator-free Pascal case;
- original snake-case form.

Examples:

    code_context
      -> code_context
      -> codeContext
      -> CodeContext

    Service.ValidateToken
      -> Service.ValidateToken
      -> ValidateToken

    search
      -> search

Matching in SQLite remains case-insensitive, so `search` can match `Search`
inside an already bounded file without becoming a global natural-word symbol
query.

Do not generate plural/singular, synonym, edit-distance, abbreviation, or
language-specific aliases.

---

## Store Query Semantics

### `SearchFilePaths`

- Query the `files` table only.
- Apply `lookupValues`-equivalent trimming and case-insensitive deduplication.
- Normalize `\` to `/`.
- For each hint, order matches by:
  1. exact path;
  2. exact final segment;
  3. final segment prefix;
  4. path-segment prefix;
  5. substring;
  6. shorter path;
  7. lexical path.
- Deduplicate paths across hints.
- Stop at the requested limit, capped at 16 by the store.
- Return repository-relative slash-normalized paths.
- Return an empty slice for empty hints.
- Never read or return source content.

### `SearchSymbolsInPaths`

- Normalize and deduplicate paths and symbol hints.
- Reject no value as an error; empty paths returns an empty slice.
- Cap paths at 8, per-path limit at 8, and total limit at 40.
- Use `rankedCandidateProjection`.
- Require:

      c.symbol_handle IS NOT NULL

- Run symbol passes in this strength order:
  1. case-sensitive qualified-name exact;
  2. case-insensitive qualified-name exact;
  3. case-insensitive symbol-name exact;
  4. symbol or qualified-name prefix.
- Run a path-constrained FTS pass when `ftsQuery` is non-empty.
- The FTS pass joins `chunk_fts`, `chunks`, `files`, and `symbols`, filters to
  the exact scoped paths and non-null symbol handles, then orders by:
  1. `bm25(chunk_fts)`;
  2. non-outline before outline/test-suite;
  3. symbol confidence descending;
  4. shorter span;
  5. path;
  6. start line;
  7. handle.
- Merge all passes in strength order, deduplicate by candidate identity, and
  enforce per-path fairness in Go.
- Return source-bearing `RankedCandidate` values exactly as existing store
  retrieval does.
- Do not write a new table, index, migration, or cached scope.

---

## Plan of Work

### Task 0: Archive v0.6 and Start v0.7

**Files:**
- Create:
  `docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md`
- Modify: `docs/superpowers/plans/README.md`
- Replace: `PLAN.md`
- Create: `docs/benchmarks/findings-v0.7.md`

**Consumes:** completed root v0.6 plan and plan lifecycle policy.

**Produces:** immutable v0.6 archive and one active v0.7 plan.

- [x] Record the exact initial state:

      git status --short
      git diff --stat
      git rev-parse HEAD
      git branch --show-current
      git log -8 --oneline
      git rev-list --left-right --count origin/master...HEAD
      go version
      go env GOOS GOARCH CGO_ENABLED

  Add the actual HEAD and worktree status to this plan's Decision Log. Preserve
  untracked `.focalspan.json` or other local files.

- [x] Verify the previous root `PLAN.md` started with:

      # FocalSpan Candidate Attribution and Coverage v0.6 Implementation Plan

  and has its completed task checks and final outcomes.

- [x] Copy the exact previous root bytes to:

      docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md

  Compare:

      git hash-object PLAN.md
      git hash-object docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md

  The object IDs must match before replacing root `PLAN.md`.

- [x] Add the v0.6 archive to the Completed section of
  `docs/superpowers/plans/README.md`. Do not edit older archives.

- [x] Replace root `PLAN.md` with this v0.7 plan and create
  `docs/benchmarks/findings-v0.7.md` containing:
  - title;
  - starting commit;
  - v0.6 measured distribution;
  - selected four cases;
  - the frozen candidate gate;
  - an empty chronological Results section that is updated by later tasks.

- [x] Run:

      git diff --check
      git grep -n "v0.6-candidate-attribution-and-coverage"

  Confirm root `PLAN.md` is the only active plan and all archive links resolve.

- [x] Commit only the plan transition:

      git add PLAN.md \
        docs/superpowers/plans/README.md \
        docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md \
        docs/benchmarks/findings-v0.7.md
      git commit -m "docs: start path-scoped symbol retrieval v0.7"

---

### Task 1: Freeze the Current Four-Case Baseline

**Files:**
- Modify: `PLAN.md`
- Modify: `docs/benchmarks/findings-v0.7.md`
- Temporary only:
  `.focalspan-bench/v0.7-baseline.{json,md,attribution.json,attribution.md}`

**Consumes:** v0.5 quality baseline and v0.6 attribution implementation.

**Produces:** exact current selected-row stages and positions before production
changes.

- [x] Run static baseline verification:

      git diff --check
      go test ./... -count=1
      go vet ./...

  Record actual test/package counts. Do not copy v0.6 counts.

- [x] Validate only the selected cases:

      go run ./cmd/focalspan-bench validate \
        --suite testdata/benchmark/focalspan-history.json \
        --case php-extractor-integration \
        --case project-metadata-indexing \
        --case jsts-search-integration \
        --case mcp-evidence-output

  Expected: 4 cases, 0 invalid.

- [x] Run the selected current baseline exactly once:

      go run ./cmd/focalspan-bench run \
        --suite testdata/benchmark/focalspan-history.json \
        --case php-extractor-integration \
        --case project-metadata-indexing \
        --case jsts-search-integration \
        --case mcp-evidence-output \
        --profile default \
        --repeat 1 \
        --json-out .focalspan-bench/v0.7-baseline.json \
        --markdown-out .focalspan-bench/v0.7-baseline.md \
        --attribution-json-out .focalspan-bench/v0.7-baseline-attribution.json \
        --attribution-markdown-out .focalspan-bench/v0.7-baseline-attribution.md \
        --force

- [x] Compare the quality report with the same rows in v0.5:

      go run ./cmd/focalspan-bench compare \
        --baseline docs/benchmarks/results-v0.5.json \
        --candidate .focalspan-bench/v0.7-baseline.json \
        --case php-extractor-integration \
        --case project-metadata-indexing \
        --case jsts-search-integration \
        --case mcp-evidence-output

  Expected: compatible true, regressions 0.

- [x] Summarize all 44 selected label rows in
  `docs/benchmarks/findings-v0.7.md` by case, profile, budget, expectation,
  terminal stage, retriever hits, ranked position, and packed position.

- [x] Explicitly confirm from the fresh output:
  - the number of selected `retrieval_missing` symbol/anchor rows;
  - v0.6 positions for `project-metadata-indexing::Run`;
  - current status of `jsts-search-integration::Search`;
  - current status of `mcp-evidence-output::codeContext`;
  - current status of `php-extractor-integration::Run`.

  If these differ from the public v0.6 artifact, record the difference and
  update numeric counts in the Decision Log before production work. Do not
  weaken the semantic gates.

- [x] Run the privacy scan used by v0.6 against attribution output. Require no:
  - source/content fields;
  - absolute Windows or Unix paths;
  - usernames;
  - environment values;
  - secret sentinel;
  - NaN or Infinity.

- [x] Record SHA-256 or Git blob-equivalent hashes of the temporary quality and
  attribution outputs in findings, then remove all four temporary files.

- [x] Commit only the frozen baseline record:

      git add PLAN.md docs/benchmarks/findings-v0.7.md
      git commit -m "docs: freeze path-scoped retrieval baseline"

---

### Task 2: Add Store-Level File Discovery

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`

**Consumes:** `files.path`, current path matching semantics, retrieval limits.

**Produces:** `Store.SearchFilePaths`.

- [ ] Add a failing test fixture with paths:

      internal/indexer/indexer.go
      internal/indexer/config.go
      internal/mcpserver/server.go
      internal/search/search.go
      docs/index.md
      testdata/repos/sample/indexer.go

  Assert:
  - `index` ranks `internal/indexer/indexer.go` before docs/testdata;
  - `mcp` finds only `internal/mcpserver/server.go` before limit;
  - exact full path wins;
  - final-segment exact wins over a deep substring;
  - output is deterministic;
  - duplicate hints do not duplicate paths;
  - backslash hints normalize;
  - empty hints return an empty non-nil slice;
  - limit 2 returns exactly 2;
  - no candidate content is read or returned.

- [ ] Confirm RED because `SearchFilePaths` is absent.

- [ ] Implement:

      func (s *Store) SearchFilePaths(
          ctx context.Context,
          hints []string,
          limit int,
      ) ([]string, error)

  using the semantics in `Store Query Semantics`.

- [ ] Build dynamic SQL only from fixed query text and placeholders. Bind every
  hint and limit.

- [ ] Add cancellation and SQL-error tests. Cancellation must return a wrapped
  context error without partial nondeterministic output.

- [ ] Add a 500-path bounded test and assert the result remains at the requested
  cap and stable across two calls.

- [ ] Run:

      go test ./internal/store -run "TestSearchFilePaths" -count=1
      go test ./internal/store -count=1
      go test ./... -count=1
      go vet ./...
      git diff --check

- [ ] Commit only store file discovery:

      git add internal/store/store.go internal/store/store_test.go
      git commit -m "feat: add bounded file path discovery"

---

### Task 3: Add Symbol Retrieval Inside Exact Paths

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/store/sqlite_spike_test.go` only if an FTS/path interaction
  needs an integration test

**Consumes:** existing symbol/chunk schema, `rankedCandidateProjection`, FTS5.

**Produces:** `Store.SearchSymbolsInPaths`.

- [ ] Create a failing store test with one file containing more than 50
  symbol-owned chunks. Put the intended function `Run` after the generic path
  search's old bounded region. Include these contents:
  - unrelated short helper functions;
  - a `Run` body containing `extract`, `index`, `store`, and `metadata`;
  - an outline for the same owner;
  - an unowned generic chunk with highly repeated query words.

  Assert:
  - `SearchPaths(..., 50)` does not guarantee the `Run` identity;
  - `SearchSymbolsInPaths` returns `Run`;
  - the unowned generic chunk is absent;
  - the non-outline `Run` chunk precedes its outline.

- [ ] Confirm RED because the new method is absent.

- [ ] Add exact and naming-style test cases:
  - `code_context`, `codeContext`, and `CodeContext` can retrieve symbol
    `codeContext` when those variants are supplied;
  - `search` retrieves `Search` case-insensitively;
  - qualified exact beats simple-name exact;
  - simple exact beats prefix;
  - prefix beats FTS-only matches.

- [ ] Add FTS-within-path tests:
  - query terms match the body of `Run` even when no symbol hint matches;
  - the same body in an unscoped path is excluded;
  - invalid FTS text is not built in the store; the safe expression arrives
    from `query.BuildFTS`;
  - empty FTS string skips the FTS pass.

- [ ] Add fairness tests with 12 candidates in one file and 3 in another:
  - per-path cap 8;
  - total cap 40;
  - both paths appear when relevant;
  - duplicate handles are removed;
  - stable ordering across two calls.

- [ ] Implement:

      func (s *Store) SearchSymbolsInPaths(
          ctx context.Context,
          paths []string,
          symbolHints []string,
          ftsQuery string,
          perPathLimit int,
          limit int,
      ) ([]model.RankedCandidate, error)

  exactly as specified in `Store Query Semantics`.

- [ ] Keep schema version unchanged and add no migration.

- [ ] Run:

      go test ./internal/store -run "TestSearchSymbolsInPaths" -count=1
      go test ./internal/store -count=1
      go test ./... -count=1
      go vet ./...
      git diff --check

- [ ] Commit only scoped symbol store behavior:

      git add internal/store/store.go internal/store/store_test.go
      git add internal/store/sqlite_spike_test.go
      git commit -m "feat: search symbols within bounded paths"

  Stage the spike test only if modified.

---

### Task 4: Define Path-Scope and Symbol-Hint Planning

**Files:**
- Modify: `internal/search/retrieval.go`
- Modify: `internal/search/retrieval_test.go`

**Consumes:** `query.Plan`, existing ranked lists, request path filters.

**Produces:** deterministic unexported scope/hint helpers.

- [ ] Write failing table tests for `pathScopeHints`.

  Inputs and required outputs:

      query: "PHPの.inc抽出結果をindexへ保存する流れはどこですか"
      includes: "index"
      excludes: "どこ", "流れ", "処理"

      query: "Where is the extractor registry assembled before adding C++?"
      includes: "extractor", "registry"
      excludes: "where", "before", "adding"

      query: "code_contextの応答を組み立てるhandlerはどこですか"
      includes: "code_context", "handler"
      excludes: "どこ"

  Assert cap 8, stable order, and case-insensitive deduplication.

- [ ] Write failing table tests for `identifierStyleVariants`.

  Required exact sets:

      code_context
        code_context
        codeContext
        CodeContext

      Service.ValidateToken
        Service.ValidateToken
        ValidateToken

      internal/search/search.go
        internal/search/search.go
        search.go
        search

  Do not require fuzzy or synonym variants.

- [ ] Write failing tests for `pathScopedSymbolHints`.

  Assert:
  - anchors precede symbols, identifiers, then words;
  - `search` is retained as a scoped symbol hint;
  - intent/navigation words are removed;
  - variants are deduplicated;
  - cap is 16;
  - original spelling is retained before generated variants.

- [ ] Write failing tests for `collectScopedPaths`:
  - request path filters first;
  - explicit path-list results second;
  - FTS paths third;
  - lexical probe paths fourth;
  - total cap 8;
  - duplicate paths keep first position;
  - no lexical probe path is used when `plan.Relations` is non-empty;
  - FTS-only mode produces no scoped paths.

- [ ] Confirm RED for all absent helpers.

- [ ] Implement the helpers without changing `query.Normalize`,
  `query.PlanQuery`, FTS terms, or the original `SearchPaths` call.

- [ ] Add property-style tests over punctuation, Unicode, empty terms, and long
  inputs. Output must contain no NUL and no item over the existing query token
  bound.

- [ ] Run:

      go test ./internal/search -run "TestPathScope|TestIdentifierStyle|TestCollectScoped" -count=1
      go test ./internal/query ./internal/search -count=1
      go test ./... -count=1
      git diff --check

- [ ] Commit only scope planning:

      git add internal/search/retrieval.go internal/search/retrieval_test.go
      git commit -m "feat: plan bounded path-scoped symbol hints"

---

### Task 5: Integrate the `path-scoped-symbol` Retriever

**Files:**
- Modify: `internal/search/search.go`
- Modify: `internal/search/retrieval.go`
- Modify: `internal/search/retrieval_test.go`
- Modify: `internal/search/trace.go`
- Modify: `internal/search/fusion.go`
- Modify: `internal/search/fusion_test.go`
- Modify: all focused fake stores that implement `search.CandidateStore`

**Consumes:** new store methods and helper outputs.

**Produces:** one bounded independently traced retriever list.

- [ ] Extend `CandidateStore` with `SearchFilePaths` and
  `SearchSymbolsInPaths`.

- [ ] Update fakes with explicit methods. A fake must record:
  - file-probe hints;
  - resolved scope paths;
  - symbol hints;
  - FTS expression;
  - per-path and total limits;
  - call order.

- [ ] Add failing mode-selection tests requiring:

      definition/full:
        qualified, exact, prefix, FTS, explicit path,
        file probe, path-scoped-symbol

      definition/no-relations:
        qualified, exact, prefix, FTS, explicit path,
        file probe, path-scoped-symbol

      callers/full:
        qualified, exact, prefix, FTS, explicit path,
        path-scoped-symbol, relation
        (no lexical file probe)

      fts-only:
        FTS only

  File-probe store calls are support operations, not `RankedList` values.

- [ ] Add a failing retriever test matching the PHP pattern:
  - no explicit path;
  - global FTS lacks `Run`;
  - `SearchFilePaths(["index", ...])` returns
    `internal/indexer/indexer.go`;
  - scoped search returns `Run`;
  - result has a `path-scoped-symbol` list containing `Run`.

- [ ] Add a failing MCP pattern test:
  - FTS seed contains another chunk from `internal/mcpserver/server.go`;
  - `code_context` generates `codeContext`;
  - scoped exact search returns `codeContext`.

- [ ] Add a failing relation-safety test:
  - a callers query has relations;
  - lexical file probe would return noisy paths if called;
  - the probe is not called;
  - FTS/explicit scope still allows exact anchor candidates;
  - relation retrieval uses only candidates matching plan anchors.

- [ ] Confirm RED before implementation.

- [ ] Add `RetrieverPathScopedSymbol` to `trace.go`.

- [ ] Add fixed weight `1.35` to `retrieverWeights`. Add a fusion test that
  proves:
  - exact symbol still outranks scoped-only signal;
  - relation still outranks scoped-only signal;
  - scoped-only signal outranks an otherwise equal FTS-only or path-only
    candidate;
  - deterministic tie order remains unchanged.

- [ ] Integrate retrieval:
  1. execute existing base retrievers unchanged;
  2. optionally probe file paths;
  3. collect at most eight scopes;
  4. call scoped symbol search with safe FTS expression and hints;
  5. append one `RankedList` when non-empty;
  6. execute relation retrieval unchanged.

- [ ] Add `RetrieverPathScopedSymbol` before `RetrieverPath`,
  `RetrieverPrefix`, and `RetrieverFTS` in relation-anchor preference, while
  preserving `qualified` and `exact` first. `candidateMatchesAnchor` remains
  mandatory.

- [ ] Update all CandidateStore implementations and fakes. Do not add fallback
  behavior that hides missing methods.

- [ ] Run:

      go test ./internal/search -run "TestRetriever|TestPathScoped|TestFusion|TestRelation" -count=1
      go test ./internal/search ./internal/store -count=1
      go test ./... -count=1
      go vet ./...
      git diff --check

- [ ] Commit only retriever integration:

      git add internal/search internal/store
      git commit -m "feat: add path-scoped symbol retriever"

  `internal/store` should be staged only if interface-adapter changes remain
  after Tasks 2 and 3.

---

### Task 6: Extend Attribution and Guard Existing Behavior

**Files:**
- Modify: `internal/benchmark/attribution.go`
- Modify: `internal/benchmark/attribution_test.go`
- Modify: `internal/benchmark/report_test.go` only if golden output includes the
  retriever enum
- Modify: relevant fixture or integration tests under `internal/app`,
  `internal/eval`, and `internal/mcpserver`

**Consumes:** new source-free retriever trace.

**Produces:** sanitized attribution and regression evidence.

- [ ] Add a failing attribution validation test for:

      retriever = "path-scoped-symbol"

  Confirm RED because `validRetriever` rejects it.

- [ ] Accept the new retriever ID and keep every other unknown value rejected.

- [ ] Add privacy tests whose scoped candidates contain:
  - source sentinel;
  - absolute path sentinel;
  - username sentinel;
  - environment sentinel.

  Assert attribution output contains only relative path, symbol, kind,
  retriever, position, and relation state.

- [ ] Add an end-to-end historical-snapshot test for one small base commit that
  verifies:
  - normal Evidence JSON has no trace field;
  - MCP structured output has no scoped paths or retriever IDs;
  - attribution output can name `path-scoped-symbol`;
  - normal packet bytes are identical with tracing off and on before trace is
    removed from the return wrapper.

- [ ] Run all Japanese relation tests and record exact values. Full-mode
  relation-bearing recall must remain `1.0`.

- [ ] Run all existing Evidence contract tests, including:
  - fidelity validity;
  - relation validity;
  - wire budget;
  - deterministic output;
  - known-handle no-resend;
  - source-free MCP summary.

- [ ] Run:

      go test ./internal/benchmark -run "TestAttribution|TestPrivacy" -count=1
      go test ./internal/app ./internal/eval ./internal/evidence ./internal/mcpserver ./internal/search -count=1
      go test ./... -count=1
      go vet ./...
      git diff --check

- [ ] Commit only attribution and regression guards:

      git add internal/benchmark internal/app internal/eval internal/evidence internal/mcpserver internal/search
      git commit -m "test: trace path-scoped symbol retrieval safely"

  Stage only files actually changed.

---

### Task 7: Run the Frozen Candidate Gate

**Files:**
- Modify: `PLAN.md`
- Modify: `docs/benchmarks/findings-v0.7.md`
- Temporary only:
  `.focalspan-bench/v0.7-candidate.{json,md,attribution.json,attribution.md}`

**Consumes:** exactly one completed production hypothesis.

**Produces:** pass/fail decision before any full run or second change.

- [ ] Before running, record:
  - candidate commit;
  - exact diff from the baseline;
  - new retriever weight;
  - path/file/candidate limits;
  - focused test results;
  - full test/vet/diff-check results.

- [ ] Run the selected four-case candidate exactly once:

      go run ./cmd/focalspan-bench run \
        --suite testdata/benchmark/focalspan-history.json \
        --case php-extractor-integration \
        --case project-metadata-indexing \
        --case jsts-search-integration \
        --case mcp-evidence-output \
        --profile default \
        --repeat 1 \
        --json-out .focalspan-bench/v0.7-candidate.json \
        --markdown-out .focalspan-bench/v0.7-candidate.md \
        --attribution-json-out .focalspan-bench/v0.7-candidate-attribution.json \
        --attribution-markdown-out .focalspan-bench/v0.7-candidate-attribution.md \
        --force

- [ ] Compare selected quality:

      go run ./cmd/focalspan-bench compare \
        --baseline docs/benchmarks/results-v0.5.json \
        --candidate .focalspan-bench/v0.7-candidate.json \
        --case php-extractor-integration \
        --case project-metadata-indexing \
        --case jsts-search-integration \
        --case mcp-evidence-output

- [ ] Evaluate every frozen gate mechanically and write a table with:
  - baseline stage/position;
  - candidate stage/position;
  - retriever hits;
  - packet presence;
  - expansion execution;
  - pass/fail reason.

- [ ] Run the privacy/finite-value scan and verify no temporary workspace,
  index, binary, or report escapes `.focalspan-bench`.

- [ ] Make exactly one decision:

  **PASS:** every hard invariant and symbol-identity gate passes. Continue to
  Task 8.

  **FAIL:** one or more gates fail. Continue to Task 9 negative branch. Do not
  alter weight, limits, SQL order, hint rules, rank, or packer.

- [ ] Record the decision in the Decision Log before any subsequent commit.

- [ ] Commit findings only:

      git add PLAN.md docs/benchmarks/findings-v0.7.md
      git commit -m "docs: record path-scoped candidate gate"

  Do not commit temporary candidate outputs.

---

### Task 8: Positive Branch — Full Acceptance and Baseline Promotion

**Run this task only when Task 7 passes.**

**Files:**
- Create: `docs/benchmarks/results-v0.7.json`
- Create: `docs/benchmarks/results-v0.7.md`
- Create: `docs/benchmarks/attribution-v0.7.json`
- Create: `docs/benchmarks/attribution-v0.7.md`
- Modify: `docs/benchmarks/findings-v0.7.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/design.md`
- Modify: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `PLAN.md`

- [ ] Run the full eight-case repeat-3 benchmark exactly once:

      go run ./cmd/focalspan-bench run \
        --suite testdata/benchmark/focalspan-history.json \
        --profile default \
        --repeat 3 \
        --json-out docs/benchmarks/results-v0.7.json \
        --markdown-out docs/benchmarks/results-v0.7.md \
        --attribution-json-out docs/benchmarks/attribution-v0.7.json \
        --attribution-markdown-out docs/benchmarks/attribution-v0.7.md \
        --force

- [ ] Compare with v0.5:

      go run ./cmd/focalspan-bench compare \
        --baseline docs/benchmarks/results-v0.5.json \
        --candidate docs/benchmarks/results-v0.7.json

  Require compatible true and zero regressions.

- [ ] Require:
  - overall required-symbol recall at least `0.25`;
  - required-path mean recall no lower than v0.5's measured `0.125`;
  - hit@5 no lower than v0.5;
  - hard invariants from the candidate gate;
  - at least one expansion executes with `known_handles`;
  - median delta-token ratio remains finite and no worse than v0.5 by more than
    10% unless expansion coverage increases and the tradeoff is documented.

- [ ] Verify the checked-in reports contain no:
  - source/content field;
  - absolute path;
  - username;
  - environment value;
  - secret sentinel;
  - NaN/Infinity.

- [ ] Update `docs/evaluation.md` with actual v0.7 metrics, commands, commit,
  environment, and limitations. Do not claim compiler-grade semantics.

- [ ] Update `docs/design.md` with the hierarchical retrieval flow and strict
  bounds. State that path probing selects files but emits no user-visible
  candidate by itself.

- [ ] Update README's development evaluation section with a short description
  of path-scoped symbol retrieval. Do not expose it as a public option.

- [ ] Change CI benchmark comparisons from
  `docs/benchmarks/results-v0.5.json` to
  `docs/benchmarks/results-v0.7.json` for smoke and manual full jobs.

- [ ] Run local workflow-equivalent commands for selected smoke.

- [ ] Commit result promotion:

      git add docs/benchmarks/results-v0.7.json \
        docs/benchmarks/results-v0.7.md \
        docs/benchmarks/attribution-v0.7.json \
        docs/benchmarks/attribution-v0.7.md \
        docs/benchmarks/findings-v0.7.md \
        docs/evaluation.md docs/design.md README.md \
        .github/workflows/ci.yml PLAN.md
      git commit -m "docs: accept path-scoped symbol retrieval v0.7"

- [ ] Push and inspect the actual GitHub Actions run. Record URL, commit, and
  conclusions for:
  - unit/vet;
  - Linux race;
  - three CGO-free builds;
  - public benchmark smoke.

  Do not trigger the manual full workflow again; the local accepted full
  report is already checked in.

---

### Task 9: Negative Branch — Revert and Preserve Evidence

**Run this task only when Task 7 fails. Skip Task 8.**

**Files:**
- Modify: `docs/benchmarks/findings-v0.7.md`
- Modify: `docs/evaluation.md`
- Modify: `PLAN.md`
- Revert production/test changes from Tasks 2 through 6

- [ ] Record exactly which frozen gates failed. Distinguish:
  - file scope absent;
  - scoped symbol absent;
  - candidate ranked but not packed;
  - packet present but expansion failed;
  - quality regression;
  - relation regression;
  - privacy or determinism failure.

- [ ] Revert only v0.7 production and feature-test commits with ordinary
  `git revert` commits, preserving history. Do not reset the branch.

- [ ] Keep Task 0/1 planning, baseline, and findings commits.

- [ ] Run closure verification:

      go test ./... -count=1
      go vet ./...
      git diff --check

- [ ] Run the selected four-case repeat-1 closure smoke once and compare with
  v0.5. Require zero regressions and restored baseline-like attribution.

- [ ] Add a negative conclusion to `docs/evaluation.md`:
  - the hypothesis;
  - the measured benefit;
  - the failed gate;
  - the revert commit;
  - the next measured failure category, without proposing a second v0.7
    production fix.

- [ ] Commit closure documentation:

      git add PLAN.md docs/benchmarks/findings-v0.7.md docs/evaluation.md
      git commit -m "docs: close rejected path-scoped retrieval v0.7"

- [ ] Push and inspect actual CI. Record the run URL and conclusions. Do not
  claim a v0.7 quality baseline or create `results-v0.7`.

---

### Task 10: Final Verification and Retrospective

**Files:**
- Modify: `PLAN.md`
- Modify: `docs/benchmarks/findings-v0.7.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/superpowers/plans/README.md` only if an archive link is missing
- Modify: `README.md` and `docs/design.md` only on the positive branch

- [ ] Run formatting and full static verification:

      gofmt -w .
      git diff --check
      go test ./... -count=1
      go vet ./...

  Record actual counts.

- [ ] Run race tests locally when supported:

      go test -race ./...

  If Windows cannot build race support, record it as unverified locally and
  cite the actual Linux CI result separately.

- [ ] Run CGO-free builds to a temporary ignored directory, then remove them:

      CGO_ENABLED=0 go build ./cmd/focalspan
      GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
      GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
      GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan
      CGO_ENABLED=0 go build ./cmd/focalspan-bench

  Use shell-appropriate environment syntax and explicit output paths.

- [ ] Run every checked-in fixture evaluation after rebuilding its index.
  Require:
  - no hit@5 regression;
  - no path/symbol recall regression;
  - budget compliance `1.0`;
  - forbidden violations `0`;
  - deterministic output `1.0`;
  - Evidence fidelity and relation validity `1.0`;
  - known resend `0`.

- [ ] Verify FTS-only behavior with focused golden/unit tests. The new store
  methods must have zero calls in FTS-only mode.

- [ ] Search production code for benchmark-specific values:

      git grep -n "php-extractor-integration" -- internal
      git grep -n "project-metadata-indexing" -- internal
      git grep -n "jsts-search-integration" -- internal
      git grep -n "mcp-evidence-output" -- internal
      git grep -n "internal/indexer/indexer.go" -- internal/search internal/store
      git grep -n "codeContext" -- internal/search internal/store

  Expected: no corpus-specific production match. Generic tests may contain
  synthetic names.

- [ ] Verify no generated binary, snapshot, temporary index, or candidate report
  remains:

      git status --short

- [ ] Complete:
  - Progress with UTC timestamps;
  - Surprises & Discoveries;
  - Decision Log;
  - Outcomes & Retrospective.

- [ ] State the milestone disposition exactly:

  **Accepted:** path-scoped symbol retrieval passed the frozen gate, full
  benchmark, and CI; `results-v0.7` is the active benchmark baseline.

  **Rejected:** the production hypothesis was reverted; attribution and
  findings are the deliverables; v0.5 remains the active quality baseline.

- [ ] Name at most one next primary milestone, selected from the terminal
  evidence:
  - scoped file discovery still misses;
  - scoped symbol matching misses;
  - candidates rank but packing drops;
  - expansion relation fails after anchor packed;
  - selected cases pass but another language/artifact dominates misses.

  Do not implement it in v0.7.

- [ ] Commit final documentation:

      git add PLAN.md docs/benchmarks/findings-v0.7.md docs/evaluation.md
      git add README.md docs/design.md docs/superpowers/plans/README.md
      git commit -m "docs: complete path-scoped symbol retrieval v0.7"

  Stage only files actually changed.

---

## Progress

Update with UTC timestamps while executing.

- [x] `2026-08-31T15:09:29Z` Task 0 completed: v0.6 root and archive
  both hash to Git blob `07c2dbdb3f1eec6b2c10a03e73feb611301f479d`;
  `go test ./... -count=1` passed 666 tests in 46 packages, `go vet ./...`
  and `git diff --check` passed. The transition is one documentation commit.
- [x] `2026-08-31T15:31:26Z` Task 1 baseline measurement completed with one
  valid run and zero retries: 4 cases, 24 quality results, 44 selected labels,
  v0.5 compatible, zero regressions, and a clean attribution privacy scan.
  Post-edit verification passed 666 tests in 46 packages, `go vet ./...`, and
  `git diff --check`; the two documentation files were committed together.
- [ ] Four-case current baseline measured and frozen.
- [ ] Store file discovery implemented and verified.
- [ ] Store scoped-symbol retrieval implemented and verified.
- [ ] Path-scope and naming-variant planning implemented.
- [ ] `path-scoped-symbol` integrated into full/no-relations retrieval.
- [ ] Attribution accepts and safely reports the new retriever.
- [ ] Frozen four-case candidate run executed once.
- [ ] Frozen gate decision recorded.
- [ ] Positive full acceptance completed, or negative revert completed.
- [ ] Local full verification completed.
- [ ] Actual remote CI inspected.
- [ ] Outcomes and next measured direction recorded.

---

## Surprises & Discoveries

- **2026-08-31:** The supplied v0.7 plan exists in the checkout as untracked
  `PLAN_v0.7.md`; the request named `PLANS_v07.md`. The repository file was
  used because its title, Plan ID, Task 0 archive path, frozen gate, and stated
  exclusions exactly match the requested milestone. It remains preserved as
  pre-existing untracked input.
- **2026-08-31:** `.focalspan-bench/` is described as ignored temporary state
  by the plan, but `git check-ignore -v .focalspan-bench` returned exit 1 in
  the current checkout. Task 1 kept the four outputs temporary and removes
  them explicitly; `.gitignore` is outside this task's frozen file scope.

---

## Decision Log

- **Decision:** Execute v0.7 in the current checkout on `master`.
  **Rationale:** At `2026-08-31T15:09:29Z`, HEAD was
  `987c5d26ad588e57c86130927bd075442ddcad98`, exactly even with
  `origin/master`; tracked files were clean, while `.focalspan.json` and
  `PLAN_v0.7.md` were untracked and are preserved. The user explicitly made
  the current checkout the only source of truth, so no alternate worktree or
  branch is introduced.
  **Date/Author:** 2026-08-31 / Codex.

- **Decision:** Accept archived v0.6 blob
  `07c2dbdb3f1eec6b2c10a03e73feb611301f479d` as the transition source.
  **Rationale:** Before replacing root `PLAN.md`, both it and
  `docs/superpowers/plans/completed/2026-08-31-v0.6-candidate-attribution-and-coverage.md`
  produced that identical `git hash-object` value. The v0.6 plan had no
  unchecked task boxes and contained its final outcomes.
  **Date/Author:** 2026-08-31 / Codex.

- **Decision:** Freeze the fresh 44-row baseline without changing numeric or
  semantic gates.
  **Rationale:** The one baseline run produced 20 retrieval-missing, 20
  packing-dropped, and 4 packed selected labels. All 44 rows match the public
  v0.6 artifact exactly; the specified PHP/MCP symbol-and-anchor subset remains
  16 retrieval misses, project metadata `Run` remains ranked 20, JSTS `Search`
  remains ranked 10 and unpacked, and MCP `codeContext` plus PHP `Run` remain
  retrieval-missing.
  **Date/Author:** 2026-08-31 / Codex.

- **Decision:** Separate file discovery from symbol retrieval.
  **Rationale:** v0.6 showed that exposing a correct file via generic path
  chunks did not expose `Run`, did not unblock expansion, and could contaminate
  relation-anchor pools.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Introduce a separately weighted `path-scoped-symbol` retriever.
  **Rationale:** A separate list makes attribution and ablation possible.
  Reusing `path` or `fts` would hide whether hierarchical retrieval supplied
  the candidate.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Freeze the new RRF weight at `1.35`.
  **Rationale:** The signal is more constrained than global prefix/FTS/path but
  less authoritative than exact-symbol or relation retrieval. Freezing it
  before measurement prevents benchmark-driven tuning.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Allow lexical file probing only for plans without relations.
  **Rationale:** v0.6 observed a Japanese relation-recall regression when broad
  path candidates entered relation-oriented searches. Definition queries need
  file discovery; relation queries can use FTS/explicit scopes and exact anchor
  filtering.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Keep FTS-only unchanged.
  **Rationale:** It remains the lexical ablation control and must not silently
  gain hierarchical retrieval.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Require packet and expansion improvement, not just retrieval
  movement.
  **Rationale:** v0.6 already demonstrated that moving a label from retrieval
  to packing without final recall is insufficient.
  **Date/Author:** 2026-08-31 / project planning.

- **Decision:** Exclude MCP server Instructions from v0.7.
  **Rationale:** Tool-use guidance cannot repair missing candidate identities
  and would confound evaluation of the retrieval hypothesis. It belongs in a
  later independently measured usability milestone.
  **Date/Author:** 2026-08-31 / project planning.

---

## Outcomes & Retrospective

Implementation has not begun. At completion, replace this paragraph with:

- starting and final commits;
- accepted or rejected disposition;
- exact selected baseline and candidate stage counts;
- symbol recall and expansion results;
- full benchmark metrics when accepted;
- regression and privacy status;
- local and remote verification;
- limitations;
- one evidence-selected next milestone.

Do not describe retrieval quality as improved unless the positive branch passes
all frozen gates and the full report is accepted.

---

## Validation and Acceptance

v0.7 is complete only when all applicable statements are true:

- The completed v0.6 plan is archived byte-for-byte.
- The fresh four-case baseline is measured before production changes.
- `SearchFilePaths` returns bounded paths without chunks or content.
- `SearchSymbolsInPaths` returns only symbol-owned candidates from exact scopes.
- File scopes, hints, and candidates obey all caps.
- Existing query normalization and planning are unchanged.
- Existing `SearchPaths` semantics are unchanged.
- FTS-only mode is unchanged and never calls new store methods.
- Lexical file probing does not run for relation plans.
- The new retriever has its own trace identity and fixed weight.
- Normal CLI/MCP/Evidence output contains no trace or scope data.
- Attribution remains deterministic, source-free, finite, and path-relative.
- The candidate is run once against the frozen four-case gate.
- No second production hypothesis is attempted after the gate.
- Positive branch: symbols are packed, an expansion is unblocked, full
  benchmark and CI pass, and `results-v0.7` becomes the baseline.
- Negative branch: production changes are reverted, closure checks and CI pass,
  and no `results-v0.7` quality claim exists.
- Existing fixture and Evidence metrics do not regress.
- CGO-free cross-builds pass.
- Linux race CI is actually inspected.
- The plan's Progress, discoveries, decisions, and retrospective are complete.
- No corpus-specific production logic or incomplete stub remains.

---

## Idempotence and Recovery

- Store search methods are read-only and safe to rerun.
- Empty hints, empty paths, and empty FTS expressions return empty slices rather
  than errors.
- Cancellation propagates and does not mutate the index.
- Baseline and candidate benchmark outputs live under ignored
  `.focalspan-bench/` and may be deleted and regenerated only within the
  run-count rules.
- If a baseline run fails before producing a valid report because of an
  infrastructure error, record the cause and retry once.
- If a candidate run produces a valid report, do not rerun it to seek a better
  result.
- If Task 7 fails, use `git revert` for v0.7 production commits. Preserve
  chronological evidence; do not reset or edit history.
- If a checked-in historical ref disappears, validation fails. Do not silently
  substitute another commit.
- If the positive full report fails privacy validation, do not commit it.
- If remote CI fails for infrastructure unrelated to code, record the job and
  reason before rerunning. Do not report success until an actual run passes.
- Leave the completed root plan in place until the next plan transition.

---

## Interfaces and Dependencies

Allowed dependency direction remains:

    internal/search
      -> internal/query
      -> internal/model

    internal/store
      -> internal/model

    internal/app
      -> internal/search
      -> internal/store

    internal/benchmark
      -> internal/app
      -> internal/search trace

`internal/store` must not import `internal/search` or `internal/query`.
Therefore `SearchSymbolsInPaths` accepts primitive paths, symbol hints, and the
already-safe FTS string rather than a query-plan type.

`internal/search` owns:

- hint derivation;
- naming variants;
- path-scope assembly;
- retriever identity;
- mode boundaries;
- fixed limits.

`internal/store` owns:

- parameterized file-path lookup;
- exact-path symbol lookup;
- path-constrained FTS;
- deterministic SQL ordering;
- per-path and total caps.

`internal/rank`, `internal/evidence`, public CLI, and MCP server do not gain a
dependency on the new helper types.

No new third-party dependency is expected.
