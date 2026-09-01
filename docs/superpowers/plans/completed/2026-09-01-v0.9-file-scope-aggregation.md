# FocalSpan File-Scope Retrieval v0.9 Implementation Plan

> **For agentic workers:** Execute task-by-task. Every production behavior
> starts with a failing deterministic test, then RED confirmation, minimal
> GREEN implementation, focused verification, and an explicit-path commit.

**Goal:** Improve Codex/MCP useful-evidence-per-token by finding likely files
from the complete existing `index.db` before selecting source-faithful chunks,
without changing public MCP or Evidence contracts.

**Architecture:** Add read-only file-level projections at query time over the
existing `files`, `symbols`, `chunks`, and `chunk_fts` tables. Aggregate
symbol, FTS, and path signals into a bounded deterministic file scope, then
retrieve a small number of symbol-owned chunks inside that scope through one
separately traced retriever. Ranking, packing, wire format, and normal MCP
output remain unchanged.

**Tech Stack:** Go 1.27, standard library, existing SQLite/FTS5 store,
`internal/{query,search,store,benchmark,benchcli,evidence}` packages, and the
existing historical benchmark harness. No new dependency or schema migration.

**Spec:** User-approved v0.9 design: existing `index.db` first, file-level
evidence aggregation, balanced acceptance gate, SCIP deferred to a separate
milestone.

**Plan ID:** `v0.9-file-scope-aggregation`

**Status:** Completed — candidate rejected; no file-scope production change promoted.

## Global Constraints

- Preserve user-owned dirty `AGENTS.md`, untracked `.focalspan.json`, and
  untracked `TASKS.md`; stage only intended paths.
- Archive completed v0.8 byte-for-byte before changing the root plan.
- Do not use `git reset`, `git restore`, `git checkout --`, `git clean`, or
  `git stash`.
- Keep `focalspan.context.v1`, `focalspan.benchmark-attribution.v1`, all MCP
  tool names/instructions, `known_handles`, Evidence fidelity, and normal CLI
  output unchanged.
- Do not modify query normalization/planning, existing retriever SQL,
  relation linking, rank weights except the frozen new retriever weight, or
  packer/token estimator behavior.
- Do not add network, external LLMs, embeddings, SCIP generation,
  repository-code execution, package restore, or runtime dependencies.
- New code is read-only against the index and uses bound SQL values.
- Every production behavior begins with a failing deterministic test.
- Candidate measurements run only after static verification and only once;
  infrastructure failure may be retried once after recording the cause.
- A failed candidate gate triggers normal reverse-order `git revert`; findings
  remain recorded and no quality baseline is promoted.

## Purpose / Big Picture

v0.8 measured 95 historical labels: 55 `retrieval_missing`, 35
`packing_dropped`, and 5 `packed`. Focused/2048 unmet counts were 9
`path_scope_missing`, 2 `symbol_match_missing`, 0 `ranking_dropped`, and 7
`packing_dropped`; `path_scope_missing` is the next primary layer.

The rejected v0.7 experiment selected paths from already returned chunk
candidates and a bounded lexical probe. It did not advance PHP `Run` or MCP
`codeContext`. v0.9 instead groups each signal by file across the complete
indexed tables before candidate caps, so one file's many chunks cannot hide
other likely files.

The development metric is:

`1000 * unique packed required evidence labels / sum of final serialized
estimated tokens for code_context and executed code_expand responses`.

Required labels are unique packed paths, symbols, and successfully executed
expansion expectations. Internal scores never enter normal MCP responses.

## Context and Orientation

- `internal/search/search.go`: `CandidateStore`, `SearchRequest`, entrypoint.
- `internal/search/retrieval.go`: independent retrievers and trace lists.
- `internal/store/store.go`: parameterized SQLite/FTS5 queries.
- `internal/benchmark` and `internal/benchcli`: source-free attribution and
  quality/benchmark reports.
- `docs/benchmarks/findings-v0.8.md`: accepted diagnosis evidence;
  `docs/benchmarks/results-v0.5.json`: quality baseline.

## Interfaces and Dependencies

No public API changes. Extend only the internal `CandidateStore` boundary with
read-only methods equivalent to:

```go
SearchSymbolFiles(ctx context.Context, hints []string, limit int) ([]string, error)
SearchFTSFiles(ctx context.Context, ftsQuery string, limit int) ([]string, error)
SearchPathFiles(ctx context.Context, hints []string, limit int) ([]string, error)
SearchCandidatesInFiles(ctx context.Context, paths, symbolHints []string, ftsQuery string, perFileLimit, totalLimit int) ([]model.RankedCandidate, error)
```

The store returns only repository-relative paths for file methods and only
symbol-owned source candidates for the final method. `internal/search` owns
hint derivation, file-list RRF, caps, mode gating, retriever identity, and the
fixed `1.35` weight for `path-scope-aggregate`. `internal/store` owns SQL
ordering and caps without importing search or query packages.

