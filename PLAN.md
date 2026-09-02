# FocalSpan v0.12 Anchor-First Evidence Packing Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement this
> plan task-by-task. Keep the RED/GREEN verification order and update this
> file with actual results.

**Goal:** Preserve exact, path, and relation anchors inside the existing Evidence
packing budget so useful evidence increases without changing retrieval, ranking,
wire schemas, or public MCP/CLI interfaces.

**Architecture:** `internal/evidence.Compiler` will perform one bounded anchor
reservation pass over the already ranked candidates, then use the existing
utility, role, mode, and variant logic for the remaining slots. Reservation is
internal only: candidates keep their existing model fields and final packet
ordering remains deterministic. If an anchor cannot fit verbatim, existing
elision/signature variants are tried before the anchor is omitted.

**Tech Stack:** Go, SQLite-backed model candidates, existing deterministic token
estimator, `internal/evidence` packet compiler, and the source-free historical
benchmark.

**Spec:** User-approved “FocalSpan token効率改善候補・優先順位”, rank 2
`anchor-first Evidence packing`.

## Global Constraints

- Do not change retrieval, RRF/ranking weights, query planning, Evidence schema,
  MCP/CLI methods, `known_handles`, or SQLite schema.
- Preserve source fidelity, relation provenance, deterministic ordering, and
  the hard serialized budget for every packet.
- Reserve only exact symbol, explicit path, and relation anchors; generic
  lexical, documentation, and configuration candidates remain governed by the
  existing filters.
- Keep the development attribution/diagnosis reports source-free and outside
  normal MCP/CLI output.
- Run one candidate benchmark after static verification; retry only a clear
  infrastructure failure once, and never promote an unmeasured result.
- Preserve user-owned dirty files (`AGENTS.md`, `.focalspan.json`, `TASKS.md`)
  and stage explicit paths only.

## Context and Orientation

- `internal/budget/budget.go` owns legacy `model.ContextBundle` packing and
  remains a comparison path; it is not changed in this milestone.
- `internal/evidence/compiler.go` owns the public Evidence packet and assigns
  roles, variants, relations, guidance, and budget-aware selection. The
  reservation pass is limited to this compiler path.
- `internal/benchmark` measures packed labels, cumulative wire tokens, and
  source-free attribution/diagnosis; the frozen focused/2048 baseline is
  `packing_dropped=7`, packed labels `5`, cumulative wire `12,740`, and useful
  evidence efficiency `0.3925`.

## Plan of Work

### Task 0: Freeze transition

- [x] Archive v0.11 byte-identically at
  `docs/superpowers/plans/completed/2026-09-02-v0.11-identity-bridge-retrieval.md`.
- [x] Replace the root plan with this v0.12 plan before product edits.
- [x] Preserve dirty user files and make the transition documentation-only.

### Task 1: RED tests for anchor reservation

**Files:** `internal/evidence/compiler_test.go`.

- [x] Add a failing test where a lower-ranked exact-symbol anchor appears after
  a large relation/source candidate and must remain in the packed bundle when
  the current tail-trim would otherwise remove it.
- [x] Add a failing test where a relation anchor and its relation candidate are
  both retained, with no dangling public relation edge and stable item order.
- [x] Add a failing test proving generic lexical/documentation candidates are
  not treated as reserved anchors.
- [x] Add a failing test for tight-budget fallback: an anchor may be elided or
  reduced to its signature, but must not exceed the serialized budget or alter
  source bytes that remain verbatim.
