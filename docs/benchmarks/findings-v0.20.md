# schema v1/v2 relation-linker comparator v0.20 findings

## Scope and execution

v0.20 added an opt-in, test-only comparator for the v0.19 relation-linking
performance gates. It recreates the pre-v0.19 all-symbol/all-relation scan and
per-relation write path without restoring that path to production code. Each
v1/v2 scenario uses independently seeded but identical current-scale fixtures
inside one test process.

The fixture contains 450 files, 5,000 symbols, 28,000 relations, and 21,676
unresolved relations. Final relation rows are compared byte-for-byte after JSON
serialization and also recorded by a source-free SHA-256 digest.

## Comparator result

| Scenario | v1 | v2 | Speedup | v1 candidates | v2 candidates | Absolute gate | Ratio gate |
|---|---:|---:|---:|---:|---:|---|---|
| unchanged | `50.6104576s` | timer-resolution `0s` | greater than measurable | 28,000 | 0 | `<=250ms` pass | `>=10x` pass |
| small related | `50.6633834s` | `362.3495ms` | `139.82x` | 28,000 | 63 | `<=1s` pass | `>=5x` pass |
| full | `48.3201814s` | `136.1627ms` | `354.87x` | 28,000 | 21,676 | `<=5s` pass | `>=2x` pass |

All scenarios produced 28,000 identical relation rows with digest
`1cd73ec9ecd76507f8df20835070d2c2a978a5f989b8f3cea0bfb024c2ef7c2b`.
The initial complete comparator invocation passed. Its final full-scenario line
was compacted by the command-output layer, so the full subtest alone was rerun
to recover the exact v1/v2 values shown above; no failed gate was retried.

## Verification

- deterministic duration-ratio and relation-row helper tests passed;
- `go test ./... -count=1` passed;
- `go vet ./...` passed; and
- `git diff --check` passed.

The comparator is skipped unless `FOCALSPAN_SCHEMA_V2_COMPARATOR=1` is set.
Production linker behavior, public MCP/CLI interfaces, Evidence wire, ranking,
packing, and the v0.15 token baseline are unchanged.

## Outcome

The previously unverified TASKS.md ratios are now independently demonstrated.
v0.19 therefore satisfies both its absolute current-scale gates and the
10x/5x/2x comparative gates. The next milestone may proceed to benchmark
hardening for token-efficiency work.
