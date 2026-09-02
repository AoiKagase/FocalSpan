# FocalSpan Adaptive Focused Excerpt v0.13 Findings

## Scope and execution

v0.13 tested one bounded Evidence-fidelity candidate: for long focused
content, add a private same-fidelity excerpt variant with a two-line
declaration prefix and narrower hit context. The normal excerpt remains the
first variant and is used when the adaptive source is not strictly smaller.
Retrieval, ranking, Evidence/MCP schemas, CLI methods, relations, and
`known_handles` were unchanged.

The historical `focalspan-history-v0.5` suite ran once with the `default`
profile, repeat 1, attribution enabled, and diagnosis enabled. It completed
with 48 quality rows, 40 attribution results, and 40 diagnosis results.
Comparison with the frozen baseline reported `compatible: true` and zero
regressions.

Static verification completed before the candidate gate:

- `go test ./... -count=1`: passed;
- `go vet ./...`: passed;
- `git diff --check`: passed;
- native build: passed;
- CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds: passed; and
- `go test -race ./...`: **UNVERIFIED** because the local MinGW compiler
  reports `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

## Candidate gate status

The strict gate was evaluated against the v0.10 baseline at focused/2048.

| Metric | Baseline | Adaptive candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels | 7 | 7 | no increase |
| packed labels | 5 | 5 | retain all |
| cumulative estimated wire tokens | 12,740 | 12,740 | strictly lower |
| useful evidence | 5 | 5 | increase efficiency |
| useful evidence / 1,000 tokens | 0.3925 | 0.3925 | strictly greater |
| median metadata overhead | 0.9242 | 0.9242 | no regression |

The candidate therefore **failed the strict positive gate**. The adaptive
variant passed its focused unit and fidelity tests, but the historical cases
did not select it in a way that changed the cumulative wire denominator or
useful evidence. This result is not a new quality baseline; the v0.10 product
baseline remains active.

All quality rows retained budget compliance, deterministic output, valid
relations, and zero forbidden violations. The attribution and diagnosis
outputs used their existing schemas and contained no source body.

## Artifact hashes

The six temporary reports were hashed before cleanup:

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `39F125B2BCE9E1A514A82063DB0B02D5A62F9382397B17AFEA0BEF5886F588FF` |
| candidate quality Markdown | `10879E0D3A0299819E4476475FCAEC931494DDACC0BFE1088CFE398E6A8B1087` |
| candidate attribution JSON | `425DD767BCDEF04E791C31065616A94468BE49B4D54D576F82FB9232F7826DA2` |
| candidate attribution Markdown | `7B82CF947316C5C6F1AEB32DBE47F773D3D228433AF260701ED2CBE948D2567C` |
| candidate diagnosis JSON | `1DEA51B6EA2A6593DA2771D3AEEE7C3393B5484EFEB5D1195B58E3923B794287` |
| candidate diagnosis Markdown | `5C7F22E42EA5245D9701EC2D6DE43EC73C45B9DDDA432967A6EC3A7F14D131BC` |

Privacy scanning found zero exact matches for absolute local paths, usernames,
source/content sentinels, secrets, `NaN`, and `Infinity`. Temporary reports,
the benchmark workspace, generated binaries, and writable caches are removed
after this finding is recorded.

## Retrospective

The adaptive line-window implementation is deterministic and preserves source
fidelity, late hits, relation endpoints, and hard budgets. The frozen
historical suite did not expose a measurable wire reduction, so the candidate
is rejected without a second benchmark or a production baseline update. A
future excerpt attempt should first identify benchmark cases that select long
focused source, then measure replacement of an actually emitted variant rather
than adding an unselected alternative.