## Plan of Work

### Task 0: Transition from v0.8 and Freeze Baseline

- [x] Verify branch, HEAD, status, `PLANS.md`, archive hash, and baseline tests.
- [x] Archive v0.8 at
  `docs/superpowers/plans/completed/2026-09-01-v0.8-failure-layer-attribution.md`
  byte-for-byte and verify matching SHA-256.
- [x] Make this v0.9 plan the sole active root plan and commit only the
  transition as `docs: start file-scope aggregation v0.9`.
- [x] Run the current eight-case `full-evidence-focused`/2048 benchmark once;
  freeze path-scope counts, packed labels, and cumulative token denominator.

### Task 1: Add File-Level Store Queries (RED/GREEN)

- [x] Add RED tests for exact/final-segment/prefix/substring path ordering,
  case-insensitive deduplication, empty input, cancellation, limits, and
  path-only output.
- [x] Add RED tests proving FTS results group by distinct file and that a
  high-frequency file cannot consume the complete file scope with chunks.
- [x] Add RED tests for symbol-file lookup and path-constrained candidates,
  including required `symbol_handle`, per-file, total, and stable tie-breaks.
- [x] Confirm RED, then add parameterized methods to `internal/store/store.go`
  and `CandidateStore`; do not change migrations or schema version.
- [x] Use exact and naming-variant symbol passes followed by safe FTS inside
  selected paths; exclude generic unowned chunks and preserve source fields.
- [x] Run focused and full store tests; commit
  `feat: add indexed file-scope queries`.

  The implementation was measured and then reverted after the frozen
  candidate gate failed; the commit remains in history as the attempted
  candidate.

### Task 2: Aggregate Scopes and Integrate Retriever (RED/GREEN)

- [x] Add RED tests for hint filtering, naming variants, navigation-word
  exclusion, eight-file scope cap, stable file RRF, per-file fairness, total
  24-candidate cap, and fixed retriever ID.
- [x] Add RED tests that definition full/no-relations queries call the new
  path, while relation-bearing and `fts-only` queries do not.
- [x] Confirm RED, then derive at most eight file hints and sixteen symbol
  hints from the existing `query.Plan`.
- [x] Fuse file lists with RRF `k=60` and weights 1.80/1.00/0.90, retain eight
  paths, retrieve at most four candidates per path and 24 total, and integrate
  one `path-scope-aggregate` list at frozen weight 1.35.
- [x] Run focused and full search tests; commit
  `feat: aggregate indexed file scopes for retrieval`.

  The implementation was measured and then reverted after the frozen
  candidate gate failed; the commit remains in history as the attempted
  candidate.

### Task 3: Efficiency Metric and Frozen Candidate Gate

- [x] Add RED tests for cumulative context/expansion token accounting, unique
  packed-label counting, and zero-denominator handling.
- [x] Add RED tests proving normal quality JSON, MCP structured output, and
  `focalspan.context.v1` bytes are unchanged with tracing enabled.
- [x] Implement development-only useful-evidence-per-1,000-token reporting;
  preserve attribution v1 compatibility and normal-output privacy.
- [x] After tests, vet, and diff check pass, run the selected candidate once.
- [x] Accept only if at least 3 of 9 focused/2048 path-scope misses advance,
  at least one new human label is packed, useful-evidence/token strictly
  improves, and all existing packed labels/invariants remain valid.
- [x] If accepted, run the complete benchmark matrix and record results. If
  rejected, revert only candidate commits and record negative evidence.

  Candidate rejected: path scope 9→9, packed labels 5→5, and efficiency
  0.3925→0.3838. Negative evidence is recorded in
  `docs/benchmarks/findings-v0.9.md`.

### Task 4: Closure and Verification

- [x] Run changed-file formatting, `git diff --check`, `go test ./... -count=1`,
  and `go vet ./...`.
- [x] Run Evidence/MCP/privacy, fixture, deterministic, finite-number,
  forbidden-path, relation-validity, budget, and known-handle tests.
- [x] Run CGO-free native and Windows amd64/Linux amd64/Darwin arm64 builds to
  an ignored temporary directory, then remove generated files.
