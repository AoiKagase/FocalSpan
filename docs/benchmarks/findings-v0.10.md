# FocalSpan Evidence Compiler Efficiency v0.10 Findings

## Scope and execution

v0.10 measured one bounded Evidence-compiler candidate: omit duplicate or
fully contained same-path spans only when they add no new symbol or relation
identity. The run used the eight-case `focalspan-history-v0.5` suite, the
`default` profile, repeat 1, and enabled attribution and diagnosis. The
candidate completed on its first invocation; no infrastructure retry was
needed. Reports contain only source-free identities and measurements.

Static verification completed before the gate:

- `go test ./... -count=1`: 690 test functions across 42 packages while the
  candidate was present; no failures;
- `go vet ./...`: no issues;
- `git diff --check`: no issues;
- focused Evidence/MCP/app/benchmark tests: all four packages passed; and
- CGO-free builds: native, Windows amd64, Linux amd64, and Darwin arm64 all
  completed successfully.

The post-revert verification also passed `go test ./... -count=1` with 687
test functions across 42 packages, `go vet ./...`, `git diff --check`, and the
same focused package suite. `go test -race ./...` is **UNVERIFIED** because the
local MinGW toolchain reports `cc1.exe: sorry, unimplemented: 64-bit mode not
compiled in`.

## Frozen gate result

The gate was evaluated for `full-evidence-focused` at budget 2048. The
`packing_dropped` count is reported separately because this compiler-only
milestone does not claim retrieval improvement.

| Metric | Baseline | Candidate | Gate |
|---|---:|---:|---|
| `path_scope_missing` labels | 9 | 9 | no regression; no retrieval claim |
| `packing_dropped` labels (focused/2048) | 7 | 7 | no regression |
| packed labels (all profiles) | 5 | 5 | retain all existing labels |
| useful evidence | 5 | 5 | strict efficiency improvement |
| estimated cumulative wire tokens | 12,740 | 12,740 | no increase |
| useful evidence / 1,000 tokens | 0.3925 | 0.3925 | strictly greater than 0.3925 |

The candidate therefore failed the positive efficiency gate: it neither
reduced wire tokens nor increased useful evidence efficiency. Quality rows
remained valid (48 rows, zero invalid cases, zero forbidden violations, and
budget/deterministic checks passing), and existing packed labels were not
lost. The production candidate commits were reverted in reverse order:
`499d449` reverts `ffce23b`, then `e99c3e6` reverts `8b8702d`. This negative
result is not a new quality baseline.

## Artifact hashes

The measured temporary reports were hashed before cleanup:

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `B52E817ED59FDCAC06D69566EFEBF6A1F92443E7DE60AE3E8D269A19BF55F1C8` |
| candidate quality Markdown | `75B89AD87395618D7FD1046CB391BC4C92F15F908A03B6BEE223EB2719E660B6` |
| candidate attribution JSON | `425DD767BCDEF04E791C31065616A94468BE49B4D54D576F82FB9232F7826DA2` |
| candidate attribution Markdown | `7B82CF947316C5C6F1AEB32DBE47F773D3D228433AF260701ED2CBE948D2567C` |
| candidate diagnosis JSON | `1DEA51B6EA2A6593DA2771D3AEEE7C3393B5484EFEB5D1195B58E3923B794287` |
| candidate diagnosis Markdown | `5C7F22E42EA5245D9701EC2D6DE43EC73C45B9DDDA432967A6EC3A7F14D131BC` |

Privacy scanning found no absolute local path, username, source/content
sentinel, secret sentinel, NaN, or Infinity in the reports. Temporary reports,
build outputs, caches, and logs were removed after recording these hashes.

## Retrospective

Source-free observation plumbing and same-path containment checks were
deterministic and preserved Evidence/MCP, source-fidelity, budget,
known-handle, and privacy invariants. On the historical suite, however, the
bounded candidate did not change the selected labels or token denominator.
The next hypothesis should measure identity-bridge retrieval independently;
retrieval, ranking, packing, and Evidence changes must not be co-tuned.
