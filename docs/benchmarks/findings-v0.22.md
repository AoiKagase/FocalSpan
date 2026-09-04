# Bounded beam Evidence packing v0.22 findings

## Scope and execution

v0.22 tested a width-8, maximum-depth-6 combination search over the existing
Evidence candidates and fidelity variants. The current greedy compiler was
retained as a per-request control. A beam packet could be selected only when
its final selected count and measured model-visible wire were no greater than
the control and its internal utility was strictly higher.

Retrieval, ranking, variant generation, guidance rules, public MCP/CLI,
`focalspan.context.v1`, and known-handle behavior were unchanged. The candidate
was recorded in commit `016fe5f` and, after the gate failed, removed by ordinary
revert commit `22813d6`.

## Static verification

- RED tests failed because beam/control helpers did not exist;
- GREEN combination, ceiling, and fallback tests passed; and
- focused Evidence, app, and MCP package tests passed.

The full repository suite and vet were not run on the candidate because the
historical gate had already rejected it. They are run on the restored baseline
during closure instead.

## Candidate gate

The eight-case history suite ran once with 48 rows, default profiles, repeat 1,
attribution, and diagnosis enabled. v0.15 comparison returned
`compatible=true`, `regressions=0`, but the candidate did not change any
selected packet.

| Metric | v0.21 control | v0.22 candidate | Gate |
|---|---:|---:|---|
| focused/2048 `packing_dropped` | 7 | 7 | reduce by at least 3: fail |
| focused/2048 packed | 1 | 1 | retain: pass |
| useful Evidence | 5 | 5 | increase: fail |
| cumulative estimated wire | 12,304 | 12,304 | no increase: pass |
| useful Evidence / 1,000 tokens | 0.4064 | 0.4064 | greater than 0.4064: fail |
| UTF-8 model-visible bytes | 34,280 | 34,280 | no increase: pass |
| selected fidelity | 10 / 15 / 30 / 0 | 10 / 15 / 30 / 0 | unchanged |

The focused/2048 diagnosis remained 9 path-scope missing, 2 symbol-match
missing, 7 packing-dropped, and 1 packed label. The strict per-request control
ceiling prevented regressions but also left no qualifying beam combination in
the historical suite.

## Outcome

The bounded beam hypothesis is rejected without a second benchmark. v0.21
remains the active product baseline. Further packing work must not repeat the
same utility objective and strict control ceiling unchanged; metadata pruning
phase 2 is the next independent candidate.
