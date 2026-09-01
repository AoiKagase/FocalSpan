# FocalSpan v0.11 Identity-Bridge Retrieval Plan

## Purpose / Big Picture

Reduce retrieval misses without increasing the frozen v0.10 wire baseline. The
largest remaining gap is `path_scope_missing`; this milestone adds one bounded
identity bridge that resolves natural-language package/module hints to
structural entry points, then to symbol-bearing paths and symbols. It does not
change ranking, packing, Evidence, MCP schemas, or known handles.

Frozen gate: focused/2048 path-scope misses 9, symbol misses 2,
packing drops 7, packed labels 5, cumulative estimated wire tokens 12,740,
useful-evidence efficiency 0.3925.

## Context and Orientation

- `internal/query`: normalized terms and intent/anchor planning.
- `internal/search/retrieval.go`: ordered retriever execution and relation
  anchor selection.
- `internal/search/search.go`: fusion, path filtering, and final ranking.
- `internal/store/store.go`: SQLite candidate queries.
- Structural entry-point kinds include `package`, `module`, `crate_module`,
  `compilation_unit`, `translation_unit`, `pawn_unit`, and `xaml_document`.
- `internal/linker` already contains path-mapping logic; this milestone must
  not widen linker relation resolution or persist new schema.

## Interfaces and Dependencies

The public search, CLI, MCP, `focalspan.context.v1`, and `known_handles`
interfaces remain unchanged. Add only an internal CandidateStore method and a
retriever list entry. The bridge accepts bounded structural hints and returns
ordinary `model.RankedCandidate` values. Generic documentation/configuration
chunks are excluded. No source content is placed in trace-only bridge data.

## Plan of Work

### Task 0: Freeze transition

- [x] Archive v0.10 byte-identically at
  `docs/superpowers/plans/completed/2026-09-02-v0.10-evidence-compaction.md`.
- [x] Replace the root plan with this single v0.11 milestone before product
  edits; preserve dirty user files (`AGENTS.md`, `.focalspan.json`,
  `TASKS.md`).

### Task 1: RED tests for staged identity bridge

Files: `internal/search/retrieval_test.go`, `internal/search/search_test.go`,
`internal/store/store_test.go`.

- [x] Test that natural-language package/module hints produce a structural
  bridge query, followed by symbol-bearing path candidates, while explicit
  path terms continue using only `SearchPaths`.
- [x] Test that bridge candidates are deterministic, bounded, and fused as a
  distinct internal retriever without changing existing retriever order.
- [x] Test that documentation/configuration chunks and broad arbitrary words
  are never promoted to structural anchors.
- [x] Test relation expansion receives only exact/structural symbol anchors,
  never a broad path candidate.
- [x] Run the focused tests and record the expected RED failure.

### Task 2: Minimal GREEN implementation

Files: `internal/query/planner.go` or a new internal helper,
`internal/search/retrieval.go`, `internal/search/search.go`,
`internal/store/store.go`, and their tests.

- [x] Derive at most a small deterministic set of bridge hints from package,
  module, namespace, crate, or equivalent structural language terms. Do not
  pass all natural-language words to path search.
- [x] Add a store query that first selects structural entry-point candidates
  by exact/qualified identity, then returns only their symbol-bearing paths
  and symbols. Use stable path/line/handle ordering and the existing limit.
- [x] Mark bridge results with an internal retriever identity/score only;
  preserve existing public trace fields and ranking weights unless required
  to keep deterministic fusion. Do not adjust packing or Evidence.
- [x] Ensure path filters and changed-only filters still apply after fusion.
- [x] Keep relation anchors restricted to exact symbol or bridge-resolved
  structural symbols; do not feed generic documents or raw path expansions.
- [x] Run focused GREEN tests, then the full package suite.

### Task 3: Static verification and candidate gate

