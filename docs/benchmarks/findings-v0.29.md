# Known-handles delta phase 2 v0.29 findings

## Scope and execution

The candidate removed two conservative early returns from private known-delta
guidance pruning. It still removed only `known_anchor_not_repeated` and known
handle `self` actions; relation edges, non-self actions, and safety limitations
were preserved.

RED/GREEN tests covered relation-action and relation-edge cases, initial packet
identity, known resend, dangling edges, and retained actionable guidance. The
candidate passed 151 focused tests, 709 full tests in 46 packages,
`go vet ./...`, and `git diff --check`.

## Candidate eval

The Evidence fixture eval was run exactly once. All contracts passed, but the
measured target did not change:

| Metric | v0.28 baseline | v0.29 candidate |
|---|---:|---:|
| median delta ratio | 0.5521958243 | 0.5521958243 |
| expected / role / fidelity / relation | 1 / 1 / 1 / 1 | 1 / 1 / 1 / 1 |
| budget / deterministic / late-hit / empty | 1 / 1 / 1 / 1 | 1 / 1 / 1 / 1 |
| forbidden violations / known resend | 0 / 0 | 0 / 0 |

The strict `<0.5521958` gate failed. This proves the newly covered synthetic
edge/action shapes are not present in the measured with-known fixture. Per the
early gate, the history candidate was not run because an unchanged multi-turn
ratio could not establish token benefit.

The candidate was recorded as `1cbb4a1` and removed by ordinary revert
`0f067c2`. The accepted history baseline remains wire 11,693, bytes 32,494,
useful Evidence 5, and efficiency 0.4276.

## Decision

Rejected as no-op. Do not relax the relation/action early-return rules again
until a real with-known packet trace demonstrates redundant guidance and the
delta fixture is extended to cover that real shape.