- [x] Inspect actual CI conclusions for tests, vet, Linux race, cross-builds,
  and public smoke; skipped jobs remain skipped.

  No new remote CI run was available because the candidate was reverted
  locally; local race remains unverified because the installed MinGW compiler
  reports `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.
- [x] Verify status contains no generated reports, indexes, binaries, or
  temporary workspaces and preserves all pre-existing user changes.
- [x] Complete Progress, Discoveries, Decision Log, Outcomes, and recovery
  notes; commit documentation separately from product code.

## Validation and Acceptance

- File methods are deterministic, bounded, parameterized, path-only, and
  read-only; candidate methods return only symbol-owned chunks.
- FTS-only, relation-bearing retrieval, attribution v1, normal MCP, and
  `focalspan.context.v1` remain compatible.
- Budget, source fidelity, relation, deterministic, privacy, finite-value,
  forbidden-path, and known-handle invariants pass.
- The balanced focused/2048 gate passes exactly as specified in Task 3, or the
  production candidate is reverted with negative findings retained.
- No SCIP import or generation is included in v0.9.

## Idempotence and Recovery

- File queries and hint derivation are read-only and safe to repeat.
- Temporary benchmark outputs are removed only after hashes and counts are
  recorded; a valid candidate report is never rerun for a better result.
- Infrastructure retry is recorded before one retry; code failures require a
  new RED test rather than a weakened gate.
- Failed candidates use normal reverse-order `git revert`; completed archives
  remain immutable.

## Progress

- [x] `2026-09-01` v0.8 selected `path_scope_missing`; existing `index.db` reuse,
  SCIP deferral, file aggregation, and balanced gate were agreed.
- [x] `2026-09-01T07:30Z` v0.8 archive, v0.9 transition commit `a98f9fc`,
  and the eight-case repeat-1 baseline measurement completed. Baseline focused
  2048 counts were path 9, symbol 2, packing 7, packed 1; cumulative wire
  denominator was 12,740 tokens.
- [x] `2026-09-01T08:07Z` Store file-level queries were implemented and
  focused/full tests passed; candidate commit `ef8bfe0` was later reverted.
- [x] `2026-09-01T08:07Z` Scope aggregation and retriever integration were
  implemented and focused/full tests passed; candidate commit `8accf98` was
  later reverted.
- [x] `2026-09-01T08:11Z` Development efficiency reporting and deterministic
  label accounting were added in `767e896`; the candidate gate was executed
  with one recorded infrastructure retry and rejected.
- [x] `2026-09-01T08:39Z` Closure verification and documentation completed:
  687 tests and vet passed after revert, CGO-free native/cross-builds passed,
  and race was blocked by the local compiler limitation.

## Surprises & Discoveries

- `TASKS.md` is a separate future schema-v2 relation-linking task. It targets
  update latency and explicitly excludes search, Evidence, MCP, and token
  behavior; it is not part of v0.9.
- v0.7's path-scoped-symbol candidate was reverted after no PHP/MCP identity
  advanced. v0.9 changes the scope source to full-index file aggregation.
- SQLite FTS file ranking could not safely aggregate `bm25()` at the grouped
  file level in the current query shape; the implementation uses deterministic
  match counts with path tie-breaks and leaves BM25 refinement for a later
  hypothesis.
- Explicit `SearchRequest.Paths` needed a pre-candidate hard filter. Without
  it, broad symbol/FTS files could consume the bounded scope and be discarded
  only after retrieval; the regression test caught and fixed this before the
  candidate run.
- The candidate changed wire token counts without changing any measured label
  stage. File scope alone did not bridge the selected files to the intended
  symbols on this historical corpus.

## Decision Log

- **2026-09-01 / user and Codex:** Use `.focalspan/index.db` as canonical;
  SCIP is optional future enrichment, never a replacement.
- **2026-09-01 / user and Codex:** v0.9 precedes `TASKS.md` because the goal is
  useful evidence per Codex/MCP token; schema v2 primarily improves update
  latency.
- **2026-09-01 / user and Codex:** Choose file-level signal aggregation over
  a new file FTS schema or multi-turn MCP interaction.
- **2026-09-01 / user and Codex:** Freeze the balanced gate: 3/9 advances, one
  new packed label, strict evidence/token improvement, no regressions.
- **2026-09-01 / Codex:** Reject the v0.9 candidate after the single completed
  measurement: path-scope misses stayed at 9, packed labels stayed at 5, and
  useful evidence per 1,000 tokens fell from 0.3925 to 0.3838. Revert only the
  two candidate production commits and preserve the negative findings.

## Outcomes & Retrospective

v0.9's store and search implementation was completed with deterministic RED→
GREEN tests, then rejected by its frozen candidate gate and reverted via
`3f8c3f2` and `63381fc` in reverse order. The development-only efficiency
reporting remains available for future candidates without changing normal
quality JSON or the Evidence/MCP contracts. The durable negative measurement
is in `docs/benchmarks/findings-v0.9.md`; no v0.9 quality baseline was
promoted.

Local closure evidence is 687 passing tests in 46 packages and a clean vet
run. Evidence/MCP/privacy-focused tests passed 311 cases in 8 packages.
CGO-free native, Windows amd64, Linux amd64, and Darwin arm64 builds passed
and their ignored temporary outputs were removed. Local race testing remains
unverified because the installed MinGW `cc1.exe` cannot compile 64-bit mode;
no new remote CI run was available after the local rejection/revert.
