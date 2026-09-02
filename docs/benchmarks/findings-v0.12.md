# FocalSpan Anchor-First Evidence Packing v0.12 Findings

## Scope and execution

v0.12 tested a bounded anchor-first reservation pass in the Evidence
compiler. Exact-symbol, explicit-path, and relation candidates (including
available relation endpoints) were tried before generic lexical candidates,
using only the existing fidelity variants. Retrieval, ranking, Evidence/MCP
schemas, CLI methods, and `known_handles` were unchanged.

Static verification completed before the candidate gate:

- the four new anchor-reservation tests and the full Evidence package passed;
- `go test ./... -count=1` passed across all repository packages;
- `go vet ./...` passed with no diagnostics;
- `git diff --check` passed; and
- native plus CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds
  completed successfully with temporary outputs removed.

`go test -race ./...` is **UNVERIFIED** because the local MinGW toolchain
reports `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

## Candidate gate status

After static verification, the historical benchmark was run once with the
eight-case `focalspan-history-v0.5` suite, `default` profile, `repeat 1`, and
attribution plus diagnosis enabled. It completed with 48 quality rows, 40
attribution results, and 40 diagnosis results. The report records the active
plan commit `2799d799952d635b1570e226000b1302bbdfc545`; the candidate source
was captured immediately afterward as temporary commit `ab62461` and then
reverted by `43ca5fc`.

| Metric | Baseline | Anchor-first candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels (focused/2048) | 7 | 4 | at least 3 fewer |
| packed observations (all profiles) | 5 | 20 | retain all existing labels |
| useful evidence | 5 | 18 | strictly increase efficiency |
| estimated cumulative wire tokens | 12,740 | 27,667 | no increase |
| useful evidence / 1,000 tokens | 0.3925 | 0.6506 | strictly greater than 0.3925 |

The candidate therefore **failed** the strict gate: packing drops improved by
three and useful-evidence efficiency increased, but cumulative wire tokens
increased by 14,927. The baseline comparison was otherwise marked compatible,
with 15 wire-token regressions and 5 improvements; the comparison command
returned exit code 1 because of those regressions. No new quality baseline is
claimed.

The candidate implementation and tests were reverted in reverse order after
the gate. The v0.10 baseline remains active.

## Artifact hashes and privacy scan

The six temporary reports were privacy-scanned for source/content keys,
absolute paths, usernames, environment values, secret sentinels, NaN, and
Infinity; no matches were found. Their SHA-256 hashes were:

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `f5014052258fc81e68afe9aebbe5d8d22e6de5afaaa72637b30f024ea6d3264d` |
| candidate quality Markdown | `eb2dfc98d1e0ce824dcf145a19c4dcde5241b0beffd35b1e0a1df03c31d18256` |
| candidate attribution JSON | `974f95a805f56d20aa5bdf9c1de327f51dd64c1d649142ebe7d66bdc80ac1cf4` |
| candidate attribution Markdown | `6a098d8c43225e4531468cc195d30e5977b50dfd94c4aaeec6b2eb439fd3d4a8` |
| candidate diagnosis JSON | `1c77b9da15bdf5d5976bb4144a5a548da7a010f518d9e508d9fc923bdc27c55d` |
| candidate diagnosis Markdown | `ed3f65d9d2f474df2077ae466d5002632c5d2c72ee458a4a47b7858fda66bbb3` |

Temporary reports, the benchmark workspace, build outputs, and writable Go
caches are removed after this finding is recorded. No source text, absolute
path, username, or environment value is included in this report.

## Retrospective

The reservation invariant worked in focused tests and reduced focused/2048
packing drops, but reserving every structural candidate before utility
selection admitted too many Evidence items and raised metadata/source wire
cost. The next packing attempt must preserve the baseline candidate count or
replace existing content with a measured compact variant; it must not simply
reserve all anchors. The rejected v0.12 implementation is not a new quality
baseline.
