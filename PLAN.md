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
- [ ] CS04 full property spans.
- [ ] Version migration, integration, review and all validation.

## Surprises & Discoveries
- Tool output compression can hide source text; bounded source reads are used.

## Decision Log
- 2026-10-18T12:12:29.174Z Execute serially because all four changes share the C# parser and builder. One fresh whole-branch reviewer follows implementation.
- Unknown receivers and overload ambiguity remain unresolved; no type inference or compiler is introduced.
- Leave the final commits on the task branch; merge/push/deploy were not requested.

## Outcomes & Retrospective
Pending implementation and validation.

CS03 discovery: base is a keyword token, so call-name parsing must retain it instead of dropping the receiver.
