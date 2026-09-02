# FocalSpan v0.13 Adaptive Focused Excerpt Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement this
> plan task-by-task. Keep the RED/GREEN verification order and update this
> file with actual results.

**Goal:** Reduce wire tokens for long focused Evidence without losing required
symbol/relation hits, source fidelity, budget compliance, or deterministic
ordering.

**Architecture:** Keep the current `focusedSegments` representation and public
Evidence packet unchanged. Add a private adaptive focused excerpt variant in
`internal/evidence/fidelity.go` that reuses the current ranked hit selection
but narrows declaration and hit context only when it is measurably smaller than
the existing excerpt. The existing variant and selection logic remain the
fallback, so short sources, source mode, outline mode, and candidates without a
useful reduction retain their current behavior.

**Tech Stack:** Go, deterministic line-window selection, the existing
`budget.TokenEstimator`, `internal/evidence` compiler/fidelity tests, and the
source-free historical benchmark.

**Spec:** User-approved “FocalSpan token効率改善候補・優先順位”, rank 3
`長いsourceのadaptive focused excerpt`.

## Purpose / Big Picture

Focused Evidence already preserves source bytes but can carry more declaration
and surrounding context than the query needs. The adaptive variant will be
considered only for long candidates and will contain exact source line slices
around the same highest-scoring focused terms. The compiler may select the
smaller variant through its existing utility-per-token comparison; no public
field, schema, retriever, ranker, relation resolver, or known-handle contract
changes.

## Global Constraints

- Do not change retrieval, RRF/ranking weights, query planning, Evidence schema,
  MCP/CLI methods, `known_handles`, or SQLite schema.
- Only `ModeFocused` receives the adaptive variant; `ModeSource` remains
  verbatim-first and `ModeOutline` remains synthetic/signature-only.
- Every emitted source segment must be an exact byte slice of the candidate
  content; omitted segments contain no generated source text.
- Required symbol metadata, relation provenance, valid relation endpoints,
  hard serialized budgets, and deterministic output ordering must remain
  unchanged.
- Development attribution/diagnosis reports remain source-free and outside
  normal MCP/CLI output.
- Run the historical candidate benchmark once after static verification; retry
  only a clear infrastructure failure once, and never promote an unmeasured
  result.
- Preserve user-owned dirty files (`AGENTS.md`, `.focalspan.json`, `TASKS.md`)
  and stage explicit paths only.
- `PLAN.md` is the only active ExecPlan. The v0.12 archive is immutable.

## Context and Orientation

- `internal/evidence/fidelity.go` builds ordered `ContentVariant` values. The
  current focused excerpt calls `focusedSegments` from `segments.go`.
- `internal/evidence/segments.go` selects a declaration prefix and up to three
  scored hit windows with two lines before and four lines after each hit.
- `internal/evidence/compiler.go` compares variants by utility per measured
  incremental wire cost; changing variant cost does not change candidate
  retrieval or relation construction.
- `internal/evidence/fidelity_test.go` and `segments_test.go` assert source
  fidelity, late-hit retention, UTF-8/CRLF handling, and variant ordering.
- `internal/evidence/compiler_test.go`, `wire_test.go`, and `validate_test.go`
  cover packet budgets, relation endpoint validity, known handles, and
  deterministic JSON.
- The active v0.10 baseline is retained after rejected v0.11/v0.12 candidates:
  focused/2048 `packing_dropped=7`, packed labels `5`, cumulative wire tokens
  `12,740`, useful evidence efficiency `0.3925`, and median metadata overhead
  `0.924`.

## Plan of Work

### Task 0: Freeze transition

- [x] Archive the completed v0.12 root plan byte-identically at
  `docs/superpowers/plans/completed/2026-09-02-v0.12-anchor-first-evidence-packing.md`.
- [x] Replace the root plan with this v0.13 plan before product edits.
- [ ] Commit only the archive and root plan as one documentation-only
  transition, leaving `AGENTS.md`, `.focalspan.json`, and `TASKS.md` unstaged.

### Task 1: RED tests for adaptive excerpts

**Files:** `internal/evidence/fidelity_test.go`,
`internal/evidence/segments_test.go`, `internal/evidence/compiler_test.go`.

- [ ] Add `TestBuildVariantsAddsSmallerAdaptiveExcerptForLongFocusedSource`.
  Build a 60+ line method with three separated query hits. Assert that focused
  variants contain the existing excerpt and a second excerpt with fewer
  `EvidenceTokens`; do not accept a new Fidelity value.
- [ ] Add `TestAdaptiveFocusedSegmentsPreserveExactHitLines`.
  Call the private adaptive helper on long UTF-8 content with a late identifier
  and CRLF content. Assert every source segment equals the corresponding
  `testSourceLines` slice, every hit identifier is present, omitted segments
  have empty text, and the adaptive source-token count is lower than the
  standard focused result.
- [ ] Add `TestAdaptiveExcerptDoesNotChangeShortOrNonFocusedModes`.
  Assert a short target has no additional adaptive variant, `ModeSource` keeps
  its verbatim-first variants, and `ModeOutline` has no excerpt variant.
- [ ] Add `TestAdaptiveCompilerKeepsRelationAndBudgetInvariants`.
  Compile a long target plus caller/test relation candidates at budgets 512,
  1200, and 2048. Assert required target/caller handles, the same valid edge
  direction/kind/certainty, exact source slices, `Validate` success, and
  `Budget.Used <= Budget.Limit`.
