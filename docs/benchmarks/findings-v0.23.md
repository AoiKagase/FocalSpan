# Metadata pruning phase 2 v0.23 findings

## Scope and execution

The candidate extended final-packet-only metadata pruning in two bounded ways:

- omit `language` when a supported path extension uniquely identifies it; and
- omit `exact_symbol` and `qualified_symbol` reasons from target/change items
  whose symbol identity is already present.

Ambiguous and mixed-language extensions retain `language`. Retrieval, ranking,
packing, source, relations, selected handles, guidance, public MCP/CLI,
`focalspan.context.v1`, and known-handle behavior were unchanged.

## Static verification

- focused Evidence, app, and MCP tests passed;
- `go test ./... -count=1` passed;
- `go vet ./...` passed with a repository-local Go cache; and
- `git diff --check` passed.

## Candidate benchmark

The history candidate was measured once after implementation. It remained
compatible with zero comparison regressions and improved 40 individual rows.

| Metric | v0.21 control | v0.23 candidate | Result |
|---|---:|---:|---|
| useful Evidence | 5 | 5 | unchanged |
| cumulative estimated wire | 12,304 | 11,983 | -321 |
| useful Evidence / 1,000 tokens | 0.4064 | 0.4173 | improved |
| UTF-8 model-visible bytes | 34,280 | 33,414 | -866 |
| packet JSON bytes | 31,956 | 31,090 | -866 |
| summary bytes including separator | 2,324 | 2,324 | unchanged |
| Evidence content value bytes | 3,459 | 3,459 | unchanged |
| guidance value bytes | 4,010 | 4,010 | unchanged |
| envelope/metadata bytes | 24,487 | 23,621 | -866 |
| selected fidelity | 10 / 15 / 30 / 0 | 10 / 15 / 30 / 0 | unchanged |

The focused/2048 diagnosis was also unchanged: `path_scope_missing=9`,
`symbol_match_missing=2`, `packing_dropped=7`, and packed Evidence `1`.

## Gate decision

Accepted. Both estimator-independent bytes and estimated wire decreased
strictly, all 40 changed rows improved, useful Evidence and selected fidelity
were preserved, and the comparison reported `compatible=true` with zero
regressions. The improvement is metadata-only and does not claim to resolve the
remaining retrieval or packing misses.
