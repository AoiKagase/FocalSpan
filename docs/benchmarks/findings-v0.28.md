# Guidance-funded fidelity fallback v0.28 findings

## Scope and execution

The candidate kept legacy selected handles fixed, rebuilt guidance from a
single higher-fidelity variant, and adopted a candidate only when Evidence
tokens increased while completed packet wire remained at or below legacy wire.
The search was bounded by selected items and their existing variants.

RED/GREEN tests covered an affordable verbatim promotion, stale self-guidance
removal, handle preservation, strict wire ceiling, deterministic selection,
and exact fallback for an unaffordable source. The candidate passed 711 tests
in 46 packages, `go vet ./...`, and `git diff --check`.

The history benchmark was run exactly once on the candidate worktree. It was
recorded as `c63cb12` without further source changes and removed by ordinary
revert `9d6a11b` after the no-op gate. As in v0.27, the generated report names
the preceding transition HEAD because the candidate commit followed the
one-shot measurement; the measured diff and candidate commit are identical.

## Candidate result

All 48 history rows were byte-identical to the accepted baseline:

| Metric | v0.27 reverted baseline | v0.28 candidate |
|---|---:|---:|
| useful Evidence | 5 | 5 |
| cumulative estimated wire | 11,693 | 11,693 |
| efficiency | 0.4276 | 0.4276 |
| UTF-8 model-visible bytes | 32,494 | 32,494 |
| packet JSON / summary bytes | 31,090 / 1,404 | 31,090 / 1,404 |
| Evidence content / guidance bytes | 3,459 / 4,010 | 3,459 / 4,010 |
| envelope/metadata bytes | 23,621 | 23,621 |
| selected fidelity | 10 / 15 / 30 / 0 | 10 / 15 / 30 / 0 |

Comparison was compatible with zero regressions. The strict fallback prevented
the v0.16 row regressions, but no real history row had enough post-pruning
headroom for a fidelity promotion.

## Decision

Rejected as no-op and reverted. The fallback is safe in focused unit cases but
adds product complexity without changing the measured corpus. Reconsider only
after trace identifies a concrete selected signature whose higher fidelity and
recomputed guidance fit under its completed legacy packet wire.

After revert, 709 tests in 46 packages and `go vet ./...` passed. The accepted
baseline remains wire 11,693, bytes 32,494, useful 5, and efficiency 0.4276.
