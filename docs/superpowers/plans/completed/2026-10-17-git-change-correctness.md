# Git change correctness after merged-branch cleanup

## Purpose / Big Picture
Complete the three remaining audit findings: impact candidates must not disappear behind the global candidate cap, Git quoted paths must match indexed paths, and changed-only queries must include staged changes and propagate Git errors.

## Progress
- [x] 2026-10-17 UTC: Deleted all eight local branches merged into master with safe branch deletion. Detached old worktrees, preserving all untracked files and root dirty changes. Created codex/git-change-correctness.
- [x] 2026-10-17 UTC: Observed RED for late file/chunk impact, staged-only and quoted-path queries, Git failure suppression and parser path decoding. Implemented exact-path filtering before ranking/cap, decoded Git paths, and propagated changed-only errors. Focused original regressions passed.
- [x] 2026-10-17 UTC: Additional RED-to-GREEN checks caught mixed staged/unstaged line shifts and source lines resembling diff headers. Use HEAD-to-worktree coordinates and only parse file headers before hunks. Added unborn-repository and cap/determinism controls.
- [x] 2026-10-17 UTC: Full verification and independent review passed. Committed scoped changes as 279b011 and fast-forward merged into master. Deleted the now-merged work branch; only local master remains.
- [x] 2026-10-17 UTC: Full go test ./..., go test -race ./..., go vet ./..., native build and CGO-disabled windows/amd64, linux/amd64, darwin/arm64 builds passed. Focused regressions including unborn HEAD and deterministic caps passed. Independent read-only review found no actionable issues.

## Surprises & Discoveries
All old feature branches were ancestors of master. Old PHP/structural worktrees contain untracked files and were preserved. No remote branches other than master were present; no network operation was needed.

## Decision Log
- 2026-10-17 UTC: Resume the remaining three findings in original severity order. Preserve user-owned dirty files. No fixture code execution, network or dependency restore. Keep the current checkout, using a new feature branch. No worktree deletion requested or performed.
- Use exact-path Store retrieval before changed-line filtering, rank and then apply MaxCandidates. Preserve existing public packet/CLI formats.
- Compare HEAD to the current worktree for committed repositories so staged/unstaged changes share current line coordinates. For unborn HEAD, all lines of tracked files are changed. Propagate Git errors through queries and baseline calculation. The original range-union approach failed the mixed-edit regression and was replaced.

## Outcomes & Retrospective
All remaining audit findings are implemented with behavioral regressions and passed full verification and independent review. Commit 279b011 is merged into master with an unchanged, verified code tree; the work branch was deleted safely. User-owned tracked edits retain patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a. Tests/builds use offline settings and artifacts under .build/git-change-verification. All nine merged feature branches (eight old branches plus this task branch) were removed; old detached worktree directories and their untracked files remain intact. No runtime deployment or remote push. This final progress update is documentation-only.

## Context and Orientation
internal/app/evidence.go currently limits AllCandidates before change filtering. internal/gitx/diff.go does not decode Git C-quoted paths. internal/app/service.go only reads unstaged changes and suppresses failures.

## Plan of Work
1. Add failing real-Git service regressions with low candidate budgets and a late changed file/chunk; add exact-path store tests when introducing the helper.
2. Add parser cases for C-quoted Unicode, spaces, tabs/quotes and rename paths. Fix path decoding while retaining existing diff kinds and ranges.
3. Add staged-only query tests and Git-error propagation tests for legacy/evidence queries and baseline requests; implement range union and error returns.
4. Run focused/full tests, full race tests with verified 64-bit UCRT GCC, vet, native build and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 builds. Include existing Evidence wire/invariant tests. Review and merge locally, preserving unrelated edits.

## Validation and Acceptance
Observe RED for behavioral changes. Cover late files and late chunks, exact path matching, deterministic candidate limits, malformed quoted paths, staged-only changes and Git failures. Full suite must retain Evidence fidelity and budget invariants. Disable GOPROXY/GOSUMDB and automatic toolchain downloads. Cross-builds do not imply target runtime acceptance.

## Idempotence and Recovery
Use temporary test fixtures only. Keep old worktrees detached with their original files. Archive prior PLAN byte-for-byte at docs/superpowers/plans/completed/2026-10-17-impact-reference-errors.md. Commit plan transitions separately. Revert only task-owned changes if needed.

## Interfaces and Dependencies
Add internal Store exact-path retrieval. changedRanges gains an error return and its internal callers propagate it. No new dependencies, public flags or Evidence wire fields.
