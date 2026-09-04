# Token benchmark hardening v0.21 findings

## Scope

v0.21 added estimator-independent UTF-8 byte accounting to development
benchmark results. The exact model-visible payload is split into compact packet
JSON, the newline plus canonical summary, serialized Evidence content values,
guidance values, and remaining envelope/metadata bytes. Selected fidelity
counts are recorded without source bodies.

The existing Evidence acceptance fixture already exercised a positive initial
target, an actually selected long late-hit excerpt, and a known-handle
expansion. A deterministic no-result case was added. It records the current
baseline as an abstention miss: one Evidence item, 245 estimated wire tokens.
No relevance filter was added in this measurement-only milestone.

## Historical suite result

The eight-case, 48-row history suite ran once with the default profiles,
repeat 1, attribution, and diagnosis enabled. Comparison with v0.15 returned
`compatible=true` and `regressions=0`.

| Metric | Result |
|---|---:|
| useful Evidence | 5 |
| estimated cumulative wire tokens | 12,304 |
| useful Evidence per 1,000 tokens | 0.4064 |
| UTF-8 model-visible bytes | 34,280 |
| compact packet JSON bytes | 31,956 |
| summary bytes including separator | 2,324 |
| Evidence content value bytes | 3,459 |
| guidance value bytes | 4,010 |
| remaining envelope/metadata bytes | 24,487 |
| selected fidelity | 10 verbatim / 15 excerpt / 30 signature / 0 synthetic |

The byte components sum exactly to model-visible bytes. At focused/2048 the
existing aggregate remains eight valid cases, median wire 252, median metadata
overhead 0.9222, budget/deterministic/relation validity 1, and forbidden
violations 0.

## Verification and outcome

- byte-breakdown RED/GREEN tests passed;
- no-result and existing positive/long/delta Evidence fixture tests passed;
- normal MCP responses reject the new development-only field names;
- focused benchmark, Evidence evaluation, and MCP tests passed;
- the history comparison reported zero regressions.

The measurement shows that only 3,459 of 34,280 UTF-8 bytes are Evidence
content values, while envelope/metadata and guidance dominate. It also proves
that long excerpts are selected in the current corpus. v0.22 can therefore
evaluate bounded combination packing without relying only on the product token
estimator.
