# FocalSpan Candidate Attribution and Coverage Findings v0.6

## Frozen starting state

Task 1 was measured on 2026-08-31 UTC from commit
`6582dc56b3f1a972f60b8671c8c5718c80588d22`. The branch was `master`, two
documentation commits ahead of fetched `origin/master`; no tracked or staged
change existed before this record. The pre-existing untracked
`.focalspan.json` was not read, modified, staged, or committed.

- Environment: Go 1.27.0, Windows amd64, `CGO_ENABLED=1`.
- Frozen v0.5 quality object:
  `f914facbfbf55c450fd26769bdc7bd6a992112dc`.
- Byte-identical v0.5 plan archive object:
  `281211f6754bd3b1e45a7b321d8aaab1a1a27094`.
- `go test ./... -count=1`: exit 0, 657 tests in 46 packages.
- `go vet ./...`: exit 0, no issues reported.
- `git diff --check`: exit 0.
- Public suite validation: 8 cases, 0 invalid.
- Development smoke: `php-extractor-integration` and
  `cpp-extractor-registry`, default profiles, repeat 1, 2 cases and 12 quality
  results.
- v0.5 comparison: exit 0, compatible true, 0 regressions.

The temporary candidate JSON, Markdown, indexes, and workspace were removed
after comparison. No eight-case quality run was performed in Task 1.

## Attribution measurement

Task 4 added an opt-in development-only trace and exact-label join without
changing normal CLI, MCP, or Evidence Packet output. Focused RED tests first
failed for the absent runner and benchcli attribution APIs. The implementation
then passed 61 benchmark/benchcli tests and 665 full repository tests in 46
packages; `git diff --check` passed.

The bounded two-case repeat-1 smoke ran once after integration:

- Cases: `php-extractor-integration`, `cpp-extractor-registry`.
- Profiles: `default`; repeat: 1.
- Quality: 2 cases, 12 results; v0.5 comparison compatible true, 0 regressions.
- Attribution: 25 labels, all `retrieval_missing`.
- Privacy scan: no source field/text sentinel, absolute path, username, secret
  sentinel, environment value, NaN, or Infinity.

The single planned diagnostic then ran once, with no retry:

- Reason: classify the frozen required paths, required symbols, and expansion
  anchors before selecting any production change.
- Scope: 8 cases, `default` profiles, repeat 1; 48 quality results.
- Attribution: 40 Evidence-profile results and 95 labels. Legacy profiles
  received no fabricated attribution.
- v0.5 quality comparison: compatible true, 0 regressions.
- Terminal stages: 55 `retrieval_missing`, 35 `packing_dropped`, 5 `packed`,
  and zero `label_not_indexed`, `linking_unresolved`, or `ranking_dropped`.

| Expectation | Retrieval missing | Packing dropped | Packed |
|---|---:|---:|---:|
| Required path | 20 | 15 | 5 |
| Required symbol | 25 | 15 | 0 |
| Expansion anchor | 10 | 5 | 0 |

Artifacts are `attribution-v0.6.json` and `attribution-v0.6.md`; their Git blob
IDs before commit are `a90fb06e00db7a6f9240fa9935ff1c823b1f5878` and
`e61943f97ea06edf5b7d1dd8fd47df6de87fe6cc`. Both parse/render with LF endings
and contain no source, absolute path, username, environment value, secret
sentinel, NaN, Infinity, or normal-output debug field. Temporary candidate
quality files, indexes, and workspaces were removed.

These counts are measurements, not a selected optimization. The Task 5
Decision Log commit must exclude the zero `label_not_indexed` rows, compare the
actionable retrieval and linking misses under the predeclared rule, freeze one
coherent subset and numeric gates, and contain no production change.

## Frozen improvement decision

The predeclared selection rule chooses retrieval: there are 55 actionable
`retrieval_missing` rows and zero `linking_unresolved` rows. The selected defect
is narrower than all 55 rows. In `php-extractor-integration`, the normalized
query contains the general lexical term `index`, while the path retriever is fed
only explicit path-shaped terms. Consequently the indexed `Run` candidate in
`internal/indexer/indexer.go` never appears in any raw retriever list.

Freeze these 12 baseline rows, all currently `retrieval_missing`:

- Case: `php-extractor-integration`.
- Labels: required path `internal/indexer/indexer.go`; required symbol
  `internal/indexer/indexer.go::Run`; expansion anchor
  `internal/indexer/indexer.go::Run` with relation `callers`.
- Profiles/budgets: `full-evidence-focused` at 1024, 2048, and 4096; and
  `no-relations-evidence-focused` at 2048.
- Excluded by design: `fts-evidence-focused`, because it does not execute the
  path retriever.

The one selected implementation is to let the existing bounded path retriever
consume normalized lexical path hints in addition to explicit path tokens.
Production/test scope is exactly `internal/search/retrieval.go` and
`internal/search/retrieval_test.go`. The implementation must remain
corpus-independent and must not change query parsing, FTS, retriever weights,
fusion/ranking, relation resolution, packing, Evidence, labels, profiles,
budgets, or attribution thresholds.

Frozen numeric gates:

- Selected retrieval misses decrease from 12 to at most 11, with at least one
  selected label advancing beyond retrieval.
- At least one of the four selected expansion-anchor rows becomes `packed`, so
  at least one previously blocked expansion becomes executable.
- No selected row becomes `label_not_indexed`, and no label in the 95-row
  diagnostic moves to an earlier terminal stage.
- Required-path/symbol recall or executable-anchor count increases for the
  affected case; the v0.5 quality comparison remains compatible with zero
  regressions.
- Legacy fixtures, Evidence/wire/privacy invariants, deterministic output,
  full tests, vet, diff check, and bounded run-count rules remain green.

Rejected alternatives are intentionally not co-tuned: linker work has zero
measured rows; the 35 packing drops are downstream and Evidence compaction is
out of scope; raising FTS/fusion limits or changing ranking weights is broader
and does not target this exact missing path signal; parser changes and
corpus-specific aliases would violate the milestone constraints. The other
retrieval-missing cases do not share the same directly observed lexical path
hint and are not part of this hypothesis.
