# Recover interrupted relation linking

## Purpose / Big Picture

After file updates commit, cancellation or failure during linking must remain recoverable on an unchanged subsequent incremental update. Cross-file calls must eventually resolve without forcing a full reindex.

## Progress

- [x] 2026-10-17 UTC: Traced file updates, manifest revision and linking across separate transactions; archived the completed extraction-failure plan byte-for-byte.
- [x] 2026-10-17 UTC: RED on initial and incremental indexing left ValidateToken calls unresolved after reopen; repeated interruption incorrectly returned success on its second attempt.
- [x] 2026-10-17 UTC: Pending marker committed with file updates; unchanged reopen recovery and repeated cancellation now pass. Full indexer suite passed, including existing pre-write cancellation tests.
- [x] 2026-10-17 UTC: Full race tests, go vet, native and three cross-builds passed. Independent read-only review found no actionable issues.
- [x] 2026-10-17 UTC: First normal full suite failed in existing TestSupervisorFailedCandidatesKeepOldWorker with worker EOF. Isolated five-repeat rerun and serial full-suite rerun both passed without source edits.
- [x] 2026-10-17 UTC: Full offline verification and independent review passed. Scoped fix a31e976 fast-forward merged into master; merged task branch removed.

## Surprises & Discoveries

File hashes and manifest revision are saved before linking. After interruption the next run has no file delta, so scoped linking skips unresolved work.

Missing recovery metadata conservatively forces one full linking pass for older indexes. Successful completion clears the marker so unchanged updates return to scoped linking; the regression checks this behavior.

Concurrent normal/race verification produced one normal-suite supervisor fixture failure (worker connection closed). The corresponding race package passed, and the isolated test passed five repetitions without source edits. Do not infer a root cause from this alone; rerun the normal suite serially and retain this evidence.

## Decision Log

- 2026-10-17 UTC: Keep a durable pending-link marker in the existing file-update transaction. Clear only after successful run finalization; a surviving marker forces full relation linking next run. No schema migration or public packet change.
- 2026-10-17 UTC: Own only PLAN.md, its predecessor archive, internal/indexer/indexer.go and a new indexer recovery test. Preserve user-owned dirty files and existing worktrees; no network, dependency restore, fixture code execution or deployment.

## Outcomes & Retrospective

Implemented a durable linking_pending marker without schema changes. Initial and incremental indexing recover after store reopen and repeated interruption; successful recovery restores scoped linking. Full normal suite (serial rerun), full race suite, go vet, native and three cross-builds passed. Independent review found no actionable issues. Initial supervisor fixture failure remains documented with isolated five-repeat success; its cause is unconfirmed. User tracked patch hash remains 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. Artifacts: .build/link-recovery-verification. Fix a31e976 is fast-forward merged into master with the same verified code tree. Merged task branch removed; user changes preserved. No push or runtime deployment performed by this task. Final completion update is documentation-only.

## Context and Orientation

Indexer.RunWithProgress applies file updates, calls LinkWithScope, finalizes the run and records last_successful_index. File updates and relation linking commit separately. Store meta can persist recovery state atomically with ApplyIndexWithDelta without new APIs.

## Plan of Work

1. Cancel deterministically at initial linking progress after file writes; reopen the store and run incremental update with unchanged files. Verify actual calls resolve.
2. Read pending-link metadata before writes, set the marker in the file transaction and force full linking if pending. Clear only after success; test normal incremental behavior and repeated interruption.
3. Run repository verification and read-only review. Commit explicit paths, fast-forward master, remove task branch and record evidence.

## Validation and Acceptance

Observe RED before product edits. Use focused indexer regressions plus go test ./..., go test -race ./..., go vet ./..., native build and CGO-disabled windows/amd64, linux/amd64 and darwin/arm64 builds. Set GOPROXY=off, GOSUMDB=off and GOTOOLCHAIN=local. Race compiler C:/msys64/ucrt64/bin/gcc.exe. Verify git diff --check and unrelated tracked patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. Final documentation-only changes use scoped checks.

## Idempotence and Recovery

The writer lease serializes marker and linking operations. Pending full relinking is safe to retry; failed/cancelled runs retain the marker, including after process/store restart. An interrupted marker clear may cause an extra full linking pass, never skipped recovery. Existing pre-write cancellation must remain unchanged. Revert only scoped task commits if needed.

## Interfaces and Dependencies

Existing Store.Meta/SetMeta, ApplyIndexWithDelta metadata, LinkScope.Full and progress callback. No new dependencies, schema versions or packet fields.
