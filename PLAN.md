# FocalSpan v0.12 Anchor-First Evidence Packing Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement this
> plan task-by-task. Keep the RED/GREEN verification order and update this
> file with actual results.

**Goal:** Preserve exact, path, and relation anchors inside the existing Evidence
packing budget so useful evidence increases without changing retrieval, ranking,
wire schemas, or public MCP/CLI interfaces.

**Architecture:** `internal/budget.Packer` will perform one bounded anchor
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

- `internal/budget/budget.go` owns legacy `model.ContextBundle` packing,
  content elision, test/documentation filters, and final JSON budget checks.
- `internal/evidence/compiler.go` owns the public Evidence packet and already
  assigns roles, variants, relations, and guidance; it must not be redesigned
  in this milestone.
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

**Files:** `internal/budget/budget_test.go` and, only if required by an
observable Evidence invariant, `internal/evidence/compiler_test.go`.

- [ ] Add a failing test where a lower-ranked exact-symbol anchor appears after
  a large relation/source candidate and must remain in the packed bundle when
  the current tail-trim would otherwise remove it.
- [ ] Add a failing test where a relation anchor and its relation candidate are
  both retained, with no dangling public relation edge and stable item order.
- [ ] Add a failing test proving generic lexical/documentation candidates are
  not treated as reserved anchors.
- [ ] Add a failing test for tight-budget fallback: an anchor may be elided or
  reduced to its signature, but must not exceed the serialized budget or alter
  source bytes that remain verbatim.
- [ ] Run only the new tests and record the expected RED failure before editing
  `internal/budget/budget.go`.

### Task 2: Minimal GREEN implementation

**Files:** `internal/budget/budget.go` and the tests from Task 1.

- [ ] Introduce a private deterministic anchor classifier. It returns true for
  candidates with `symbol-exact`, a `path` score reason, or a non-empty
  relation/relation context; it returns false for documentation, configuration,
  and generic lexical candidates.
- [ ] Build a stable reservation set keyed by candidate handle before the main
  packing loop, preserving the existing ranked order for output.
- [ ] During budget pressure, prefer the reserved anchor over non-reserved
  candidates and try the existing content elision/signature fallback before
  incrementing `OmittedCount` for the anchor.
- [ ] Keep relation metadata and existing duplicate-relation elision unchanged;
  never emit a relation edge whose endpoints are absent from the final bundle.
- [ ] Run the focused budget/evidence tests, then the full Go package suite.

### Task 3: Static verification and candidate gate

- [ ] Run `gofmt` on changed Go files and `git diff --check`.
- [ ] Run `go test ./... -count=1` and `go vet ./...`.
- [ ] Run native plus CGO-free Windows amd64, Linux amd64, and Darwin arm64
  builds into a temporary directory; remove outputs after verification.
- [ ] Run the historical suite exactly once with `default`, `repeat 1`,
  attribution, and diagnosis enabled.
- [ ] Accept only if focused/2048 reduces `packing_dropped` by at least three,
  keeps packed labels at least `5`, keeps wire `<=12,740`, improves efficiency
  strictly above `0.3925`, and preserves all fidelity, relation, budget,
  deterministic-ordering, known-handle, and MCP contract checks.
- [ ] Record measured values, artifact hashes, and privacy scan in
  `docs/benchmarks/findings-v0.12.md` without source text or absolute paths.

### Task 4: Closure and recovery

- [ ] If the gate fails, reverse-revert only the v0.12 product commit(s), retain
  the negative findings, and leave the v0.10 baseline intact.
- [ ] If the gate passes, retain the implementation and record the new measured
  baseline without changing public interfaces.
- [ ] Update this plan with UTC progress, discoveries, decisions, outcomes, and
  actual verification evidence; do not modify archived plans.
- [ ] Remove generated reports, indexes, binaries, caches, and temporary
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
and no new public debug fields.

## Progress

- [x] 2026-09-02: v0.11 archive SHA-256 matches the active plan; user-owned
  files remain unstaged.
- [x] 2026-09-02: v0.12 design approved; transition prepared for RED tests.
- [ ] Task 1 RED tests.
- [ ] Task 2 anchor-first implementation.
- [ ] Task 3 static verification and candidate gate.
- [ ] Task 4 closure and recovery.

## Surprises & Discoveries

- The legacy packer currently removes final items from the tail when the
  serialized bundle exceeds budget, so a later anchor can be lost even after
  its content has been accepted.
- Existing relation duplicate elision changes content only; relation endpoint
  validity must remain a separate invariant while reservation is added.

## Decision Log

- 2026-09-02: Proceed to rank 2 only after v0.11 identity-bridge rejection;
  retrieval and packing remain separate milestones.
- 2026-09-02: Reserve exact symbol, explicit path, and relation candidates by
  handle; do not reserve broad symbol-prefix or lexical hits.
- 2026-09-02: Preserve ranked output order and use existing elision/signature
  variants rather than introducing a new public packet field.

## Outcomes & Retrospective

Pending Task 1 RED tests and the single measured candidate gate.
