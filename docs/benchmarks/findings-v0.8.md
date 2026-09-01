# Failure-layer attribution v0.8 findings

## Scope and execution

FocalSpan v0.8 adds a development-only diagnosis report over the existing
source-free attribution trace. It does not change query normalization,
retrieval, RRF, ranking, linking, packing, token accounting, normal CLI/MCP
output, or `focalspan.context.v1`.

The frozen measurement ran at commit
`8518fe82aa114c3307a197603675d879e4dbe838` on 2026-09-01 UTC. Before the
measurement:

- `go test ./... -count=1` passed 683 tests in 46 packages;
- `go vet ./...` reported no issues;
- `git diff --check` reported no issues; and
- `focalspan-bench validate` reported 8 cases and 0 invalid.

The eight-case `default`-profile benchmark ran exactly once with `--repeat 1`
and zero retries. It produced 48 quality results, 40 Evidence-profile
attribution results, and 40 matching diagnosis results. Comparison against
`docs/benchmarks/results-v0.5.json` was compatible with zero regressions.

## Attribution compatibility

The existing `focalspan.benchmark-attribution.v1` result remained exactly at
the accepted v0.6 stage counts:

| Attribution stage | Labels |
|---|---:|
| `retrieval_missing` | 55 |
| `packing_dropped` | 35 |
| `packed` | 5 |
| `label_not_indexed` | 0 |
| `linking_unresolved` | 0 |
| `ranking_dropped` | 0 |

All 95 diagnosis labels matched the corresponding v1 identity and copied
attribution stage. Every one of the 55 retrieval-missing rows mapped to exactly
one upstream diagnosis layer: 45 `path_scope_missing` and 10
`symbol_match_missing`.

## Diagnosis result

| Expectation | Path scope missing | Symbol match missing | Packing dropped | Packed |
|---|---:|---:|---:|---:|
| Required path | 20 | 0 | 15 | 5 |
| Required symbol | 20 | 5 | 15 | 0 |
| Expansion anchor | 5 | 5 | 5 | 0 |
| **Total** | **45** | **10** | **35** | **5** |

The frozen next-layer rule considers the four unmet layers only in
`full-evidence-focused` at budget 2048:

| Layer | Labels |
|---|---:|
| `path_scope_missing` | 9 |
| `symbol_match_missing` | 2 |
| `ranking_dropped` | 0 |
| `packing_dropped` | 7 |

The maximum is 9, so the one selected next primary layer is
`path_scope_missing`. No tie-break was required.

## Output integrity

The six temporary reports parsed successfully and used LF output. Scans found
zero source/content fields, absolute paths, username occurrences, environment
names or selected environment values, secret sentinels, NaN, or Infinity.
Their SHA-256 hashes were:

| Temporary report | SHA-256 |
|---|---|
| quality JSON | `8888f20220592ae24926f0958e836162a6fb13ebbb188829fba53c85f79e0ad9` |
| quality Markdown | `f69660a38d309afa174edb5c486fc10a5f86e6b68f3a812659d153c9b49d1f15` |
| attribution JSON | `425dd767bcdef04e791c31065616a94468be49b4d54d576f82fb9232f7826da2` |
| attribution Markdown | `7b82cf947316c5c6f1aeb32dbe47f773d3d228433af260701ed2cbe948d2567c` |
| diagnosis JSON | `1dea51b6ea2a6593da2771d3aeee7c3393b5484efeb5d1195b58e3923b794287` |
| diagnosis Markdown | `5c7f22e42ea5245d9701ec2d6de43ec73c45b9ddda432967a6ec3a7f14d131bc` |

The reports are measurement intermediates, not a new baseline, and were
removed after these hashes and counts were recorded. No `results-v0.8` file is
created.

## Local closure verification

Fresh closure verification passed 683 tests in 46 packages and `go vet ./...`;
`git diff --check` reported no issues. The focused diagnosis/attribution/privacy
selection passed 16 tests, and the app/eval/Evidence/MCP/search contract suite
passed 205 tests in 5 packages.

With `CGO_ENABLED=0`, native `focalspan` and `focalspan-bench` builds succeeded,
as did `focalspan` builds for Windows amd64, Linux amd64, and Darwin arm64.
The ignored build directory and all five binaries were removed afterward.
Diff inspection from `77f42cf` found no changes to attribution v1, search,
rank, or budget production code. Changes under app, Evidence, and MCP were
tests only. Final status contained only the pre-existing `AGENTS.md` change and
user-owned untracked `.focalspan.json` and `TASKS.md`.

## Limitations

This milestone identifies the earliest measured failure boundary; it does not
show that any retrieval or packet result improved. A same-path raw candidate
only establishes that file scope was reached, not that the intended symbol can
be extracted, matched, ranked, or packed. The selected `path_scope_missing`
layer requires a separately planned production hypothesis and acceptance gate.
Remote CI remains unverified until the closure commits are pushed and the
actual workflow jobs are inspected.
