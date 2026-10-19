# PHP / INC functional fixes

## Purpose / Big Picture
Implement five approved fixes: scoped calls, grouped use, import-kind isolation, typed constants, parenthesized includes. Preserve CLI/MCP fields and DB schema. No PHP/Composer execution, network, or new dependencies. The initial scope excluded commits/merge/deployment; the 2026-10-19 follow-up explicitly authorizes committing, merging into master, and redeploying the Windows binary. Push is outside scope.

## Context and Orientation
PHP parser/builder: internal/extract/php. Cross-file resolution: internal/linker. Dependency projection: internal/store. Version refresh: internal/indexer. Fixtures: testdata.

## Interfaces and Dependencies
Preserve model.Relation and schema. Keep lexical call targets; imported/absolute targets retain leading backslash. Shared PHP helpers serve extraction/linking/dependency keys. Product Go verification explicitly authorized.

## Plan of Work
1. Add five deterministic failing regression groups before production changes.
2. Fix grouped use, typed constants, include parentheses, and separate import aliases; observe PHP GREEN.
3. Fix scoped extraction/linking and dependency projection; verify full/incremental parity, ambiguity and target-only changes.
4. Public Query/Expand/Impact/Evidence acceptance with source fidelity and budget/invariant checks. Advance to extractors-v22-php-functional; verify unchanged-source refresh.
5. Offline full tests/race/vet/build and Windows amd64/Linux amd64/Darwin arm64 CGO-disabled builds; independent review.

## Validation and Acceptance
Observe RED then GREEN for each behavior. Validate negative and positive names, aliases, unknown receivers, duplicates, group recovery, constant source, static/dynamic/outside includes. GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local. Never claim unrun gates pass.

## Idempotence and Recovery
Worktree C:/Users/SandS/.codex/worktrees/php-functional-fixes/FocalSpan; branch codex/php-functional-fixes. Original 13 dirty-file hashes in C:/Temp/focalspan-php-original-hashes.json. No reset/clean/stash. Predecessor archived byte-for-byte at docs/superpowers/plans/completed/2026-10-18-csharp-functional-fixes.md.

## Progress
- [x] Worktree/branch, original dirty-file hash baseline, predecessor archive.
- [x] Five regression groups written before production changes.
- [x] Syntax/import/include RED witnessed (five regression groups); main tests GREEN.
- [x] Scoped extraction/linking and incremental dependency regressions RED then GREEN.
- [x] Public Query/Expand/Impact/Evidence and full/incremental parity GREEN in host app suite; version v21-to-v22 RED witnessed and indexer suite GREEN.
- [x] 2026-10-19T02:09:24Z Final offline normal/race tests each exit 0: 44 packages, 1494 tests/subtests passed; vet, native build and all three CGO-disabled cross-builds exit 0.
- [x] Bounded independent static review completed with no established Critical/Important findings. Reviewer did not independently execute tests; an approval-stalled diff command limited exhaustive diff coverage. Parent executed all final gates.

## Surprises & Discoveries
Generic linking can undo conservative extraction: fix both layers. Native worktree Git/copy requires approved host permissions.
- Sandbox Go package tests could not create Git fixture configs or rename SQLite upgrade files. Host runs passed app/store/indexer/linker; these are environment restrictions, not waived assertions.
- Additional scope regression found namespace-relative calls and opaque receivers becoming bare calls; RED witnessed and PHP suite GREEN after correction.
- Post-implementation alias review: external class aliases were discarded when no local type existed, and lexical readers erased absolute type prefixes. Added actual linker RED plus five absolute-reference RED cases (inheritance/property/signature/new/class literal), then fixed kind-aware type/import linking and lexical spelling. PHP+linker targeted suites GREEN in focalspan-php-types-green2.log.

## Decision Log
- 2026-10-19: Initial implementation excluded commits, overriding the PLANS.md documentation-transition commit rule. The follow-up now authorizes separate documentation-transition and product commits, merge, and deployment.
- 2026-10-19: Execute inline; final independent reviewer required by executing-plans skill.
- 2026-10-19 Ruling: same-file PHP calls keep their lexical target even when locally resolved. Store persists them unresolved so the linker evaluates all files; lookup projection survives linking and causes reextraction on additions/removals/ambiguity. No schema change.
- 2026-10-19 Ruling: defer namespaced global fallback until linking; a preferred declaration may live in another file. Type alias normalization now respects absolute and namespace-relative names.
- 2026-10-19 Ruling: preserve import kind on unresolved import Source using php-import-class/function/const. This uses the existing relation fields and no schema/public shape change. Class references retain expanded aliases and absolute lexical spelling; class/function identical qualified names remain distinct lookup kinds.

## Outcomes & Retrospective
All five approved behaviors are implemented and verified, including follow-up class-alias/absolute-type and receiver/recovery corrections. Public CLI/MCP field shapes and DB schema are unchanged. Extraction version is extractors-v22-php-functional. The initial implementation completed without indexed PHP/Composer execution, network/dependency fetch, product commits, merge, push, or deployment. The authorized integration follow-up is recorded below.

Final evidence (all commands set GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local):

| Gate | Result | Evidence |
| --- | --- | --- |
| go test -count=1 -json ./... | exit 0; 44 packages, 1494 tests/subtests passed, zero failures | C:/Temp/focalspan-php-test-final2.jsonl |
| go test -race -count=1 -json ./... | exit 0; same 1494 tests/subtests, zero failures; installed x64 UCRT GCC | C:/Temp/focalspan-php-race-final2.jsonl |
| go vet ./... | exit 0 | C:/Temp/focalspan-php-vet-final2.log |
| go build ./cmd/focalspan (scratch output) | exit 0 | C:/Temp/focalspan-php-build-final2.log |
| CGO_ENABLED=0 Windows amd64 / Linux amd64 / Darwin arm64 builds | all exit 0 | C:/Temp/focalspan-php-windows-final2.log / focalspan-php-linux-final2.log / focalspan-php-darwin-final2.log |
| git diff --check | exit 0 | Final worktree inspection |

Public acceptance exercises typed constant Query/self Expand, imported function callee Expand, changed INC Impact, Evidence Validate, verbatim bytes and MeasureModelVisible budgets at 512/1200 tokens. Target-only addition, duplicate ambiguity, and deletion produce the same call relations under incremental and full indexing. Generic old extraction is refreshed for unchanged INC source from v21 to v22, and a subsequent run is unchanged.

Original checkout dirty-file hashes match 13/13; predecessor archive is byte-identical to original PLAN.md. Initial delivery retained uncommitted changes on codex/php-functional-fixes in the managed worktree above. Earlier final logs without the final2 suffix predate the type-reference correction and are superseded. Documentation updates follow the successful final gates and require only documentation-scoped inspection.

## Commit / Merge / Deployment Follow-up

Authorization: user requested commit, merge, and redeployment on 2026-10-19. Preserve unrelated master changes and do not push.

- [x] Baseline rechecked: master and task worktree share base 0054c2c; original dirty-file hashes still match 13/13; no top-level Git lock found.
- [ ] Commit the plan transition separately, then the 16 explicit product/test/fixture paths.
- [ ] Fast-forward master; verify merged working tree including existing MCP overlay; preserve original dirty files.
- [ ] Build Windows amd64 CGO-disabled candidate from merged master; validate it on isolated PHP/INC fixture through a fresh MCP process.
- [ ] Back up and atomically replace H:/bin/focalspan.exe; verify hashes and fresh installed-binary MCP behavior; record exact revision and modified-source status.
- [ ] Record final results, archive the merged task worktree, and remove the merged task branch.
