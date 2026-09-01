# FocalSpan Packer Efficiency v0.10 Implementation Plan

> **For agentic workers:** Execute task-by-task. Every production behavior
> starts with a failing deterministic test, then RED confirmation, minimal
> GREEN implementation, focused verification, and an explicit-path commit.

**Goal:** Reduce Codex/MCP wire-token cost by making Evidence packing more
efficient while preserving every source-fidelity, budget, known-handle, and
public MCP contract.

**Architecture:** Keep retrieval and ranking unchanged in v0.10. Add an
opt-in, source-free packing observation path for development attribution, then
test one bounded packer candidate: omit duplicate or fully contained spans when
they add no new symbol/relation identity, while retaining exact-symbol and
relation anchors. The existing `Pack` API remains the production entry point;
the observation path is an internal diagnostic wrapper.

**Tech Stack:** Go 1.27, standard library, existing `internal/budget`,
`internal/model`, `internal/evidence`, and `internal/benchmark` packages, and
the existing historical benchmark harness. No new dependency, database schema,
MCP method, or Evidence schema.

**Spec:** User-approved v0.10 direction recorded in this plan; the frozen v0.9
negative findings remain at `docs/benchmarks/findings-v0.9.md` and the v0.9
archive remains immutable.

## Global Constraints

- Preserve user-owned dirty `AGENTS.md`, untracked `.focalspan.json`, and
  untracked `TASKS.md`; stage only intended paths.
- Keep at most one active root `PLAN.md`; archive the completed v0.9 plan
  byte-for-byte before this transition.
- Keep `focalspan.context.v1`, `focalspan.benchmark-attribution.v1`, all MCP
  tool names/instructions, `known_handles`, Evidence fidelity, and normal CLI
  output unchanged.
- Do not change query normalization/planning, search retrievers, relation
  linking, ranking weights, SQLite schema, or token-estimator constants.
- Development observations are opt-in, source-free, and excluded from normal
  MCP, Evidence, and quality JSON responses.
- Every production behavior starts with a deterministic RED test and a recorded
  RED confirmation before the minimal GREEN implementation.
- Run one candidate benchmark per hypothesis only after static verification;
  retry only an infrastructure failure once and record the cause.
- Do not use network access, external LLMs, embeddings, SCIP, repository-code
  execution, package restore, or runtime dependencies.
- Do not use `git reset`, `git restore`, `git checkout --`, `git clean`, or
  `git stash`.

## Purpose / Big Picture

The v0.8 attribution run classified 95 historical labels: 55
`retrieval_missing`, 35 `packing_dropped`, and 5 `packed`. In focused/2048,
there were 9 `path_scope_missing`, 2 `symbol_match_missing`, and 7
`packing_dropped` labels. v0.9's file-scope aggregation left path misses at
9, packed labels at 5, increased cumulative estimated wire tokens from 12,740
to 13,028, and reduced useful evidence per 1,000 tokens from 0.3925 to 0.3838.
Its production candidate was reverted and is not a quality baseline.

v0.10 therefore measures where packing spends tokens before changing it. The
candidate is intentionally limited to repeated or contained evidence spans
that do not add a new identity. It must reduce the denominator without hiding
source text required by an exact symbol, relation, expansion anchor, or
`known_handles` delta.

The development metric remains:

`1000 * unique packed required evidence labels / sum of final serialized
estimated tokens for code_context and executed code_expand responses`.

Required labels are unique packed paths, symbols, and successfully executed
expansion expectations. Internal observations never enter normal MCP output.

## Context and Orientation

- `internal/budget/budget.go`: `Packer.Pack`, source-payload estimation,
  budget elision, test/documentation policy, and current low-utility filtering.
- `internal/budget/budget_test.go`: deterministic packer behavior and budget
  regressions.
- `internal/model/model.go`: `PackRequest`, `RankedCandidate`,
  `ContextBundle`, and `ContextItem` compatibility shapes.
