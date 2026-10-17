# Parallel index sessions (parallel-index-locks)

## Purpose / Big Picture
Allow independent CLI/MCP sessions to share a local index without stale scans overwriting later updates or migration removing an in-use database. Contention waits at most five seconds and respects cancellation.

## Progress
- [x] 2026-10-15 UTC: Inspected Store, Indexer, MCP query flow and previous work. Archived previous plan; reconnect UAT remains unverified.
- [x] 2026-10-15 UTC: Observed RED for live-Store migration, scan-before-lock, cancellation progress and concurrent initialization.
- [x] 2026-10-15 UTC: Implemented process locks, lifecycle protection and serialized indexing.
- [x] 2026-10-15 UTC: Focused indexlock, store, indexer, app and mcpserver tests passed outside sandbox, including process death, migration artifacts and error recovery.
- [x] 2026-10-15 UTC: Final go test ./..., go test -race ./..., go vet ./..., native build and all three CGO-disabled cross-builds passed. git diff --check passed; read-only review had no actionable findings.

## Surprises & Discoveries
- Store opens ran migration SQL even for existing current-schema databases.
- Upgrade recovery deletes fixed sibling artifacts; it requires exclusive lifecycle protection before recovery.
- Existing root edits are completed MCP guidance work and must remain intact.
- Git ref writes require sandbox escalation; no residual lock was removed.
- Windows sandbox rejects DB rename and Git fixture configuration writes. Focused tests passed when rerun outside sandbox. Putting fixtures inside ignored .build also makes repository scanners intentionally skip them; final tests use the normal temp location outside sandbox.
- Concurrent initializers can queue for exclusive access after another initializer has completed and retained shared access. Missing-DB openers periodically reacquire shared access and recheck state rather than timing out behind healthy readers.
- Final review found a same-Service gap between Indexer writer release and upgrade finalization. A deterministic paused-finalization test reproduced ignored cancellation and scan entry; a context-aware Service gate now covers indexing plus finalization.
- Default C:\MinGW GCC is 32-bit-only and cannot build Go's amd64 race runtime. Existing C:\msys64\ucrt64\bin\gcc.exe reports x86_64-w64-mingw32; verification selects it without installation.
- Service-local and OS-writer acquisition share one five-second wait budget via context values. Only acquisition contexts expire; successful long indexing operations retain the caller's original execution deadline.

## Decision Log
- 2026-10-15 UTC: Use existing x/sys OS locks, persistent files, shared lifetime leases and a separate writer lock. No new downloaded dependencies.
- 2026-10-15 UTC: Five-second cap; earlier context deadline/cancellation wins. No stale-result fallback.
- 2026-10-15 UTC: Preserve checkout edits; use codex/parallel-index-locks and explicit staging paths. Predecessor is superseded with unfinished reconnect UAT recorded.

## Outcomes & Retrospective
Implemented process lifecycle/writer locks, serialized scans, protected initialization/recovery/migration, a cancellable same-Service finalization gate and one shared five-second acquisition budget. Regression tests observed failure before fixes for migration exclusion, premature scanning/cancellation, concurrent initialization and same-Service finalization.

Final verification on Windows amd64: go test ./... PASS; go test -race ./... PASS (existing x86_64 UCRT GCC selected); go vet ./... PASS; native go build ./cmd/focalspan PASS. CGO_ENABLED=0 builds passed for windows/amd64, linux/amd64 and darwin/arm64. Linux/macOS runtime behavior was not executed. Verification logs and binaries are under .build/parallel-index-verification/.

Read-only reviewer found no actionable issues. The subsequently identified same-Service finalization gap was reproduced by TestSameServiceWaiterCancelsDuringUpgradeFinalization and fixed RED-to-GREEN; the shared wait-budget regression and final full/race suites passed afterward. No deferred review findings or acceptance reductions.

Existing MCP guidance edits remain outside this task's code commit. The predecessor's reconnect UAT remains explicitly unverified in its archive. No push, merge into master, runtime binary replacement or MCP registration change was requested; keep codex/parallel-index-locks for review.

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
