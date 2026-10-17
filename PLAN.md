# Preserve deletion-only diff boundaries

## Purpose / Big Picture
A deletion inside a surviving function must remain discoverable by Git impact and changed-only context queries, even when the hunk has zero added lines. Return indexed source with its real locations and retain existing Evidence budgets/invariants.

## Progress
- [x] 2026-10-17 UTC: Inspected Git hunk parser and overlap consumers. Parser discards all new-count-zero hunks, including removals inside surviving functions.
- [x] 2026-10-17 UTC: Observed RED for middle/start/EOF parser anchors and all three real-Git states. Legacy/Evidence impact and unstaged/staged changed-only queries returned no function. Empty/full-file deletion and insertion controls passed.
- [x] 2026-10-17 UTC: Kept actual deletion-only boundaries as one-based point ranges and retained full-deletion/empty-hunk behavior. New and existing focused parser/service regressions passed, including exact source/locations and measured Packet budgets/invariants.
- [x] 2026-10-17 UTC: Full verification and independent review passed. Scoped fix committed as 7347051 and fast-forward merged into master. Deleted the merged task branch; only local master remains.
- [x] 2026-10-17 UTC: Full go test ./..., go test -race ./..., go vet ./..., native build and all three CGO-disabled cross-builds passed. Independent read-only review found no actionable issues. Focused tests include explicit packet wire measurement/invariants and exact source fidelity.

## Surprises & Discoveries
Git zero-length new ranges use the preceding surviving line (or zero at file start). A boundary anchor can select a surviving enclosing chunk without reconstructing removed source. File deletions and truly empty hunks must retain empty current-source ranges.

## Decision Log
- 2026-10-17 UTC: Limit this continuation to deletion-only hunks within current source. Preserve user-owned edits and the completed predecessor. No network, dependencies, fixture code execution or schema changes.
- For old-count-positive/new-count-zero hunks in a surviving file, emit a point range at max(1, new-start). Keep fully deleted files and old/new-count-zero hunks empty. Do not add packet fields or synthesize removed source.
- Verify new context results with explicit wire measurement, packet validation and exact current fixture source, since candidate inclusion changes Evidence behavior.

## Outcomes & Retrospective
Deletion-only changes now retain a conservative surviving boundary instead of disappearing from range-based lookup. New public-service regressions observed the old failure then passed with exact current fixture bytes and positions. No packet fields or deleted source synthesis were introduced. Existing full-file deletion and empty-hunk behavior remains covered. User-owned tracked edits retain patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. Full verification and independent review passed; 7347051 is merged into master with the same verified code tree. The merged task branch was removed. No push or runtime deployment was performed. This final update is documentation-only.

## Context and Orientation
internal/gitx/diff.go currently keeps ranges only when the new line count is positive. internal/app changed-only and impact paths select indexed chunks by overlap with these ranges. Empty zero-change diff behavior is already tested.

## Plan of Work
1. Add hunk cases for middle/start/EOF deletion anchors, implicit old count, full deletion and zero changes. Add real-Git unstaged/staged and committed-ref impact cases; verify legacy/evidence query results and source fidelity/wire budgets.
2. Parse old hunk count and preserve a one-line point anchor only for actual deletion in surviving files. Keep all other parsing behavior.
3. Run focused/full tests, full race tests with 64-bit UCRT GCC, vet, native build and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 cross-builds. Review, commit explicit paths and merge locally.

## Validation and Acceptance
Observe RED before implementation. Real fixture changes remove statements from a valid Go function, then index the surviving source. Require the correct function from changed-only and impact, exact source-mode bytes/locations, valid packets and measured wire usage within the declared budget. A fully deleted file must not become current source merely through a boundary anchor. Cross-builds do not imply non-Windows runtime execution.

## Idempotence and Recovery
Temporary Git fixtures only. Preserve dirty user files. Archive predecessor byte-for-byte at docs/superpowers/plans/completed/2026-10-17-sqlite-pool-configuration.md. No push or runtime deployment.

## Interfaces and Dependencies
No public wire/type/flag changes. Existing internal LineRange can represent a point boundary; only Git hunk extraction changes.
