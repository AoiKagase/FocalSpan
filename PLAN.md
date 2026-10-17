# Preserve UTF-8 BOM source identity and byte positions

## Purpose / Big Picture

Scanned file content, SHA-256 and size must describe the original bytes, including a UTF-8 BOM. Indexed byte ranges must select their exact content from those bytes. Adding or removing only a BOM must be detected by incremental indexing. Preserve current first-line extraction and Evidence source/wire contracts.

## Progress

- [x] 2026-10-17 UTC: Found scanner removes BOM before hashing, sizing and extraction. Archived the completed linking-recovery plan byte-for-byte.
- [ ] Observe failing scanner, registry and incremental/service regressions.
- [ ] Preserve raw scan bytes and translate BOM-free parser offsets back to raw positions.
- [ ] Full offline verification, independent review, scoped commit, merge and merged branch cleanup.

## Surprises & Discoveries

All extractors previously received BOM-free content from the scanner. Simply passing BOM through risks losing first-line symbols in language-specific parsers. Normalize only the extraction view and translate resulting symbol/chunk byte ranges by the BOM length.

## Decision Log

- 2026-10-17 UTC: Keep raw identity in SourceFile; centralize BOM handling in Registry.Extract so existing parsers keep their prior input. Do not change packet schemas or line numbers.
- 2026-10-17 UTC: Own PLAN.md, predecessor archive, scanner and registry production sections plus new regression files. Preserve user dirty changes. No network, dependency restore, fixture code execution or deployment.

## Outcomes & Retrospective

Pending implementation and verification.

## Context and Orientation

internal/repository/scanner.go currently strips three bytes from BOM-prefixed UTF-8. internal/extract/extract.go selects extractors. Indexer compares SourceFile.SHA256 to stored hashes. Store candidates contain source byte ranges. Evidence source output requires exact content and wire-budget/invariant verification.

## Plan of Work

1. Assert raw scanner content/hash/size, registry exact raw-byte ranges and first-line symbols for representative languages, plus BOM-only incremental changes and source Evidence fidelity.
2. Remove scanner normalization, normalize parser input inside registry, restore symbol/chunk byte positions without changing content, line ranges or hashes of extracted chunks.
3. Run required verification and review. Commit explicit task paths, fast-forward master, delete merged task branch and record evidence.

## Validation and Acceptance

Observe RED before production edits. Run focused regressions, go test ./..., go test -race ./..., go vet ./..., native build and CGO-disabled Windows amd64/Linux amd64/Darwin arm64 builds, offline GOPROXY=off/GOSUMDB=off/GOTOOLCHAIN=local. Race compiler C:/msys64/ucrt64/bin/gcc.exe. Serialize full normal and race suites after a prior unrelated supervisor fixture flake under concurrent verification. Verify Evidence invariants and measured wire budget, git diff --check and unrelated patch hash 481d2c278a820ccfda79ec7bde9bd0ddd35cd68a.

## Idempotence and Recovery

Raw BOM-bearing hashes differ from old stripped hashes, so existing affected files reindex on their next update. BOM-free files remain unchanged. Revert only task commits if necessary; preserve user files and detached worktrees. Final docs-only changes require scoped checks only.

## Interfaces and Dependencies

Existing SourceFile, Extraction, Registry, Indexer and Evidence APIs; no dependencies, migrations or new public fields.