- `internal/app/evidence.go`: production route from search results to Evidence
  compilation; do not alter this route's public request or response contract.
- `internal/evidence/{compiler,wire,validate}_*.go`: packet serialization,
  fidelity, budget, and known-handle invariants.
- `internal/benchmark/{efficiency,attribution,runner}.go`: source-free label
  accounting, cumulative wire-token accounting, and opt-in attribution.
- `internal/benchmark/efficiency_test.go` and
  `internal/benchmark/attribution_test.go`: development-only metric and
  privacy/compatibility coverage.
- `docs/benchmarks/findings-v0.9.md`: measured negative evidence and baseline
  comparison.

## Interfaces and Dependencies

The public `Packer.Pack(req model.PackRequest) model.ContextBundle` signature
and all model JSON fields remain unchanged. Add only an internal diagnostic
surface in `internal/budget`:

```go
type PackObservation struct {
	Handle                string
	Path                  string
	Symbol                string
	CandidateTokens       int
	SerializedDeltaTokens int
	Packed                bool
	DropReason            string
	ContainedByHandle     string
}

func (p *Packer) PackWithObservations(req model.PackRequest) (model.ContextBundle, []PackObservation)
```

`Pack` delegates to the same implementation with observations discarded, so
existing callers and serialized bundles remain unchanged. `PackObservation`
contains identity and counts only; it must never contain source content,
absolute paths, user names, secrets, or environment values. Benchmark code may
join these observations with attribution identities without changing
`focalspan.benchmark-attribution.v1`.

## Plan of Work

### Task 0: Transition and Freeze the v0.9 Baseline

**Files:**

- Create: `docs/superpowers/plans/completed/2026-09-01-v0.9-file-scope-aggregation.md`
- Modify: `PLAN.md`

- [x] Confirm the v0.9 archive is byte-identical to the prior root plan. The
  archive comparison must report no differences before this plan is edited.
- [x] Record the frozen baseline in this plan: focused/2048 path misses 9,
  packed labels 5, cumulative estimated wire tokens 12,740, and useful
  evidence per 1,000 tokens 0.3925.
- [x] Verify `git status --short --branch` and preserve the pre-existing dirty
  `AGENTS.md`, `.focalspan.json`, and `TASKS.md`.
- [x] Commit only the plan transition and immutable archive with message
  `docs: start packer efficiency v0.10`.

### Task 1: Add Source-Free Packing Observations (RED/GREEN)

**Files:**

- Modify: `internal/budget/budget.go`
- Test: `internal/budget/budget_test.go`
- Modify: `internal/benchmark/efficiency.go`
- Test: `internal/benchmark/efficiency_test.go`
- Test: `internal/benchmark/attribution_test.go`

**Interfaces:**

- Consumes: existing `model.PackRequest`, `model.RankedCandidate`, and
  `model.ContextBundle`.
- Produces: `budget.PackObservation` and deterministic observation lists for
  development-only benchmark attribution.

- [ ] Write RED tests proving that `PackWithObservations` reports one row per
  candidate in stable input order, reports candidate and serialized-delta
  token counts, and records an explicit reason for every omitted candidate.
- [ ] Write RED tests proving that observations contain no source content,
  absolute paths, secrets, or user/environment values.
- [ ] Write RED tests proving `Pack(req)` and
  `PackWithObservations(req)` return byte-identical `ContextBundle` JSON for
  the same request, including `known_handles`-driven Evidence compilation.
- [ ] Run the focused RED tests and record the expected failure before adding
  implementation.
- [ ] Implement the observation wrapper around the existing pack loop without
  changing budget clamping, elision, item order, or omission behavior.
- [ ] Add benchmark-side aggregation that consumes observations only when the
  existing opt-in attribution path is enabled; normal quality and MCP output
  must ignore them.
- [ ] Run focused budget/benchmark/evidence tests and commit
  `feat: add source-free packing observations`.

### Task 2: Implement the Single Bounded Packer Candidate (RED/GREEN)

