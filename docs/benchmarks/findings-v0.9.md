# FocalSpan File-Scope Retrieval v0.9 Findings

## Scope and execution

v0.9 tested bounded file-level aggregation over the existing SQLite index. The
candidate used the eight-case `focalspan-history` suite, the `default` profile,
and repeat 1. The first invocation returned no observable completion or output;
after recording that infrastructure failure, one permitted retry completed and
its artifacts are the candidate measured below. No source text or repository
execution was used.

Static checks before the candidate completed successfully:

- `go test ./... -count=1`: 697 tests in 46 packages;
- `go vet ./...`: no issues; and
- `git diff --check`: no issues.

## Frozen gate result

The gate was evaluated only for `full-evidence-focused` at budget 2048.

| Metric | Baseline | Candidate | Gate |
|---|---:|---:|---|
| `path_scope_missing` labels | 9 | 9 | at least 3 advances |
| packed labels (all profiles) | 5 | 5 | at least 1 new label |
| useful evidence | 5 | 5 | strict efficiency improvement |
| estimated cumulative wire tokens | 12,740 | 13,028 | denominator |
| useful evidence / 1,000 tokens | 0.3925 | 0.3838 | strict improvement |

The candidate therefore failed all three positive gate conditions. Existing
packed labels and quality invariants remained unchanged, but wire tokens rose
for 48 quality rows (for example, PHP 258→293 and MCP Evidence 260→269).
The candidate production commits were reverted in reverse order; this negative
result is not a new quality baseline.

## Artifact hashes

The measured temporary reports were hashed before cleanup:

| Artifact | SHA-256 |
|---|---|
| baseline quality JSON | `12FFDC70BDD44FCED08B314DEB2DE5365A1009567CC4371366E8757970D2B93C` |
| baseline quality Markdown | `3259EEAC40ACBACB367FFF62B9DD033761D1CBFD2C68984AF9B66F7584168F58` |
| baseline attribution JSON | `425DD767BCDEF04E791C31065616A94468BE49B4D54D576F82FB9232F7826DA2` |
| baseline attribution Markdown | `7B82CF947316C5C6F1AEB32DBE47F773D3D228433AF260701ED2CBE948D2567C` |
| baseline diagnosis JSON | `1DEA51B6EA2A6593DA2771D3AEEE7C3393B5484EFEB5D1195B58E3923B794287` |
| baseline diagnosis Markdown | `5C7F22E42EA5245D9701EC2D6DE43EC73C45B9DDDA432967A6EC3A7F14D131BC` |
| candidate quality JSON | `9DF4A88A9F0357E672821A014DAB30E0FA57694F4F1396EEA8CD0DCF110D0AC4` |
| candidate quality Markdown | `3A63A5142D52CB54E258A95D08CC97052D165FE0304AF526636F7DBEBBF867B7` |
| candidate attribution JSON | `9B3B494AF9DCAB7A456D84CB52B6DC6A7E1B734BCA438308A002BD5A2015F9D0` |
| candidate attribution Markdown | `61828BF70585D5BF993795E276567DDD56A5CBC32A1B669C9A8365A9DBD6F145` |
| candidate diagnosis JSON | `6D7E735841D0CD9E953162BED4628793234367760B51274D18821208B9F15D00` |
| candidate diagnosis Markdown | `4D285FE325135E8C7BDFF1A6C64CC78AA98CE4D41F49E03328CAFBA8F25A4885` |

Privacy scanning of the reports found no absolute local path, source/content
sentinel, username, secret sentinel, NaN, or Infinity. Temporary reports and
benchmark workspaces were removed after recording these hashes.

## Retrospective

The implementation proved deterministic, bounded file grouping and preserved
normal Evidence/MCP contracts, but aggregate scope did not move the selected
historical labels. The next retrieval hypothesis needs a stronger bridge from
file scope to the intended symbol or a separately measured ranking/packing
change; v0.9's file aggregation is not promoted.
