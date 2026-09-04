# Emitted long excerpt replacement v0.26 findings

## Trace-only scope

This milestone inspected the already-generated v0.25 development result before
making any product change. A long excerpt was defined conservatively as a row
whose total Evidence content exceeds 1 KiB. The metric includes every Evidence
item in the row, so it is an upper bound on the selected excerpt itself.

## Observed excerpt rows

Fifteen history rows emitted one excerpt each:

| Case | Rows | Total Evidence content per row |
|---|---:|---:|
| dotnet-structural-registry | 5 | 228 bytes |
| jsts-search-integration | 5 | 124 bytes |
| cpp-extractor-registry | 5 | 41 bytes |

The maximum upper bound was 228 bytes, far below the 1 KiB precondition.
Existing focused segment generation already combines a declaration prefix with
query-hit windows and has dedicated late-hit/source-line tests.

## Decision

No qualifying emitted-long-excerpt row exists. Per the plan's precondition, no
RED/GREEN product work and no candidate benchmark were run. The v0.25 baseline
therefore remains unchanged: history wire 11,693, UTF-8 model-visible bytes
32,494, useful Evidence 5, efficiency 0.4276, and selected fidelity
10 / 15 / 30 / 0.

This is a no-op finding, not evidence that excerpt replacement can never help
on a different corpus. It should be reconsidered only after development trace
shows an actually emitted excerpt above the threshold; the rejected v0.13
additional-variant design must not be retried.
