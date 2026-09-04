# Relevance-aware abstention v0.25 findings

## Scope and trace evidence

Abstention applies only to initial Evidence queries without explicit path scope
or changed-only filtering. It uses existing private rank reasons and does not
change ranking weights, packing, expansion, impact, public MCP/CLI,
`focalspan.context.v1`, or known handles.

The pre-implementation no-result fixture returned one synthetic module outline
at 230 estimated tokens. Attribution showed seven FTS candidates. Every one had
`retrieval-fusion` and a lexical reason caused by pieces of the nonexistent
snake-case identifier, while symbol/path retrievers returned zero results and
no candidate had an identity reason. The implemented boundary therefore treats
lexical-only support as insufficient when the parsed query contains an
identifier. Natural-language queries may still use lexical support; symbol,
qualified-symbol, prefix, path, or changed support remains sufficient.

## RED and static verification

- the no-result eval and direct app test first failed with one retained item;
- helper tests cover lexical-only identifier queries, strong identity reasons,
  natural lexical queries, explicit path scope, and changed-only bypasses;
- all nine Evidence fixture cases passed, including eight positive cases,
  late-hit and known-handle expansion;
- `go test ./... -count=1` passed 709 tests in 46 packages;
- `go vet ./...` and `git diff --check` passed.

## Target fixture result

| Metric | v0.24 control | v0.25 candidate | Result |
|---|---:|---:|---|
| no-result Evidence items | 1 | 0 | corrected |
| no-result estimated wire | 230 | 85 | -145 |
| empty packet correct | 0 | 1 | corrected |
| positive expected coverage | 1 | 1 | unchanged |
| role / fidelity / relation validity | 1 / 1 / 1 | 1 / 1 / 1 | unchanged |
| budget / deterministic | 1 / 1 | 1 / 1 | unchanged |
| forbidden violations / known resend | 0 / 0 | 0 / 0 | unchanged |
| late-hit preservation | 1 | 1 | unchanged |
| median delta ratio | 0.5550053 | 0.5521958 | improved |

## History candidate

The eight-case history candidate was measured exactly once. It had no
abstention target row and remained byte-identical to v0.24: useful Evidence 5,
cumulative estimated wire 11,693, efficiency 0.4276, UTF-8 model-visible bytes
32,494, packet JSON 31,090, summary 1,404, and fidelity 10 / 15 / 30 / 0.
Comparison reported `compatible=true` and zero regressions.

Generated development reports contained no user absolute path or credential
marker found by the privacy scan.

## Gate decision

Accepted as a fixture-targeted improvement. The pre-trace proved an affected
row, that row became a valid smaller empty packet, every positive fixture and
history row remained non-regressed, and no public contract changed. No token
benefit is claimed for the current history suite.
