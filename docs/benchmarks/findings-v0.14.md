# Metadata field pruning v0.14 findings

## Scope and execution

v0.14 added a private, final-packet-only metadata pruning pass. It keeps the
public `focalspan.context.v1` schema and JSON key names, preserves selection,
ranking, source fidelity, relations, budgets, and known-handle behavior, and
uses existing `omitempty` fields to omit redundant language/kind values and
low-value `why` codes. Guidance limits and next-action contracts were not
changed.

The historical `focalspan-history-v0.5` suite ran exactly once with the
`default` profile, repeat 1, attribution enabled, and diagnosis enabled. It
completed with 48 quality rows, 40 attribution results, and 40 diagnosis
results. The report comparison completed with `compatible=true` and
`regressions=0`.

Static verification completed before the candidate gate:

- `go test ./... -count=1`: passed (691 tests);
- `go vet ./...`: passed;
- `git diff --check`: passed;
- native build: passed;
- CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds: passed; and
- `go test -race ./...`: **UNVERIFIED** because the local MinGW compiler
  reports `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

## Candidate gate status

The strict gate was evaluated against the active v0.10 baseline at
focused/2048.

| Metric | Baseline | Metadata-pruned candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels | 7 | 7 | no increase |
| packed labels | 5 | 5 | retain all |
| cumulative estimated wire tokens | 12,740 | 12,304 | strictly lower |
| useful evidence | 5 | 5 | retain |
| useful evidence / 1,000 tokens | 0.3925 | 0.4064 | strictly greater |
| median metadata overhead | 0.9242 | 0.9222 | improved |

All quality rows retained budget compliance, deterministic output, valid
relations, zero forbidden violations, and the existing MCP/Evidence contract.
Attribution remained 55 retrieval-missing, 35 packing-dropped, and 5 packed
labels, so metadata pruning did not alter retrieval or candidate selection.

The candidate therefore **passed the strict positive gate**. The v0.14
quality JSON and Markdown reports are recorded as the new candidate baseline
under `docs/benchmarks/results-v0.14.json` and
`docs/benchmarks/results-v0.14.md`.

## Artifact hashes

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `21EECE8049B12D42299A257A95B6C460E5A5A09F80B33568F5F284384FEB7E5B` |
| candidate quality Markdown | `8ACCC4828C3E5A6050107CA9A9C1314B47E5D79E5B0355C364C0B958F6D5B137` |
| candidate attribution JSON | `425DD767BCDEF04E791C31065616A94468BE49B4D54D576F82FB9232F7826DA2` |
| candidate attribution Markdown | `7B82CF947316C5C6F1AEB32DBE47F773D3D228433AF260701ED2CBE948D2567C` |
| candidate diagnosis JSON | `1DEA51B6EA2A6593DA2771D3AEEE7C3393B5484EFEB5D1195B58E3923B794287` |
| candidate diagnosis Markdown | `5C7F22E42EA5245D9701EC2D6DE43EC73C45B9DDDA432967A6EC3A7F14D131BC` |

Privacy scanning found zero exact matches for absolute local paths, usernames,
source/content sentinels, secrets, `NaN`, and `Infinity`. Temporary benchmark
reports, the workspace, generated binaries, and writable caches are removed
after this finding is recorded.

## Retrospective

Final-packet-only pruning reduced the measured wire denominator without
changing candidate selection or any required Evidence field. The positive
gate is met by a 436-token cumulative reduction and an efficiency increase
from 0.3925 to 0.4064. The race result remains unverified until a compatible
64-bit MinGW toolchain is available.
