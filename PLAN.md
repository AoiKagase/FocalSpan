# Propagate explicit Git impact reference errors

## Purpose / Big Picture
Invalid explicit Git base/head references must return errors through Impact, ImpactEvidence and MCP code_impact rather than empty success.

## Progress
- [x] 2026-10-17 UTC: Inspected shared impact path; archived completed predecessor byte-for-byte.
- [ ] Add regressions and observe RED before production changes.
- [ ] Propagate Git error with its cause; observe GREEN.
- [ ] Complete verification and final review.

## Surprises & Discoveries
Explicit-reference Git errors are overwritten during candidate retrieval.

## Decision Log
- 2026-10-17 UTC: Only fix explicit-reference errors. Preserve no-reference non-Git behavior and unrelated edits. Work in the existing feature checkout as authorized by the approved plan.
- 2026-10-17 UTC: Use PLAN.md as ledger; do not execute repository helper scripts. Commit the plan transition separately from product code. No network or dependency restore.

## Outcomes & Retrospective
Pending.

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