**Files:**

- Modify: `internal/budget/budget.go`
- Test: `internal/budget/budget_test.go`
- Test: `internal/evidence/wire_test.go`
- Test: `internal/evidence/known_test.go`
- Test: `internal/app/evidence_test.go`

**Interfaces:**

- Consumes: ranked candidates in their existing order and the observation
  helper from Task 1.
- Produces: the same `model.ContextBundle` shape with fewer redundant source
  spans only when the candidate is not an exact-symbol or relation anchor.

- [ ] Write RED tests for identical content hashes on one path, fully
  contained same-path spans, exact-symbol anchors, relation anchors, and
  separate expansion expectations.
- [ ] Write RED tests proving a contained candidate is retained as a
  signature-only item when it supplies a distinct exact symbol or relation
  identity, and is omitted only when it supplies no new identity.
- [ ] Write RED tests proving source fidelity, item order, budget limits,
  `known_handles` suppression, and packet-local handles remain valid.
- [ ] Run the focused RED tests and record the failure.
- [ ] Add deterministic duplicate/containment checks inside the existing pack
  loop. Compare repository-relative path and valid line spans; preserve the
  first ranked candidate, exact-symbol reasons, relation candidates, and any
  candidate needed by an expansion expectation. Do not alter ranking scores,
  query plans, or token-estimator constants.
- [ ] Record `duplicate_span` or `contained_without_new_identity` in the
  observation rather than exposing a new diagnostic field in normal output.
- [ ] Run focused packer, Evidence wire, known-handle, and app compatibility
  tests; commit `feat: compact redundant evidence spans`.

### Task 3: Static Verification and the Frozen Candidate Gate

**Files:**

- Create: `docs/benchmarks/findings-v0.10.md`
- Modify: `PLAN.md`

- [ ] Run formatting on changed Go files and `git diff --check`.
- [ ] Run `go test ./... -count=1`, `go vet ./...`, and the Evidence/MCP,
  privacy, finite-number, forbidden-path, deterministic, budget, source
  fidelity, and known-handle tests. Record actual counts and failures.
- [ ] Run CGO-free native, Windows amd64, Linux amd64, and Darwin arm64 builds
  into an ignored temporary directory and remove generated files afterward.
- [ ] If static verification passes, run the eight-case
  `full-evidence-focused`/2048 candidate exactly once. Retry only when the
  harness itself fails to produce a result; record that infrastructure cause
  before the one retry.
- [ ] Accept the packer candidate only when cumulative wire tokens are no
  greater than 12,740, useful evidence per 1,000 tokens is strictly greater
  than 0.3925, no existing packed label is lost, and all source-fidelity,
  budget, deterministic, privacy, Evidence/MCP, and `known_handles`
  invariants remain valid.
- [ ] For this packer-only milestone, retrieval counts may remain unchanged;
  a search improvement is not claimed from a packing result. Record the
  observed `packing_dropped` and packed-label deltas separately.
- [ ] Record baseline, candidate, gate decision, artifact hashes, privacy
  scan, and any infrastructure retry in
  `docs/benchmarks/findings-v0.10.md` without source text or absolute paths.
- [ ] If the gate fails, revert only the two candidate commits in reverse
  order, retain the negative findings, and do not promote a new baseline.

### Task 4: Closure and Handoff to the Next Retrieval Hypothesis

**Files:**

- Modify: `PLAN.md`
- Modify: `docs/benchmarks/findings-v0.10.md`

- [ ] Run the final targeted verification after any accepted/reverted result
  and confirm no generated reports, indexes, binaries, caches, or temporary
  workspaces remain.
- [ ] Keep local MinGW race testing explicitly `UNVERIFIED` if the compiler
  limitation recurs; do not report it as a pass or failure of the candidate.
- [ ] Complete Progress, Surprises & Discoveries, Decision Log, Outcomes &
  Retrospective, and recovery notes with UTC timestamps and actual evidence.
