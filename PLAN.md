# Cancellable SQLite schema-probe contention

## Purpose / Big Picture
Concurrent index openers must wait through transient SQLite schema-read contention rather than fail immediately with database is locked (261). Preserve the existing five-second acquisition budget, caller cancellation and non-busy errors.

## Progress
- [x] 2026-10-17 UTC: Reproduced TestConcurrentInitialOpenKeepsReadersAlive failure in a 100-repeat race run. Found unconfigured, context-free readSchemaVersionPath connection used before Store configuration.
- [x] 2026-10-17 UTC: Deterministic real-lock tests returned immediate SQLITE_BUSY before release and before the caller deadline (RED). Corrupt-database control passed without waiting.
- [x] 2026-10-17 UTC: Passed the existing open context to QueryRowContext and retried only primary SQLite BUSY (including extended codes) with cancellable 25ms waits. Release/deadline/control tests passed; explicit cancellation control added.
- [x] 2026-10-17 UTC: Focused/stress/full verification and review passed. Scoped fix committed as fa44628 and fast-forward merged into master. Deleted the merged work branch; only local master remains.
- [x] 2026-10-17 UTC: New real-lock regressions, cancellation and corruption controls plus the existing concurrent initializer passed 100 repeats with race enabled (85.439s). Full tests, full race suite, vet, native build and all three CGO-disabled cross-builds passed. Independent review found no actionable issues.

## Surprises & Discoveries
The earlier one-off 261 failure was reproducible. All openers hold shared lifecycle leases, so transient SQLite WAL recovery may still contend even though migration is protected by exclusive leases. The probe has no busy timeout and does not consume the caller context.

## Decision Log
- 2026-10-17 UTC: Continue the audit with this observed failure only. Preserve dirty user files. Reuse current checkout with a feature branch. No new dependencies, network, fixture source execution or runtime deployment.
- Retry only primary SQLITE_BUSY (including extended recovery code 261), under the existing openWithLease context and its five-second ceiling. Do not retry schema errors, ignore cancellation, or change migration/data semantics.
- Keep the completed predecessor byte-for-byte in docs/superpowers/plans/completed/2026-10-17-git-change-correctness.md.

## Outcomes & Retrospective
The previously intermittent initialization failure is reproduced and addressed with bounded context-aware BUSY retry. Deterministic regressions observed failure before the fix and passed afterward. Both the lifecycle lease and the original acquisition deadline are retained throughout retries. No migration, public wire or source-fidelity behavior changed. Full verification and independent review passed; fa44628 is merged into master with the same verified code tree. The merged task branch was removed. User-owned tracked edits preserve patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. No push or runtime deployment was performed. This final update is documentation-only.

## Context and Orientation
internal/store/lifecycle.go acquires leases and probes the database through readSchemaVersionPath in store.go. The probe opens a separate connection before configureAndMigrate. Existing store/concurrency_test.go checks simultaneous initialization with 12 readers.

## Plan of Work
1. Hold a real SQLite exclusive transaction on a completed fixture DB. Require openWithLease to wait until release, respect a caller deadline, and release its lifecycle lease after cancellation. Confirm the tests fail before implementation.
2. Pass the open context into the probe. Use QueryRowContext and retry only SQLite busy results with cancellable short waits, preserving errors and original budget. Add non-busy-error control coverage.
3. Run focused tests and the existing concurrent initializer race test repeatedly, then full tests/race/vet/native and CGO-disabled cross-builds. Obtain independent read-only review, commit only task files and merge locally.

## Validation and Acceptance
No timing-only reproduction is needed for the regression: an explicit SQLite exclusive lock is held before opening. Positive wait and deadline cases must pass, along with corrupt/unsupported schema controls and lifecycle/migration suites. Run offline, use 64-bit UCRT GCC for race. Cross-build Windows amd64, Linux amd64 and Darwin arm64; do not claim non-Windows runtime execution.

## Idempotence and Recovery
Use temporary fixture DBs only. Close all connections and release leases on errors. Preserve unrelated dirty files. No network, runtime binary replacement or database migration changes.

## Interfaces and Dependencies
Internal readSchemaVersionPath receives context.Context. No public CLI/MCP/Evidence types change, and no dependency is added.