- [ ] Run only the new tests with a repository-local Go cache and record the
  expected failures before editing production code. The RED result must be a
  missing adaptive variant/helper, not a cache or toolchain failure.

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/segments.go`, `internal/evidence/fidelity.go`.

- [ ] Extract the current window construction into a private helper that keeps
  the existing margins (`before=2`, `after=4`, prefix through
  `declarationPrefixEnd`) byte-for-byte equivalent for `focusedSegments`.
- [ ] Add `adaptiveFocusedSegments(candidate, plan)` using the same term scoring,
  hit ordering, merge rule, and three-window cap, with `before=0`, `after=1`,
  and a declaration prefix capped at two lines. Build segments only through
  `joinLines` and `absoluteLines`; never synthesize markers in `Text`.
- [ ] In `BuildVariants`, after constructing the normal focused excerpt, append
  the adaptive excerpt only when the candidate has at least 40 indexed lines,
  both excerpts contain source, and the adaptive excerpt's estimated source
  tokens are strictly lower. Keep the normal excerpt first and deduplicate
  variants with the existing helper.
- [ ] Keep `ModeSource`, `ModeOutline`, synthetic candidates, signature
  fallback, and all public types unchanged.
- [ ] Run the focused Evidence tests and then `go test ./... -count=1`; adjust
  only implementation details needed to satisfy the RED tests and preserve
  existing behavior.

### Task 3: Static verification

- [ ] Run `gofmt` on changed Go files and `git diff --check`.
- [ ] Run `go test ./... -count=1` and `go vet ./...` with a repository-local
  cache if the default cache is denied.
- [ ] Run native plus CGO-free Windows amd64, Linux amd64, and Darwin arm64
  builds into a temporary directory; remove generated outputs afterward.
- [ ] Run `go test -race ./...`; if the known local MinGW 64-bit compiler
  limitation repeats, record it as `UNVERIFIED` and do not label it passed.

### Task 4: Candidate benchmark gate

- [ ] Run the historical `focalspan-history-v0.5` suite exactly once with the
  `default` profile, `repeat 1`, attribution enabled, and diagnosis enabled.
- [ ] Compare against the v0.10 baseline and require all of the following:
  focused/2048 `packing_dropped` does not increase; packed labels remain at
  least `5`; cumulative wire tokens are strictly below `12,740`; useful
  evidence efficiency is strictly above `0.3925`; all quality, fidelity,
  relation, budget, deterministic-ordering, known-handle, and MCP contract
  checks pass; and no source text, absolute path, username, or secret appears
  in development reports.
- [ ] Record measured values, artifact hashes, compatibility result, and
  privacy scan in `docs/benchmarks/findings-v0.13.md` without source text or
  absolute paths. Do not record a new baseline unless the strict gate passes.

### Task 5: Closure and recovery

- [ ] If the gate passes, keep the bounded product commit, update this plan
  with actual measurements, and identify the new baseline explicitly.
- [ ] If the gate fails, reverse-revert only the v0.13 product commit(s), keep
  the RED/GREEN and benchmark findings as historical evidence, and leave the
  v0.10 product baseline active.
- [ ] Remove generated reports, indexes, binaries, caches, and temporary
  workspaces; preserve all user-owned dirty files.
- [ ] Update this plan with UTC progress, discoveries, decisions, outcomes,
  and exact verification status. Never edit the archived v0.12 plan.

## Validation and Acceptance

For every adaptive packet, all source segments must be exact slices of the
candidate source, required query hits and relation endpoints must remain
present, and `MeasureModelVisible(packet)` must equal the reported usage and
stay within the clamped limit. Repeated identical inputs must marshal to
identical JSON. The strict candidate gate requires a wire-token reduction with
no packed-label loss or quality/invariant regression; otherwise the candidate
is rejected and reverted.

## Idempotence and Recovery

Adaptive variants are computed in memory from candidate content and query terms;
no index or persistent state changes. Re-running compilation with identical
inputs is deterministic. A rejected candidate is recovered with ordinary
reverse-order `git revert` of only its explicit product commit(s); findings and
the immutable v0.12 archive remain.

## Interfaces and Dependencies

No public interface changes. `model.PackRequest`, `model.ContextBundle`, the
Evidence packet schema `focalspan.context.v1`, MCP methods, CLI output, and
`known_handles` remain unchanged. The adaptive helper is private to
`internal/evidence`; benchmark attribution and diagnosis observe only the
existing packet and source-free labels.

## Progress

- [x] 2026-09-02: v0.12 archive created from the active plan with matching
  SHA-256 `FD824A54157E0F45E481B5854954192C35C8B61E2CDECA0D37B21F68E5F11887`.
- [x] 2026-09-02: v0.13 design approved; transition plan written before
  product edits.
- [ ] Record the documentation-only transition commit hash.
- [ ] Record RED test output.
- [ ] Record GREEN/static verification output.
- [ ] Record the one candidate benchmark and gate decision.

## Surprises & Discoveries

- v0.12 showed that admitting more structural Evidence can improve recall and
  efficiency while violating the no-wire-growth gate; v0.13 therefore adds a
  variant only when its measured source token cost is lower.
- The existing focused excerpt already preserves late lexical hits and exact
  source bytes, so adaptive behavior must narrow context rather than invent
  summaries or change hit ranking.

## Decision Log

- 2026-09-02: Proceed to rank 3 only after v0.12 anchor-first packing was
  rejected for wire growth.
- 2026-09-02: Keep the normal excerpt as the first variant and add a same-
  fidelity adaptive excerpt only for long candidates with a strict token
  reduction; this preserves existing fallback behavior.
- 2026-09-02: Limit adaptive changes to focused mode and private line-window
  helpers; public packet, retrieval, relation, and metadata contracts remain
  frozen.

## Outcomes & Retrospective

The outcome section will be completed from the measured candidate result. A
failed gate must remain a negative milestone and must not be described as a
quality baseline.
