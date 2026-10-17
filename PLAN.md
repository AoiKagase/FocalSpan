# Preserve original generic extraction source bytes

## Purpose / Big Picture

Generic chunks must retain original line endings and byte ranges selecting exactly their content. CRLF, mixed endings, UTF-8, EOF and BOM must not produce normalized source labeled verbatim. Refresh existing indexes once after the extraction behavior changes.

## Progress

- [x] 2026-10-17 UTC: Traced generic normalized lines into appendGeneric content and byteOffset. Archived completed BOM plan byte-for-byte.
- [ ] Observe failing exact-source/range and public Evidence regressions, including old-version refresh.
- [ ] Slice raw bytes by selected lines and bump extractor version for existing indexes.
- [ ] Full verification, independent review, scoped commit, merge and merged branch cleanup.

## Surprises & Discoveries

Generic splitLines normalizes CRLF for parsing. appendGeneric reconstructs content with LF and counts each newline as one byte; EndByte also includes an invented final newline while Content excludes it. Keep parsing behavior and line selection unchanged; derive source storage from original bytes.

## Decision Log

- 2026-10-17 UTC: Preserve existing exclusion of the final selected line terminator; preserve all bytes between selected lines. Use source line starts and clamp EOF naturally, keeping symbols/chunks/content hashes consistent.
- 2026-10-17 UTC: Own PLAN.md, predecessor archive, generic appendGeneric and version constant, plus new focused tests and any BOM regression tightening. Preserve user changes and existing worktrees. No network, restore, fixture code execution or deployment.

## Outcomes & Retrospective

Pending implementation and verification.

## Context and Orientation

internal/extract/generic/generic.go funnels declaration, Markdown and fallback windows through appendGeneric. internal/extract/sourceutil maps original line starts. Registry adjusts BOM offsets. internal/indexer/indexer.go extractorVersion invalidates unchanged files when extraction changes. Evidence uses stored chunk source in source mode.

## Plan of Work

1. Test raw content/hash/ranges for LF/CRLF/mixed/UTF-8/empty/EOF sources and selected noninitial lines; cover public PowerShell Evidence with BOM and CRLF and old-version refresh.
2. Map selected line ranges back to raw bytes without changing parser input. Update version so old normalized chunks are replaced on incremental indexing.
3. Run focused/full verification and read-only review, commit explicit paths, fast-forward master, remove merged task branch and record results.

## Validation and Acceptance

Observe RED before product changes. Validate packet invariants and measured wire budget for source Evidence. Run go test ./..., go test -race ./..., go vet ./..., native build and CGO-disabled windows/amd64, linux/amd64, darwin/arm64 builds. Offline GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local; race compiler C:/msys64/ucrt64/bin/gcc.exe. Serialize normal and race suites. Run git diff --check and preserve unrelated tracked patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a.

## Idempotence and Recovery

Version mismatch forces a single refresh even with unchanged file hash; next update skips unchanged files. Existing writer transaction and cancellation protections apply. Revert only scoped task commits if needed and preserve user files. Final docs-only edits use scoped checks.

## Interfaces and Dependencies

Existing generic extractor, sourceutil.SourceMap, SourceFile, Indexer metadata and Evidence APIs. No schema migration, dependencies or public fields.
