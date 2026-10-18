# C# functional fixes Implementation Plan

## Purpose / Big Picture
Fix four approved C# extraction defects: generic member names, expression-body calls, receiver resolution, and complete property source spans. This plan is the executable specification of the approved 2026-10-18 chat plan.

## Context and Orientation
Production parsing is in internal/extract/csharp. Cross-file calls are resolved in internal/linker. internal/indexer versions invalidate stale extraction. The previous completed C++ plan is archived byte-for-byte.

## Interfaces and Dependencies
Public APIs, Evidence Packet fields, and database schema remain unchanged. Internal declaration ranges and C# scope helpers may change. No new dependencies, network access, external LLM, or compilation/execution of indexed C# source. Product Go tests/builds are authorized.

## Plan of Work
1. Archive predecessor; activate this plan in a documentation-only commit.
2. CS01: add failing generic declaration/call tests; recover names across nested angle arguments; preserve operators; pass and commit.
3. CS02: add failing expression method/test relation tests; store optional expression range and reuse bounded call extraction; pass and commit.
4. CS03: add failing scope/receiver/linker tests; share conservative C# resolution across extraction/linking, leave unknown/ambiguous calls unresolved; pass and commit.
5. CS04: add failing property source/signature tests; include attributes, modifiers and types without preceding members/comments; pass and commit.
6. Advance extractor version to extractors-v21-csharp-functional; verify unchanged-file reindex and public Query/Expand/Impact/Evidence behavior; review whole branch and record gates.

## Validation and Acceptance
Each defect needs a witnessed RED to GREEN cycle using real extraction. Cases: generic nesting and normal/operator controls; expression lambdas/initializers; bare/this/static/unknown/ambiguous receivers across files; attributed/multiline/expression/indexer/explicit-interface properties and exact bytes. Integration fixtures under testdata; test source slices and wire budgets/invariants. Run go test ./..., go test -race ./..., go vet ./..., go build ./cmd/focalspan; CGO_ENABLED=0 cross-build Windows amd64, Linux amd64, Darwin arm64. Set GOPROXY=off and GOSUMDB=off. Report unavailable prerequisites instead of downloading dependencies.

## Idempotence and Recovery
Use codex/csharp-functional-fixes in the managed isolated worktree. Preserve the original dirty checkout and its baseline hashes. Stage explicit task paths only. Never reset, clean, stash or push. Deployment is authorized by the 2026-10-18 follow-up below. Commit each verified defect separately; record remaining work here.

## Progress
- [x] 2026-10-18T12:12:29.174Z Created isolated worktree; captured original dirty-file hashes.
- [x] 2026-10-18T12:16:57.382Z CS01 generic names: witnessed missing Echo/Map and Value arity RED; C# package GREEN (exit 0).
- [x] 2026-10-18T12:19:01.974Z CS02 expression calls: witnessed four missing relations and truncated lambda RED; C# package GREEN (exit 0).
- [x] 2026-10-18T12:25:24.562Z CS03 conservative scope: extraction and real-store linker RED witnessed; both packages GREEN. Baseline go test ./... GREEN before product changes (exit 0).
- [x] 2026-10-18T12:27:06.521Z CS04 full property spans: missing modifier/type/attribute RED witnessed; all property variants and C# package GREEN.
- [x] 2026-10-18T12:40:38.504Z Version migration RED/GREEN and public Query/Expand/Impact/Evidence integration GREEN.
- [x] 2026-10-18T13:01:06.064Z Whole-branch review closed; all final validation gates exit 0.

## Surprises & Discoveries
- Tool output compression can hide source text; bounded source reads are used.

## Decision Log
- 2026-10-18T12:12:29.174Z Execute serially because all four changes share the C# parser and builder. One fresh whole-branch reviewer follows implementation.
- Unknown receivers and overload ambiguity remain unresolved; no type inference or compiler is introduced.
- Initial implementation retained its task branch. The subsequent user request explicitly authorized commit, merge and binary redeployment; push remains outside scope.

## Outcomes & Retrospective
Completed CS01-CS04 and two independent-review corrections. Final code commit: 430f2e0035289cf7ee96701b40626be19ee929a2. Public interfaces/schema unchanged. The code was subsequently fast-forward merged to master and deployed under the explicit 2026-10-18 follow-up authorization. The merged task worktree was archived; push was not requested. Original dirty checkout tracked files remain byte-identical (7/7), and the predecessor archive matches its original SHA-256.

CS03 discovery: base is a keyword token, so call-name parsing must retain it instead of dropping the receiver.

