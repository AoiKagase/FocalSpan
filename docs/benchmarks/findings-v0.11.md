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

After static verification, the historical benchmark was run with the eight
case `focalspan-history-v0.5` suite, `default` profile, `repeat 1`, and
attribution plus diagnosis enabled. The first invocation stopped before the
benchmark because the default Go build cache was not writable; the same
invocation was then rerun with a writable temporary cache and completed with
48 quality rows, 40 attribution results, and 40 diagnosis results.

The frozen gate was evaluated for `full-evidence-focused` at budget 2048:

| Metric | Baseline | Identity bridge | Gate |
|---|---:|---:|---|
| `path_scope_missing` labels | 9 | 7 | at least 3 advances |
| `symbol_match_missing` labels | 2 | 2 | no improvement |
| `packing_dropped` labels | 7 | 9 | no regression |
| packed labels (all profiles) | 5 | 5 | retain all |
| useful evidence | 5 | 5 | strictly increase efficiency |
| estimated cumulative wire tokens | 12,740 | 12,840 | no increase |
| useful evidence / 1,000 tokens | 0.3925 | 0.3894 | strictly greater |

The candidate therefore **failed**: only two path-scope labels advanced,
packing drops increased by two, wire increased by 100 tokens, and efficiency
declined. Baseline comparison also found four quality rows with a required-path
recall regression in the project-metadata case. All candidate rows remained
budget-compliant, deterministic, relation-valid, and forbidden-free, but those
invariants do not override the failed gate. No new quality baseline is
claimed.

The six temporary reports were privacy-scanned for source/content fields,
absolute paths, usernames, environment values, secret sentinels, NaN, and
Infinity; no matches were found. Their SHA-256 hashes were:

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `51ae4ef2e4259f20c5644431776f7e01331de0023d16969a547fff381541ee61` |
| candidate quality Markdown | `9d751f428402a2321a63804f386d58a80fa09565c979bfd2d97c2f7cbc7e11c1` |
| candidate attribution JSON | `a4af9dbbb45654d3bb4939e34a8b05d5c2eaa22bd32ee368aa91a552d75e3cb1` |
| candidate attribution Markdown | `66db40c49edf55b31053c64a00b9aaad60f3956cd22dbf9307319e368b126722` |
| candidate diagnosis JSON | `396f68296aca708bcc4d81c084a48cf05a3c436dbaf87d79df048200f23c6bb8` |
| candidate diagnosis Markdown | `4012a7c9669ec9a73196917ebe4e7b0da3ea02f73a1824e03d65ad22b6f8fa14` |

Temporary reports, the benchmark workspace, and the writable Go cache are
removed after this finding is recorded. No source text, absolute path,
username, or environment value is included in this report.

## Implementation evidence

The focused tests cover deterministic hint extraction, explicit-path
suppression, bounded lexical fallback, relation-anchor selection, structural
entry filtering, symbol filtering, stable ordering, and documentation
exclusion. The development attribution validator accepts the internal
retriever identity without changing the v1 wire schema.

## Next action

Reverse-revert only the identity-bridge product commit and retain this
negative finding. The next optimization must start from the v0.10 baseline;
do not retry the rejected identity-bridge hypothesis without a new plan and
new evidence.
