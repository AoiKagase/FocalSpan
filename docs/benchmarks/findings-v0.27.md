# Structural constructor micro-retriever v0.27 findings

## Scope and execution

The candidate was limited to registry assembly/registration vocabulary. It
probed the existing exact-symbol store once for `NewWithConfig`, with limit 8,
and exposed a private `structural-constructor` attribution identity. It did not
reuse the v0.11 path bridge, lexical seeds, or store changes.

RED tests fixed positive/negative triggers, explicit-path suppression, the
single probe and cap, deterministic trace identity, and attribution validation.
The GREEN candidate passed focused tests, 712 full tests in 46 packages,
`go vet ./...`, and `git diff --check`.

The history benchmark was run exactly once on the candidate worktree. The
candidate was then recorded as commit `9ff32b2` without further source changes
and removed by ordinary revert commit `c6878a6` after the gate failed. The
generated report records the preceding transition HEAD because measurement
occurred before the candidate commit; the measured worktree diff and `9ff32b2`
content are identical.

## Diagnosis gate

At focused/2048, the candidate moved four path-scope labels forward. The Rust
path and symbol were packed, while the C++ path and symbol only moved into the
packing-dropped layer.

| Diagnosis | v0.26 control | v0.27 candidate | Result |
|---|---:|---:|---|
| path_scope_missing | 9 | 5 | -4 |
| symbol_match_missing | 2 | 2 | unchanged |
| packing_dropped | 7 | 9 | +2 regression |
| packed | 1 | 3 | +2 |

## Token and quality gate

| Metric | v0.26 control | v0.27 candidate | Result |
|---|---:|---:|---|
| useful Evidence | 5 | 13 | +8 |
| cumulative estimated wire | 11,693 | 11,925 | +232 fail |
| useful Evidence / 1,000 tokens | 0.4276 | 1.0901 | improved |
| UTF-8 model-visible bytes | 32,494 | 33,170 | +676 fail |
| packet JSON bytes | 31,090 | 31,766 | +676 |
| summary bytes | 1,404 | 1,404 | unchanged |
| Evidence content bytes | 3,459 | 3,667 | +208 |
| guidance bytes | 4,010 | 4,298 | +288 |
| envelope/metadata bytes | 23,621 | 23,801 | +180 |
| selected fidelity | 10 / 15 / 30 / 0 | 6 / 15 / 34 / 0 | changed |

The frozen comparison was compatible with zero quality regressions and
project-metadata did not regress, but those facts do not override the explicit
packing and wire gates. Generated reports passed the privacy marker scan.

## Decision

Rejected and reverted. Although the narrow probe improved Rust labels, it
increased packing drops and model-visible bytes and failed to pack the C++
constructor. Do not retry a `NewWithConfig` assembly-word probe without a new
packing-independent mechanism and new evidence.

After revert, 709 tests in 46 packages and `go vet ./...` passed. The accepted
baseline remains wire 11,693, bytes 32,494, useful Evidence 5, efficiency
0.4276, and fidelity 10 / 15 / 30 / 0.