## Integration discoveries and rulings
- 2026-10-18T12:40:38.504Z Ruling: add C# member dependency keys to existing relation_lookup projection, without schema changes. Full and incremental linker otherwise skip this.Save and relative type targets when future partial members live in differently named files. Both failures were witnessed before the lookup correction; linker suite GREEN after it.
- Local functions remain in their enclosing callable scope and are excluded from type-member lookup; witnessed RED/GREEN.
- Public Expand uses callees, not internal relation kind calls; corrected the integration test to the existing interface.
- Baseline test script initially had shell environment quoting errors. Re-ran with explicit GOPROXY=off/GOSUMDB=off/GOTOOLCHAIN=local; clean baseline go test ./... completed with exit 0.
- PLAN.md serves as this task's durable execution ledger under repository policy.

## Independent review corrections
- 2026-10-18T12:47:00.540Z Fresh reviewer reported two Important findings: complex receivers were reinterpreted as bare calls; tuple type arguments prevented generic-call recovery. Both reproduced RED, then C# and linker suites passed GREEN.
- Preserve an opaque <receiver> qualifier for unsupported receiver syntax so it cannot bind as a bare call; do not infer its type. Balance tuple/array delimiters while recovering generic names.
- Final gates are rerun after these production changes. Initial race compile used 32-bit MinGW; rerun with existing C:/msys64/ucrt64/bin/gcc.exe (x86_64-w64-mingw32), process-local CC/PATH only.

## Final Validation Evidence

2026-10-18T13:01:06.064Z — all gates below completed on the final production code. Logs are in C:/Temp/focalspan-csharp-task/.

| Gate | Result | Log |
| --- | --- | --- |
| go test -count=1 -json ./... | exit 0; 44 packages, 1461 tests/subtests passed | test-uncached.log |
| go test -race -count=1 -json ./... | exit 0; 44 packages, 1461 tests/subtests passed | race-uncached.log |
| go vet ./... | exit 0 | vet-final.log |
| go build ./cmd/focalspan (output directed to scratch) | exit 0 | build-final.log |
| CGO_ENABLED=0 Windows amd64 / Linux amd64 / Darwin arm64 | all exit 0 | windows-final.log / linux-final.log / darwin-final.log |

- No network, dependency download, external LLM, or indexed C# compilation/execution was used. Race used the installed x64 UCRT GCC via process-local CC/PATH.
- Cache-enabled full runs emitted successful test output but did not return promptly after app package completion; those task-owned processes were stopped. Fresh -count=1 runs then completed with exit 0. Test caching was bypassed, not test coverage.
- Independent source review (read-only) found two Important issues; both received RED/GREEN regressions and a clean follow-up review. No remaining Critical/Important findings.
- Evidence Validate and MeasureModelVisible budget checks passed at 512 and 1200 tokens for the C# fixture; Query/Expand/Impact and unchanged-source v20-to-v21 refresh passed.
- Only documentation is changed after these gates; final documentation checks are git diff --check and targeted inspection.

## Commit / Merge / Deployment Follow-up

Authorization: user requested commit, merge, and redeployment on 2026-10-18.

- [x] 2026-10-18T13:17:57.043Z Fast-forward merged codex/csharp-functional-fixes into master at 67063998d4920d9dd47c79693bf0981e22749acf; original dirty files retained.
- [x] Built the Windows amd64 CGO-disabled executable from that master revision, including the pre-existing local MCP changes (vcs.modified=true). Candidate path: C:/Temp/focalspan-csharp-deploy/focalspan.exe.
- [x] Existing MCP overlay: go test -count=1 -json ./internal/mcpserver and go test -race -count=1 -json ./internal/mcpserver both exit 0. Race used the installed x64 UCRT GCC.
- [x] 2026-10-18T13:13:17.0334561Z Atomically replaced H:/bin/focalspan.exe via File.Replace. New SHA-256: dffd651bd385ae2caddfd02ebba7076008b948b27f46349d385a6f15ab77583e. Previous SHA-256: 09546a2b2145d8ff80e9724cf4c7ebb040cea855a75756ccd5d281881e7649fb. Backup: H:/bin/focalspan.previous-20261018T131010Z.exe.
- [x] Fresh installed-binary MCP connection: all six tools exposed; complete C# DisplayName declaration appeared once on wire; Echo<T> source returned; ready=true, index_fresh=true, stale=false on the isolated one-file C# deployment fixture. This is fresh-process verification, not a claim that old host workers were replaced.
- [x] Merged managed worktree archived and task branch cleanup performed. Original tracked/untracked file hashes retained (10/10).

Deployment evidence: C:/Temp/focalspan-csharp-deploy/deployment.json, build-info.txt, candidate-mcp.json, installed-mcp.json, build.log, mcp.log, mcp-race.log.

Deployment discoveries: Get-FileHash was unavailable in the command environment; the first attempt stopped before copying or replacing any binary. A read-only .NET SHA256 probe matched independent Node hashes, then File.Replace and both post-swap hashes passed. A lowbox-hosted Node probe timed out on installed-binary initialization; the same smoke protocol run in the normal host environment succeeded and closed cleanly. No repository configuration or existing project worker was changed/stopped. Existing host MCP connections require reconnecting to load the replaced executable.
