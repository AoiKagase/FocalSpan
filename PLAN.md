# Repair remaining five index consistency problems

## Purpose / Big Picture

Implement the approved five fixes: fail closed on traversal errors; constrain retrieval before caps; serialize complete reads against index writers; refresh/validate Impact; report incomplete indexes as stale. Prefer consistency: shared Writer lease waits at most five seconds, then returns BusyError. Preserve MCP/Evidence schemas and database schema.

## Progress

- [x] 2026-10-17 UTC: Confirmed master a91128f and preserved dirty user paths; archived completed generic-source plan byte-for-byte; created codex/remaining-five-fixes.
- [x] 2026-10-17 UTC: Step 1 RED: missing/empty-cancelled walks returned success and unavailable root was accepted by Indexer. GREEN: repository and indexer suites pass; errors now precede all writes.
- [x] 2026-10-17 UTC: Step 2 RED on path/changed-only late exact match; GREEN across store/search/app suites and explicit 1000+ condition, literal wildcard/path boundary/case/range controls. Outside-scope anchors preserved; Evidence wire/invariants pass.
- [x] 2026-10-17 UTC: Step 3 RED admitted writer during paused read and admitted query during subprocess writer. GREEN app suite: shared lease spans all public result construction; parallel readers pass; six public APIs honor caller wait cancellation. Auto-update moved before admission; pending reads reject incomplete updates.
- [x] 2026-10-17 UTC: Step 4 RED in all worktree/ref × automatic/manual combinations returned old source. GREEN full app suite: automatic Impact updates before read lease; affected source hashes verified even after auto-update; manual mode never mutates and rejects mismatches. Deleted-file controls still pass.
- [x] 2026-10-17 UTC: Step 5 RED on missing/pending/old metadata and actual interrupted linking. GREEN app suite: Status/Health derive stale/readiness from required metadata under shared lease; data reads reject stale/incomplete indexes; recovery passes. Git-failure fixture now initializes an index so it reaches its intended Git-error boundary.
- [ ] Full offline verification, independent whole-branch review, fast-forward merge and merged task branch cleanup.

## Surprises & Discoveries

The prior audit's five temporary overlay tests are in C:/Temp/focalspan_remaining_audit_20261017_test.go. Interleaving tests must use channels and a separate writer rather than synchronously invoking Index while a read lease is held.

Final scan acceptance found file read/stat failures still skipped. Added RED/GREEN preservation test using a tracked file replaced by a directory. Fail-closed scanning also exposed custom index directories previously being read as source; RED/GREEN exclusion test now covers them. Windows missing-file errors wrapped by ContainedPath require errors.Is(fs.ErrNotExist), not os.IsNotExist. Normal Git deletions remain accepted. Root regular-file replacement now rejects before enumeration (RED/GREEN). Repository/indexer and app suites passed these compatibility controls.

## Decision Log

- 2026-10-17 UTC: User selected consistency over immediate stale snapshots. Index before taking Shared Writer lease; private read helpers must not reacquire it. Health may inspect pending metadata; data reads reject incomplete indexes.
- 2026-10-17 UTC: Preserve existing relation anchor discovery outside candidate scope. Apply path/range constraints to returned candidates before caps. Bound SQL scope batches and merge before global caps.
- Ruling: Use fixed-size JSON batches in a parameterized CTE feeding scoped chunk sources before existing ORDER/LIMIT. This preserves each retriever's exact ordering and avoids parameter/expression limits without merging independently truncated batches. SQLite JSON support verified by real-store tests.
- 2026-10-17 UTC: Auto-update disabled means no indexing; Impact rejects eligible current files whose stored hashes are absent/different. Deleted source is never synthesized.
- 2026-10-17 UTC: Own only plan/archive and source/test sections required by these five fixes. Preserve user edits in AGENTS.md, README.md, docs/design.md and internal/mcpserver/{server.go,context_v2_test.go}, plus all existing untracked files. Baseline tracked patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. No network, restore, analyzed fixture-code execution, push or deployment.

## Outcomes & Retrospective

All five implementation steps have targeted passing regressions and independent whole-branch read-only review found no actionable issues, including latest scanner compatibility changes. Final full verification and integration remain. This file is the durable inline execution ledger.

## Context and Orientation

Repository scanner silently ignores WalkDir errors. Search retrieves capped lists before Paths/ChangedOnly filtering. Service reads candidates and revision separately. Impact reads current Git ranges against stored source. Health only tests config hash and nonempty last revision. Existing indexlock supports shared/exclusive Writer leases across processes; index writers hold Exclusive through final metadata.

## Plan of Work

1. Formalize traversal regression and preserve old index/metadata on failure; propagate walk errors and context cancellation.
2. Introduce internal CandidateScope/scoped reader with parameterized scope predicates before candidate caps. Keep unconstrained behavior, relation anchors and path-boundary semantics.
3. Centralize read admission, one Shared Writer lease per public read, auto-update before admission, and one revision per result. Test same-process and subprocess contention/cancellation and parallel reads.
4. Update Impact before read admission when enabled; otherwise compare affected eligible source hashes under lease and fail explicitly on mismatch. Cover working tree, explicit refs and deletions.
5. Derive stale/readiness from config, extractor version, linking_pending and successful completion; propagate metadata errors and retain diagnostics.
6. Run full gates and fresh read-only review; commit explicit paths, merge master, delete merged task branch, document actual outcomes.

## Validation and Acceptance

Write and observe each failing deterministic regression before behavior edits. Include 130 duplicate symbols, path/changed-only filters, out-of-scope relation anchors, snapshot barriers, five-second wait/cancellation, incomplete metadata, exact Evidence source and measured wire/invariants. Run focused tests per task; final go test ./..., then go test -race ./..., go vet ./..., native build and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 cross-builds. Offline GOPROXY=off/GOSUMDB=off/GOTOOLCHAIN=local; race compiler C:/msys64/ucrt64/bin/gcc.exe. git diff --check and user patch hash must remain valid. Final docs-only changes use scoped checks.

## Idempotence and Recovery

Do not delete lock files. Preserve existing lease ordering and writer transaction rollback. Failed reads do not mutate indexes. Missing completion metadata requires an update, not automatic migration. Revert only owned commits if needed. Preserve all user files and detached worktrees.

## Interfaces and Dependencies

Internal CandidateScope/candidate reader and read-admission helpers; expose current extractor version internally for readiness comparison. No new dependencies, DB schema versions, public packet fields or new debug output. Existing five-second BusyError and cancellation remain public error behavior. Supervisor EOF investigation is excluded.