- [x] Run only the new tests and record the expected RED failure before editing
  `internal/evidence/compiler.go`; the initial cache-path failure was retried
  with a repository-local cache, after which all four tests failed as expected.

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/compiler.go` and the tests from Task 1.

- [x] Introduce a private deterministic anchor classifier. It returns true for
  candidates with `symbol-exact`, a `path` score reason, or a non-empty
  relation/relation context; it returns false for documentation, configuration,
  and generic lexical candidates.
- [x] Build a stable reservation set keyed by candidate handle before the main
  packing loop, preserving the existing ranked order for output.
- [x] During budget pressure, prefer the reserved anchor over non-reserved
  candidates and try the existing content elision/signature fallback before
  incrementing `OmittedCount` for the anchor.
- [x] Keep relation metadata and existing duplicate-relation elision unchanged;
  never emit a relation edge whose endpoints are absent from the final bundle.
- [x] Run the focused Evidence tests, then the full Go package suite.

### Task 3: Static verification and candidate gate

- [x] Run `gofmt` on changed Go files and `git diff --check`.
- [x] Run `go test ./... -count=1` and `go vet ./...`.
- [x] Run native plus CGO-free Windows amd64, Linux amd64, and Darwin arm64
  builds into a temporary directory; remove outputs after verification.
- [x] Run the historical suite exactly once with `default`, `repeat 1`,
  attribution, and diagnosis enabled.
- [x] Accept only if focused/2048 reduces `packing_dropped` by at least three,
  keeps packed labels at least `5`, keeps wire `<=12,740`, improves efficiency
  strictly above `0.3925`, and preserves all fidelity, relation, budget,
  deterministic-ordering, known-handle, and MCP contract checks.
- [x] Record measured values, artifact hashes, and privacy scan in
  `docs/benchmarks/findings-v0.12.md` without source text or absolute paths.

### Task 4: Closure and recovery

- [x] If the gate fails, reverse-revert only the v0.12 product commit(s), retain
  the negative findings, and leave the v0.10 baseline intact.
- [x] Evaluate the pass branch; it was not taken because the strict wire gate
  failed, so no new measured baseline was recorded.
- [x] Update this plan with UTC progress, discoveries, decisions, outcomes, and
  actual verification evidence; do not modify archived plans.
- [x] Remove generated reports, indexes, binaries, caches, and temporary
  workspaces; preserve user-owned dirty/untracked files.

## Validation and Acceptance

Every packed bundle must remain within its serialized token budget, preserve
source bytes for verbatim content, preserve valid relation endpoints, and remain
deterministic for identical input. Existing explicit path/symbol behavior,
known-handle suppression, normal MCP output, and `focalspan.context.v1` bytes
must remain unchanged. The strict candidate gate requires at least three fewer
focused/2048 packing drops, no loss of packed labels, no wire increase, and
strictly higher useful-evidence efficiency.

## Idempotence and Recovery

Packing is read-only and repeatable. Reservation uses only in-memory candidate
identity and does not persist state. A failed candidate is recovered with
ordinary reverse-order `git revert`; source-free findings remain as historical
evidence. Race testing may be recorded `UNVERIFIED` only for the known local
MinGW compiler limitation.

## Interfaces and Dependencies

No public interface changes. `model.PackRequest`, `model.ContextBundle`,
Evidence packet schemas, MCP methods, CLI output, and `known_handles` remain
unchanged. The private classifier and reservation helpers are consumed only by
`Packer.Pack`; benchmark attribution sees ordinary existing candidate identities
and no new public debug fields. The private classifier and reservation helpers
were consumed only by `Compiler.Compile` and were removed with the reverted
candidate.

## Progress

- [x] 2026-09-02: v0.11 archive SHA-256 matches the active plan; user-owned
  files remain unstaged.
- [x] 2026-09-02: v0.12 design approved; transition prepared for RED tests.
- [x] 2026-09-02T01:20Z: Evidence RED tests reproduced four packing failures;
  the repository-local cache was required after the default cache was denied.
- [x] 2026-09-02T01:28Z: Minimal reservation implementation passed focused
  Evidence tests and the full Go suite.
- [x] 2026-09-02T01:32Z: vet, diff check, native/cgo-free builds, and the one
  historical candidate run completed; race remained unverified by MinGW.
- [x] 2026-09-02T01:46Z: Gate failed on cumulative wire growth; temporary
  product commit `ab62461` was reverted by `43ca5fc`, and findings were saved.

## Surprises & Discoveries

- The legacy packer currently removes final items from the tail when the
  serialized bundle exceeds budget, so a later anchor can be lost even after
  its content has been accepted.
- Existing relation duplicate elision changes content only; relation endpoint
  validity must remain a separate invariant while reservation is added.
- The public Evidence compiler is the active packing path; changing the legacy
  budget packer would not affect the benchmark gate.
- Reserving every structural candidate before utility selection reduced focused
  packing drops from 7 to 4, but increased cumulative wire from 12,740 to
  27,667 because additional Evidence items and their metadata were admitted.

## Decision Log

- 2026-09-02: Proceed to rank 2 only after v0.11 identity-bridge rejection;
  retrieval and packing remain separate milestones.
- 2026-09-02: Reserve exact symbol, explicit path, and relation candidates by
  handle; do not reserve broad symbol-prefix or lexical hits.
- 2026-09-02: Preserve ranked output order and use existing elision/signature
  variants rather than introducing a new public packet field.
- 2026-09-02: Treat the candidate as rejected despite recall/efficiency gains
  because the strict no-wire-growth gate is non-negotiable; retain v0.10 as the
  active baseline and do not claim a v0.12 quality baseline.

## Outcomes & Retrospective

v0.12 is closed as a negative candidate. The reservation invariant and
regression tests were verified locally, but the product candidate was reverted
after the single measured gate: packing drops improved (7→4) and efficiency
improved (0.3925→0.6506), while cumulative wire exceeded the limit
(12,740→27,667). A future packing attempt must preserve the baseline wire
denominator or replace content with measured compact variants before rerunning
the historical gate.