- [ ] If v0.10 passes, create a separate successor plan for the identity-bridge
  retrieval hypothesis. That plan must measure exact/qualified symbol hints
  independently and must not co-tune retrieval, ranking, packing, and
  Evidence. If v0.10 fails, preserve the negative result before planning the
  retrieval successor.
- [ ] Commit documentation separately from product code with an explicit-path
  commit after the candidate decision.

## Validation and Acceptance

- `PackWithObservations` is deterministic, bounded, source-free, and does not
  alter the `Pack` result or any public JSON bytes.
- Duplicate/contained-span compaction never removes an exact-symbol anchor,
  relation anchor, expansion expectation, or `known_handles` delta.
- Evidence source fidelity, packet schema, wire budget, finite values,
  forbidden-path privacy, deterministic ordering, and MCP method contracts pass.
- The frozen v0.10 gate passes only when wire tokens do not increase and
  useful evidence per 1,000 tokens strictly improves over 0.3925, with no
  existing packed label or invariant regression.
- `go test ./... -count=1`, `go vet ./...`, `git diff --check`, and the four
  CGO-free build targets are recorded with actual results. Race status remains
  separate and explicitly verified or unverified.

## Idempotence and Recovery

- Observation collection is read-only and can be repeated without changing
  the index, search state, or public response.
- Candidate benchmark artifacts are hashed before cleanup; a completed result
  is not rerun to seek a better number.
- An infrastructure retry is allowed once only after its cause is recorded;
  code/test failures require a new RED test or a revised successor plan.
- A failed candidate uses normal reverse-order `git revert`; the v0.9 archive
  and v0.10 negative findings remain immutable historical evidence.
- Existing user-owned dirty files are never staged or changed by this plan.

## Progress

- [x] `2026-09-01` v0.9 candidate rejected and reverted; negative findings
  recorded in `docs/benchmarks/findings-v0.9.md`.
- [x] `2026-09-01` v0.9 root plan archived byte-for-byte before transition.
- [x] `2026-09-01` v0.10 design approved: measure packing cost first, then run
  one bounded duplicate/containment candidate; defer identity retrieval to a
  successor plan.
- [x] `2026-09-01` plan transition committed as `6c248d2`; only `PLAN.md` and
  the immutable v0.9 archive were staged.
- [ ] v0.10 Task 1 source-free packing observations.
- [ ] v0.10 Task 2 bounded packer candidate.
- [ ] v0.10 Task 3 static verification and frozen candidate gate.
- [ ] v0.10 Task 4 closure and successor handoff.

## Surprises & Discoveries

- v0.9 showed that widening file scope can increase serialized token cost
  without advancing any measured evidence label; file aggregation is not a
  substitute for symbol identity.
- The current packer already applies structural, test, documentation, and
  budget policies, so v0.10 must measure the actual omission/duplication cost
  before changing another policy.
- The existing attribution and efficiency infrastructure is the safest place
  to inspect token economics because it is already source-free and opt-in.

## Decision Log

- **2026-09-01 / user and Codex:** Optimize useful evidence per Codex/MCP token
  before the separate schema-v2 update-latency task in `TASKS.md`.
- **2026-09-01 / user and Codex:** Keep one hypothesis per candidate; v0.10 is
  packing only, while identity-bridge retrieval is deferred to v0.11.
- **2026-09-01 / Codex:** Retain the v0.9 baseline gate of 12,740 cumulative
  tokens and 0.3925 useful evidence per 1,000 tokens; no candidate may trade
  more wire tokens for unchanged labels.
- **2026-09-01 / Codex:** Development observations remain outside normal MCP,
  Evidence, quality JSON, and `focalspan.context.v1` bytes.

## Outcomes & Retrospective

This section is completed during Task 4 with measured candidate results. It
must state whether v0.10 was accepted or reverted, the actual token and label
deltas, invariant results, and any race/build limitations. A rejected candidate
is a valid outcome and does not become a quality baseline.
