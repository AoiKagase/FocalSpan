# MCP text summary minimization v0.24 findings

## Scope and execution

The Evidence MCP summary changed from a prose sentence to the fixed short form
`items=N tokens=used/limit omitted=N`. It retains all four numeric values while
removing words and punctuation duplicated by structured content. Packet JSON,
status/restart summaries, retrieval, ranking, packing, source, relations,
guidance, public MCP/CLI, `focalspan.context.v1`, and known handles were
unchanged.

## RED and static verification

- the updated canonical summary and live MCP session tests first failed on the
  legacy `FocalSpan evidence:` form;
- focused Evidence and MCP tests passed after the one-line implementation;
- `go test ./... -count=1` passed;
- `go vet ./...` passed with a repository-local Go cache; and
- `git diff --check` passed.

## Candidate benchmark

The eight-case history candidate was measured exactly once. Comparison against
the frozen quality control reported `compatible=true`, zero regressions, and
40 row-level wire improvements.

| Metric | v0.23 control | v0.24 candidate | Result |
|---|---:|---:|---|
| useful Evidence | 5 | 5 | unchanged |
| cumulative estimated wire | 11,983 | 11,693 | -290 |
| useful Evidence / 1,000 tokens | 0.4173 | 0.4276 | improved |
| UTF-8 model-visible bytes | 33,414 | 32,494 | -920 |
| packet JSON bytes | 31,090 | 31,090 | byte-identical |
| summary bytes including separator | 2,324 | 1,404 | -920 |
| Evidence content value bytes | 3,459 | 3,459 | unchanged |
| guidance value bytes | 4,010 | 4,010 | unchanged |
| envelope/metadata bytes | 23,621 | 23,621 | unchanged |
| selected fidelity | 10 / 15 / 30 / 0 | 10 / 15 / 30 / 0 | unchanged |

The focused/2048 diagnosis remained unchanged: `path_scope_missing=9`,
`symbol_match_missing=2`, `packing_dropped=7`, and packed Evidence `1`.
Generated development reports contained no user absolute path or credential
marker found by the privacy scan.

## Gate decision

Accepted. Packet JSON and quality were unchanged, every affected row improved,
and both estimator-independent summary bytes and cumulative estimated wire
decreased strictly. Unit and live-session tests preserve the numeric and
source-free contracts.
