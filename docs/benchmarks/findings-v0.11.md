# FocalSpan Identity-Bridge Retrieval v0.11 Findings

## Scope and execution

This milestone adds a bounded identity bridge for package/module-aware
retrieval. Explicit structural markers are resolved to structural entry files,
then to non-structural symbol-bearing chunks. Natural-language fallback probes
are limited and remain scoped to structural files; generic documentation is
never promoted. Public MCP, CLI, Evidence, and known-handle contracts were not
changed.

Static verification completed after the implementation:

- focused identity-bridge and store tests passed;
- `go test ./... -count=1` passed across all repository packages;
- `go vet ./...` passed with no diagnostics;
- `git diff --check` passed; and
- native plus CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds
  completed successfully with temporary outputs removed.

## Candidate gate status

The historical benchmark did not produce a valid candidate report. The first
invocation produced no completion output or report files and was recorded as
an infrastructure failure. The one permitted retry reached attribution
validation but stopped on `invalid sanitized hit` because the development
retriever allow-list did not yet include `identity-bridge`. A RED test then
fixed that allow-list and passed GREEN. Per the one-candidate policy, the
historical benchmark was not run again after the code correction.

Consequently, v0.11 has **no measured gate result**: path/symbol/anchor
improvement, cumulative wire tokens, and efficiency are unmeasured. No new
quality baseline is claimed, and no source text, absolute path, username, or
environment value is included in this report.

## Implementation evidence

The focused tests cover deterministic hint extraction, explicit-path
suppression, bounded lexical fallback, relation-anchor selection, structural
entry filtering, symbol filtering, stable ordering, and documentation
exclusion. The development attribution validator accepts the internal
retriever identity without changing the v1 wire schema.

## Next action

Before promotion, run a successor candidate gate under a revised ExecPlan (or
explicitly authorize one post-fix benchmark run) and require the frozen v0.10
conditions: at least three improved path/symbol/anchor labels, wire tokens
`<=12,740`, and efficiency strictly above `0.3925`, with all existing
invariants preserved.
