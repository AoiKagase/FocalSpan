# Configure every Store SQLite connection

## Purpose / Big Picture
Every physical Store connection must enforce foreign keys and retain the existing five-second SQLite busy timeout and NORMAL synchronous setting. Pool growth must not leave orphaned index rows or fail immediately on transient writer contention.

## Progress
- [x] 2026-10-17 UTC: Inspected four-connection pool and one-time PRAGMA setup. Deletion relies on cascading foreign keys. Verified the pinned driver supports a per-driver connection hook.
- [ ] Add deterministic additional-connection configuration, cascade/source-isolation and writer-lock regressions; observe RED.
- [ ] Configure each Store connection using a private driver hook; observe GREEN.
- [ ] Full verification/review, scoped commits, local master merge and branch cleanup.

## Surprises & Discoveries
configureAndMigrate applies connection-local pragmas through sql.DB once. Additional physical connections use driver defaults. Deleting a files row on such a connection can retain chunks and symbols; reused file IDs could associate stale source with a new path.

## Decision Log
- 2026-10-17 UTC: Scope this audit step to pooled Store connection settings. Preserve user-owned dirty files and the completed predecessor. No network, new dependencies or fixture source execution.
- Register a private sqlite.Driver with a connection hook, leaving the default driver and context-aware schema probe untouched. Apply busy timeout, foreign keys and synchronous mode for each Store connection. Keep persistent WAL setup and migrations under the existing lifecycle flow.
- Use the pinned driver source/docs available locally; avoid DSN/path encoding changes and process-wide default-driver hooks.

## Outcomes & Retrospective
Pending.

## Context and Orientation
internal/store/store.go opens the Store sql.DB with up to four connections; configureAndMigrate currently executes foreign_keys, busy_timeout, journal_mode and synchronous once. The schema probe uses the default driver and cancellable retry. deleteFileTx relies on ON DELETE CASCADE.

## Plan of Work
1. Reserve multiple physical connections and verify all settings. Force DeleteFile through a newly created fourth connection and verify no orphan source can appear under a later file. Check a second connection waits for a held SQLite writer lock.
2. Add a Store-only driver hook for connection-local pragmas and use it in openDatabase. Keep raw schema probes/default SQLite helpers unchanged. Retain WAL configuration in lifecycle-protected setup.
3. Run focused regressions, full tests/race/vet/native and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 builds. Include existing migration, concurrency and Evidence wire/fidelity suites. Review, commit explicit paths, merge locally and delete the merged task branch.

## Validation and Acceptance
Observe RED before implementation. Tests reserve connections so pool expansion is deterministic and exercise actual SQLite constraints/cascades/writer contention. Verify current schema opens still avoid migration writes and schema-probe cancellation remains intact. Run offline and use 64-bit UCRT GCC for race. Cross-builds do not prove non-Windows runtime behavior.

## Idempotence and Recovery
Use temporary DBs. Close reserved connections before Store cleanup and roll back held transactions. Archive previous PLAN byte-for-byte at docs/superpowers/plans/completed/2026-10-17-sqlite-schema-probe.md. Preserve unrelated changes. No runtime deployment or remote push.

## Interfaces and Dependencies
No public API, schema or Evidence wire changes. Reuse modernc.org/sqlite v1.57.0; a private driver affects only Store handles. The metadata schema probe keeps its existing default driver and retry behavior.