- [x] `gofmt` changed Go files and run `git diff --check`.
- [x] Run `go test ./... -count=1` and `go vet ./...`.
- [x] Run native and CGO-free Windows amd64, Linux amd64, and Darwin arm64
  builds into a temporary directory, then remove generated artifacts.
- [x] Run the historical focused/2048 candidate benchmark exactly once after
  static verification; retry only an infrastructure failure once.
- [ ] Accept only if at least three path/symbol/anchor labels improve, wire is
  `<=12,740`, efficiency is `>0.3925`, and all fidelity, budget,
  deterministic-ordering, known-handle, and MCP contract tests remain green.
- [x] Record execution status and privacy scan in
  `docs/benchmarks/findings-v0.11.md` without source text or absolute paths.

### Task 4: Closure and recovery

- [ ] If the gate fails, reverse-revert only this milestone's product commits,
  retain the negative findings, and leave the v0.10 baseline intact.
- [x] Update this plan with UTC progress, discoveries, decisions, outcomes,
  and actual verification evidence. Do not modify archived plans.
- [x] Preserve user-owned dirty/untracked files and remove generated reports,
  indexes, binaries, caches, and temporary workspaces.

## Validation and Acceptance

Identity bridge behavior must be deterministic and bounded. Existing explicit
path queries, exact symbol queries, relation provenance, source fidelity,
budget limits, packet bytes, MCP method contracts, and known-handle behavior
must remain unchanged. The candidate gate is strict: three or more improved
path/symbol/anchor labels, no wire increase, and efficiency strictly above
0.3925.

## Idempotence and Recovery

Search is read-only and repeatable. No database schema migration is allowed.
Candidate benchmarks run once after static verification. A failed gate is
recovered with normal reverse-order `git revert`; documentation remains as
historical evidence. Race testing may be recorded `UNVERIFIED` only for the
known local MinGW 64-bit compiler limitation.

## Progress

- [x] 2026-09-02: v0.10 plan archived byte-identically and v0.11 plan made
  active; no user-owned files staged.
- [x] 2026-09-01T23:39Z: identity bridge RED tests failed for the missing
  retriever and store API, then focused search/store tests passed GREEN.
- [x] 2026-09-01T23:44Z: full tests, vet, diff check, and all three distinct
  CGO-free targets passed; temporary outputs were removed.
- [x] 2026-09-01T23:44Z: benchmark first invocation had no report (infra
  failure); one retry stopped on the pre-existing attribution allow-list.
  The allow-list was fixed with a RED/GREEN regression test; no rerun was made.
- [x] Task 1 RED tests.
- [x] Task 2 bridge implementation.
- [x] Task 3 static verification and benchmark execution attempt.
- [x] Task 4 closure documentation; promotion gate remains unmeasured.

## Surprises & Discoveries

- Structural owner chunks are not present for every language (for example Go
  package owners), so the bridge scopes files through owner symbols and then
  returns only child symbol chunks.
- SQLite FTS aliases cannot be used with `MATCH`; the bridge uses the canonical
  FTS table name and still preserves deterministic ordering.
- The development attribution validator had a closed retriever allow-list;
  adding a retriever requires an explicit validator regression test.

## Decision Log

- 2026-09-02: Execute only the first recommended hypothesis in this
  milestone; later packing, excerpt, metadata, known-handle, guidance, cap,
  SQL-batch, schema-v2, estimator, envelope, and handle candidates remain
  separate milestones.
- 2026-09-02: Natural-language words are not sent wholesale to `SearchPaths`;
  bridge hints must pass through structural identity and exclude generic docs.
- 2026-09-01: Because the only permitted benchmark retry stopped on an
  implementation validation error, the v0.11 candidate gate is recorded as
  unmeasured rather than inferring an efficiency result.

## Outcomes & Retrospective

Implementation and static verification are complete, but the historical
candidate gate is unmeasured after the allowed benchmark retry stopped at
attribution validation. The implementation is not promoted as a new quality
baseline; findings and a post-fix successor action are recorded in
`docs/benchmarks/findings-v0.11.md`.
