# Parallel index sessions (parallel-index-locks)

## Purpose / Big Picture
Allow independent CLI/MCP sessions to share a local index without stale scans overwriting later updates or migration removing an in-use database. Contention waits at most five seconds and respects cancellation.

## Progress
- [x] 2026-10-15 UTC: Inspected Store, Indexer, MCP query flow and previous work. Archived previous plan; reconnect UAT remains unverified.
- [ ] Write and observe failing behavioral tests.
- [ ] Implement process locks, lifecycle protection and serialized indexing.
- [ ] Verify contention, process death, migration and error recovery.
- [ ] Run full tests, race, vet, build and three CGO-disabled cross-builds; review diff.

## Surprises & Discoveries
- Store opens ran migration SQL even for existing current-schema databases.
- Upgrade recovery deletes fixed sibling artifacts; it requires exclusive lifecycle protection before recovery.
- Existing root edits are completed MCP guidance work and must remain intact.
- Git ref writes require sandbox escalation; no residual lock was removed.

## Decision Log
- 2026-10-15 UTC: Use existing x/sys OS locks, persistent files, shared lifetime leases and a separate writer lock. No new downloaded dependencies.
- 2026-10-15 UTC: Five-second cap; earlier context deadline/cancellation wins. No stale-result fallback.
- 2026-10-15 UTC: Preserve checkout edits; use codex/parallel-index-locks and explicit staging paths. Predecessor is superseded with unfinished reconnect UAT recorded.

## Outcomes & Retrospective
Implementation in progress; verification pending.

## Context and Orientation
internal/store opens index.db and rebuilds legacy indexes via index.db.v2.tmp. internal/indexer scans, computes deltas, writes, links and records completion. internal/app finalizes upgrades after indexing; MCP automatic updates use that flow.

## Plan of Work
1. Add deterministic lock/process tests and a migration exclusion regression; observe RED.
2. Add internal/indexlock Acquire(ctx, directory, kind, mode), typed contention errors and idempotent release. Use a fresh OS file handle per acquisition; never delete lock files.
3. Acquire shared lifecycle before SQLite opens and exclusive lifecycle for creation/recovery/upgrades. Recheck after switching modes; protect upgrade cleanup/finalization. Close DB before releasing the lease.
4. Acquire writer before scanning and hold through linking/final metadata; rescan after waiting. Keep public CLI/MCP and Evidence Packet shapes unchanged.
5. Test and review; record actual outcomes here.

## Validation and Acceptance
Cover shared readers, exclusive writers, independent directories, same-process contention, cancellation/timeouts, waiter rescan, process death, migration artifact preservation, recovery and release on error. Use subprocess test helpers, not executed fixture code. Run go test ./..., go test -race ./..., go vet ./..., go build ./cmd/focalspan and CGO_ENABLED=0 cross-builds for windows/amd64, linux/amd64, darwin/arm64. No network or package restore. Distinguish native execution from cross-compilation.

## Idempotence and Recovery
OS releases locks at process death. Lock files remain. Existing Store handles block migration; timeout directs users to close/restart sessions. Acquisition failure must not alter index artifacts. Revalidate schema after reacquiring shared access. Preserve unrelated edits and other worktrees.

## Interfaces and Dependencies
Internal kinds: lifecycle and writer; modes: shared and exclusive. Public flags/wire unchanged. Local filesystems only; older binaries do not participate. Source edits are not a repository snapshot. Existing golang.org/x/sys may become direct; no new modules.
