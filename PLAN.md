# Propagate explicit Git impact reference errors

## Purpose / Big Picture
Invalid explicit Git base/head references must return errors through Impact, ImpactEvidence and MCP code_impact rather than empty success.

## Progress
- [x] 2026-10-17 UTC: Inspected shared impact path; archived completed predecessor byte-for-byte.
- [x] 2026-10-17 UTC: Observed RED: four invalid-ref service cases returned success; MCP also returned a successful empty packet. Valid-reference controls passed. Fixture Git init required normal permissions after sandbox denied config writes.
- [x] 2026-10-17 UTC: Added immediate wrapped error return; service regressions and non-Git compatibility test passed. MCP regression passed in a dedicated run.
- [x] 2026-10-17 UTC: Full tests, full race rerun, vet, native build and all three CGO-disabled cross-builds passed. Independent read-only review found no actionable issues.
- [ ] Commit the fix, integrate latest master, and verify the merged tree.

## Surprises & Discoveries
Explicit-reference Git errors are overwritten during candidate retrieval.
- Native full tests and vet passed. Initial race execution selected 32-bit MinGW because nested cmd environment settings did not reach Go; explicit PowerShell environment verification selected x86_64 UCRT GCC. The first correctly configured full race run hit an existing TestConcurrentInitialOpenKeepsReadersAlive SQLite lock error (261); the isolated test passed ten repeats. Full race rerun pending.
- All three CGO-disabled cross-builds passed with normal permissions. Sandbox-only attempts emitted module-cache access-denied warnings.
- master advanced to d4c1ab9 (MCP supervisor) during this task; integrate its changes and archive its completed PLAN before final master verification.

## Decision Log
- 2026-10-17 UTC: Only fix explicit-reference errors. Preserve no-reference non-Git behavior and unrelated edits. Work in the existing feature checkout as authorized by the approved plan.
- 2026-10-17 UTC: Use PLAN.md as ledger; do not execute repository helper scripts. Commit the plan transition separately from product code. No network or dependency restore.

## Outcomes & Retrospective
The explicit-reference fix and regressions passed pre-integration verification. The one transient store initialization failure is recorded above; isolated ten-repeat testing and the subsequent full race run passed. No store code was changed. Master integration remains pending.

## Context and Orientation
internal/app/evidence.go shares impact retrieval between service APIs. MCP already converts service failures to tool errors.

## Plan of Work
1. Add real local Git fixture tests for invalid base/head and valid refs through both service APIs; observe failing error cases.
2. Immediately check explicit-reference Diff errors and wrap using read Git changes and %w. Test MCP propagation.
3. Run focused/full tests, race, vet, native build and CGO-disabled cross-builds; review task changes.

## Validation and Acceptance
Invalid refs preserve exec.ExitError; valid refs yield changed evidence; existing non-Git/no-ref behavior passes. Full tests cover Evidence wire budgets and invariants. Disable module downloads and toolchain downloads. Cross-build Windows amd64, Linux amd64 and Darwin arm64 without claiming target runtime verification.

## Idempotence and Recovery
Temporary Git test fixtures never execute source code. Preserve unrelated edits. Archived predecessor: docs/superpowers/plans/completed/2026-10-15-parallel-index-locks.md.

## Interfaces and Dependencies
No new dependencies, public types or wire formats. Git failures use existing error channels.
