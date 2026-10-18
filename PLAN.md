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
Use codex/csharp-functional-fixes in the managed isolated worktree. Preserve the original dirty checkout and its baseline hashes. Stage explicit task paths only. Never reset, clean, stash, push or deploy. Commit each verified defect separately; record remaining work here.

## Progress
- [x] 2026-10-18T12:12:29.174Z Created isolated worktree; captured original dirty-file hashes.
- [x] 2026-10-18T12:16:57.382Z CS01 generic names: witnessed missing Echo/Map and Value arity RED; C# package GREEN (exit 0).
- [x] 2026-10-18T12:19:01.974Z CS02 expression calls: witnessed four missing relations and truncated lambda RED; C# package GREEN (exit 0).
- [x] 2026-10-18T12:25:24.562Z CS03 conservative scope: extraction and real-store linker RED witnessed; both packages GREEN. Baseline go test ./... GREEN before product changes (exit 0).
- [x] 2026-10-18T12:27:06.521Z CS04 full property spans: missing modifier/type/attribute RED witnessed; all property variants and C# package GREEN.
- [x] 2026-10-18T12:40:38.504Z Version migration RED/GREEN and public Query/Expand/Impact/Evidence integration GREEN.
- [ ] Whole-branch review and final validation gates.

## Surprises & Discoveries
- Tool output compression can hide source text; bounded source reads are used.

## Decision Log
- 2026-10-18T12:12:29.174Z Execute serially because all four changes share the C# parser and builder. One fresh whole-branch reviewer follows implementation.
- Unknown receivers and overload ambiguity remain unresolved; no type inference or compiler is introduced.
- Leave the final commits on the task branch; merge/push/deploy were not requested.

## Outcomes & Retrospective
Pending implementation and validation.

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
