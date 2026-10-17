# Evict stale source after extraction failure

## Purpose / Big Picture

Successful indexing must not expose old source from changed files whose extraction failed. Preserve per-file failure isolation and retry on subsequent updates.

## Progress

- [x] 2026-10-17 UTC: Found failed extraction skips replacement but retains old rows. Archived the completed deletion-only plan byte-for-byte.
- [x] 2026-10-17 UTC: Injected hard extractor failure; RED returned stale OldToken source after a successful new-revision run.
- [x] 2026-10-17 UTC: Evict failed paths transactionally. Focused indexer tests passed, including retry, repair, unaffected source and existing cancellation rollback.
- [ ] Full verification, independent review, scoped commit, local merge and merged branch cleanup.

- [x] 2026-10-17 UTC: Full go test ./..., go test -race ./..., go vet ./..., native and all three cross-builds passed. Independent read-only review found no actionable issues. git diff --check passed; unrelated tracked patch hash remains 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a.

## Surprises & Discoveries

Paths absent from the store are retried on the next scan. Eviction avoids stale source without artificial hashes. Partial extraction returning no error remains usable.

Ordinary malformed Go returns a partial AST rather than a hard failure; the regression uses a wrapper extractor injecting a deterministic error while delegating successful extraction to the real Go extractor.

## Decision Log

- 2026-10-17 UTC: Preserve failure isolation and ParseFailures. Use existing transactional deletion for failed indexed paths, without counting them as filesystem deletions.
- 2026-10-17 UTC: Preserve user changes and packet schemas; no network, dependency restore, fixture code execution or runtime deployment.

## Outcomes & Retrospective

Hard extraction errors now evict only the failed file's old rows using the existing update transaction. Repeated failure retries and repair recovery pass alongside unaffected-file and cancellation controls. Ordinary partial Go extraction is unchanged. Full offline verification and independent review passed. Build artifacts are under .build/extraction-failure-verification; merging and branch cleanup remain.

## Context and Orientation

internal/indexer/indexer.go gathers extraction results before Store.ApplyIndexWithDelta. Errors currently skip updates. Store deletion cascades per-file data and supplies relation invalidation. Tests use temporary fixtures in internal/indexer.

## Plan of Work

1. Reproduce valid source becoming unextractable; assert stale source eviction, repeated retry, unrelated file preservation and repair recovery.
2. Append failed existing paths to transactional deletions and sort deterministically. Preserve cancellation before writes.
3. Run verification and independent review; commit explicit paths, fast-forward merge master, remove merged task branch and record outcomes.

## Validation and Acceptance

Observe RED before production edits. Run focused tests, go test ./..., go test -race ./..., go vet ./..., native build and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 cross-builds. Use offline Go environment and C:/msys64/ucrt64/bin/gcc.exe for race. Run git diff --check and preserve unrelated patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. Final docs-only changes use scoped checks.

## Idempotence and Recovery

Failed paths retry until repaired. Cancellation before writes retains prior index. Removal uses the existing transaction. Revert only task commits if needed; retain user files and detached worktrees.

## Interfaces and Dependencies

Existing Indexer, registry, Store.ApplyIndexWithDelta and candidate APIs. No new dependencies or packet fields.
