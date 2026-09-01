# FocalSpan Path-Scoped Symbol Retrieval v0.7 Findings

## Starting State

- Starting commit: `987c5d26ad588e57c86130927bd075442ddcad98`
- Branch: `master`
- Upstream divergence: `0` behind, `0` ahead of `origin/master`
- Preserved local input/state: untracked `.focalspan.json` and `PLAN_v0.7.md`

## v0.6 Measured Distribution

| Terminal stage | Labels |
| --- | ---: |
| `retrieval_missing` | 55 |
| `packing_dropped` | 35 |
| `packed` | 5 |
| `linking_unresolved` | 0 |
| `ranking_dropped` | 0 |
| `label_not_indexed` | 0 |

## Selected Four Cases

- `php-extractor-integration`
- `project-metadata-indexing`
- `jsts-search-integration`
- `mcp-evidence-output`

The selected scope contains 44 label rows across the frozen Evidence-focused
profiles and budgets defined in `PLAN.md`.

## Frozen Candidate Gate

The one candidate run must preserve v0.5 compatibility, selected-case quality,
budget, determinism, relation validity, privacy, finite values, known-handle
suppression, FTS-only behavior, and Japanese relation recall. It must advance
at least 8 of the 16 specified v0.6 retrieval-missing symbol/anchor rows,
advance both named deficient cases, pack `codeContext` at 2048, pack `Run` at
2048 with rank 10 or better for project metadata, retrieve `Run` for the PHP
case, unblock a real expansion, and improve 2048 required-symbol recall in at
least two selected cases. Scope remains capped at 8 paths, 8 candidates per
path, and 40 candidates total, with symbol-owned candidates only and no
corpus-specific production logic.

Any failed hard invariant or symbol-identity condition rejects the production
hypothesis without a second adjustment. Only a fully passing gate permits the
eight-case repeat-3 evaluation.

## Results

- `2026-08-31T15:09:29Z` — Task 0 archived the completed v0.6 plan with
  identical source/archive Git blob
  `07c2dbdb3f1eec6b2c10a03e73feb611301f479d`. The v0.7 transition passed
  `go test ./... -count=1` (666 tests, 46 packages), `go vet ./...`, and
  `git diff --check` before commit.

- `2026-08-31T15:31:26Z` — Task 1 ran the selected baseline exactly once with
  retry count zero. Validation returned 4 cases and 0 invalid; the run returned
  4 cases and 24 quality results. Comparison with v0.5 was compatible with zero
  regressions. The 16 selected Evidence results contained the frozen 44 label
  rows: 20 `retrieval_missing`, 20 `packing_dropped`, and 4 `packed`.

### Frozen 44-row attribution baseline

Profiles are abbreviated `full` and `no-rel`; retriever hits are shown as
`retriever:raw-position`. A dash means the field was absent. Repeated path-hit
lists are retained because the gate is row-specific.

| Case | Profile | Budget | Expectation | Terminal stage | Retriever hits | Ranked | Packed |
| --- | --- | ---: | --- | --- | --- | ---: | ---: |
| php-extractor-integration | full | 1024 | path `internal/indexer/indexer.go` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 1024 | symbol `internal/indexer/indexer.go::Run` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 1024 | anchor `internal/indexer/indexer.go::Run` callers | retrieval_missing | - | - | - |
| php-extractor-integration | full | 2048 | path `internal/indexer/indexer.go` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 2048 | symbol `internal/indexer/indexer.go::Run` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 2048 | anchor `internal/indexer/indexer.go::Run` callers | retrieval_missing | - | - | - |
| php-extractor-integration | full | 4096 | path `internal/indexer/indexer.go` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 4096 | symbol `internal/indexer/indexer.go::Run` | retrieval_missing | - | - | - |
| php-extractor-integration | full | 4096 | anchor `internal/indexer/indexer.go::Run` callers | retrieval_missing | - | - | - |
| php-extractor-integration | no-rel | 2048 | path `internal/indexer/indexer.go` | retrieval_missing | - | - | - |
| php-extractor-integration | no-rel | 2048 | symbol `internal/indexer/indexer.go::Run` | retrieval_missing | - | - | - |
| php-extractor-integration | no-rel | 2048 | anchor `internal/indexer/indexer.go::Run` callers | retrieval_missing | - | - | - |
| project-metadata-indexing | full | 1024 | path `internal/indexer/indexer.go` | packed | fts:13,17,36,37,49,50,52-61,71-77,86,88 | 2 | 1 |
| project-metadata-indexing | full | 1024 | symbol `internal/indexer/indexer.go::Run` | packing_dropped | fts:50 | 20 | - |
| project-metadata-indexing | full | 2048 | path `internal/indexer/indexer.go` | packed | fts:13,17,36,37,49,50,52-61,71-77,86,88 | 2 | 1 |
| project-metadata-indexing | full | 2048 | symbol `internal/indexer/indexer.go::Run` | packing_dropped | fts:50 | 20 | - |
| project-metadata-indexing | full | 4096 | path `internal/indexer/indexer.go` | packed | fts:13,17,36,37,49,50,52-61,71-77,86,88 | 2 | 1 |
| project-metadata-indexing | full | 4096 | symbol `internal/indexer/indexer.go::Run` | packing_dropped | fts:50 | 20 | - |
| project-metadata-indexing | no-rel | 2048 | path `internal/indexer/indexer.go` | packed | fts:13,17,36,37,49,50,52-61,71-77,86,88 | 2 | 1 |
| project-metadata-indexing | no-rel | 2048 | symbol `internal/indexer/indexer.go::Run` | packing_dropped | fts:50 | 20 | - |
| jsts-search-integration | full | 1024 | path `internal/search/search.go` | packing_dropped | fts:6,40-44,46,47,50,51,53,56-58,60,62,66,71,83 | 10 | - |
| jsts-search-integration | full | 1024 | symbol `internal/search/search.go::Search` | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | full | 1024 | anchor `internal/search/search.go::Search` callers | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | full | 2048 | path `internal/search/search.go` | packing_dropped | fts:6,40-44,46,47,50,51,53,56-58,60,62,66,71,83 | 10 | - |
| jsts-search-integration | full | 2048 | symbol `internal/search/search.go::Search` | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | full | 2048 | anchor `internal/search/search.go::Search` callers | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | full | 4096 | path `internal/search/search.go` | packing_dropped | fts:6,40-44,46,47,50,51,53,56-58,60,62,66,71,83 | 10 | - |
| jsts-search-integration | full | 4096 | symbol `internal/search/search.go::Search` | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | full | 4096 | anchor `internal/search/search.go::Search` callers | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | no-rel | 2048 | path `internal/search/search.go` | packing_dropped | fts:6,40-44,46,47,50,51,53,56-58,60,62,66,71,83 | 10 | - |
| jsts-search-integration | no-rel | 2048 | symbol `internal/search/search.go::Search` | packing_dropped | fts:6 | 10 | - |
| jsts-search-integration | no-rel | 2048 | anchor `internal/search/search.go::Search` callers | packing_dropped | fts:6 | 10 | - |
| mcp-evidence-output | full | 1024 | path `internal/mcpserver/server.go` | packing_dropped | fts:11,81 | 12 | - |
| mcp-evidence-output | full | 1024 | symbol `internal/mcpserver/server.go::codeContext` | retrieval_missing | - | - | - |
| mcp-evidence-output | full | 1024 | anchor `internal/mcpserver/server.go::codeContext` references | retrieval_missing | - | - | - |
| mcp-evidence-output | full | 2048 | path `internal/mcpserver/server.go` | packing_dropped | fts:11,81 | 12 | - |
| mcp-evidence-output | full | 2048 | symbol `internal/mcpserver/server.go::codeContext` | retrieval_missing | - | - | - |
| mcp-evidence-output | full | 2048 | anchor `internal/mcpserver/server.go::codeContext` references | retrieval_missing | - | - | - |
| mcp-evidence-output | full | 4096 | path `internal/mcpserver/server.go` | packing_dropped | fts:11,81 | 12 | - |
| mcp-evidence-output | full | 4096 | symbol `internal/mcpserver/server.go::codeContext` | retrieval_missing | - | - | - |
| mcp-evidence-output | full | 4096 | anchor `internal/mcpserver/server.go::codeContext` references | retrieval_missing | - | - | - |
| mcp-evidence-output | no-rel | 2048 | path `internal/mcpserver/server.go` | packing_dropped | fts:11,81 | 12 | - |
| mcp-evidence-output | no-rel | 2048 | symbol `internal/mcpserver/server.go::codeContext` | retrieval_missing | - | - | - |
| mcp-evidence-output | no-rel | 2048 | anchor `internal/mcpserver/server.go::codeContext` references | retrieval_missing | - | - | - |

The fresh selected rows are byte-semantically identical to the same 44 rows in
`docs/benchmarks/attribution-v0.6.json` (`Compare-Object` differences: 0).
The frozen PHP and MCP symbol/anchor subset has exactly 16
`retrieval_missing` rows. At budget 2048, project metadata `Run` is retrieved
by FTS at raw position 50, ranked position 20, and dropped by packing; JSTS
`Search` is retrieved by FTS at position 6, ranked position 10, and dropped by
packing; MCP `codeContext` and PHP `Run` remain retrieval-missing.

Attribution JSON parsed successfully. The JSON and Markdown contained no
source/content fields, absolute Windows or Unix workspace paths, username,
environment value names, secret sentinel, NaN, or Infinity. Their temporary
quality/Markdown/attribution Git blob-equivalent hashes were, respectively:

- `132c76f1a221773c045da6529e35678f7dbb5419`
- `b4786d00a53aed1023b294ee2fafb1b76d821e62`
- `9aca8bc755a73efb487b8e86401e3ec529b5f2a1`
- `354cc2f938b26142a8e198ef585ad5cd375c2d5d`

All four temporary files were removed after hashing and scanning.

- `2026-09-01T00:16:29Z` — Frozen candidate pre-run state: candidate commit
  `605ad7d`; baseline commit `e9d6ec5`. The exact
  `git diff --name-status e9d6ec5..605ad7d` contains 14 modified paths:
  `PLAN.md`, `internal/benchmark/{attribution.go,attribution_test.go}`,
  `internal/eval/eval_test.go`, `internal/mcpserver/mcp_test.go`,
  `internal/search/{fusion.go,fusion_test.go,retrieval.go,retrieval_test.go,search.go,search_test.go,trace.go}`,
  and `internal/store/{store.go,store_test.go}`. The diff stat is 1,369
  insertions and 82 deletions. The new retriever weight is frozen at `1.35`;
  path hints, file scopes, symbol hints, per-file candidates, and total scoped
  candidates are capped at 8, 8, 16, 8, and 40. Task 6 passed 2 focused
  attribution/privacy tests, 221 contract regression tests in 5 packages, and
  691 full-suite tests in 46 packages; `go vet ./...` and
  `git diff --check` passed. No candidate benchmark had run at this point.

- `2026-09-01` — The candidate command ran exactly once with repeat 1 and zero
  retries. It produced 4 cases, 24 quality results, and a valid 20-result / 55
  label attribution report. The selected non-FTS 44 rows remained exactly 20
  `retrieval_missing`, 20 `packing_dropped`, and 4 `packed`. Comparison with
  v0.5 returned `compatible: true` and `regressions: 0`.

### Candidate gate: hard invariants

| Invariant | Measured result | Verdict |
| --- | --- | --- |
| v0.5 compatibility / regressions | `true` / `0` | PASS |
| Budget compliance | `1.0` in every quality row | PASS |
| Deterministic output | `1.0` in every quality row | PASS |
| Relation validity | `1.0` in every quality row | PASS |
| Forbidden violations | `0` in every quality row | PASS |
| Known-handle resend | Contract test passed; candidate executed no real expansion, so no candidate resend measurement exists | NOT EXERCISED |
| Earlier terminal-stage movement | `0` selected rows | PASS |
| `label_not_indexed` | `0` selected rows | PASS |
| FTS-only behavior | Focused call/output guards passed; candidate FTS rows remained separate | PASS |
| Japanese relation recall | auth full `1.0`; JSTS full `1.0` | PASS |
| Privacy / finite values | Both JSON files parsed; forbidden scan matches `0` | PASS |

### Candidate gate: symbol identity

| Frozen condition | Baseline stage / position | Candidate stage / position and hits | Packet | Expansion | Verdict |
| --- | --- | --- | --- | --- | --- |
| At least 8 of 16 PHP/MCP missing symbol/anchor rows advance | 16 `retrieval_missing` | 16 `retrieval_missing`; advanced `0` | 0 | 0 | FAIL |
| Both deficient cases advance at least one row | PHP 0; MCP 0 | PHP 0; MCP 0 | 0 | 0 | FAIL |
| MCP `codeContext`, full/2048 | `retrieval_missing` | `retrieval_missing`; no hits | No | No | FAIL |
| Project metadata `Run`, full/2048 | `packing_dropped`, rank 20, FTS 50 | `packing_dropped`, rank 28, FTS 50; no scoped hit | No | n/a | FAIL |
| PHP `Run`, full/2048 | `retrieval_missing` | `retrieval_missing`; no hits | No | No | FAIL |
| Previously blocked expansion executes | blocked | all selected anchors remain unpacked | No anchor packet | 0 successful | FAIL |
| 2048 required-symbol recall improves in at least two cases | 0 improved cases | 0 improved cases | No new symbol packet | 0 | FAIL |

JSTS `Search` did receive `path-scoped-symbol:1` in addition to `fts:6`, but
remained `packing_dropped` at ranked position 10 for every selected profile and
budget. This did not satisfy any frozen symbol-identity condition.

### Candidate gate: boundedness and residue

| Boundary | Evidence | Verdict |
| --- | --- | --- |
| At most 40 scoped candidates | fixed cap and focused boundary tests | PASS |
| At most 8 candidates per path | fixed cap and fairness tests | PASS |
| At most 8 scoped paths | fixed cap and scope-order tests | PASS |
| Symbol-owned candidates only | store query guard and tests require non-empty symbol handle | PASS |
| File probe is not a retriever/generic chunk list | retriever call/list tests | PASS |
| Deterministic normal search | deterministic unit guard and report value `1.0` | PASS |
| No corpus-specific production string | production-only search matches `0`; named values occur only in tests | PASS |

The four candidate files remained under `.focalspan-bench` and had Git
blob-equivalent hashes `3fe4de690c3050e0e391798687c129d5f0ed3587`,
`cf82d1b00523c7b050b9abdc6d3628c623dbffd4`,
`51dc68ce0485e4dd6452e7fce15b8da3976c7561`, and
`fb1830010a337d331e9c29191d2425ac15f82674`. No temporary workspace, index,
binary, or report appeared outside `.focalspan-bench`; the two pre-existing
untracked files remained `.focalspan.json` and `PLAN_v0.7.md`.

**Frozen decision: FAIL.** The production hypothesis is rejected because every
symbol-identity improvement condition failed. No weight, limit, SQL order,
hint, ranking, or packing adjustment will be attempted. Task 8 is skipped and
Task 9 reverts the v0.7 production/test commits.

### Negative closure

The failed conditions separate as follows:

- file/scope discovery did not expose PHP `Run` or MCP `codeContext` at all;
- the scoped symbol list was absent for those identities and for project
  metadata `Run`;
- JSTS `Search` did enter the scoped list at raw position 1 but remained ranked
  10 and unpacked; project metadata `Run` worsened from rank 20 to 28 and
  remained unpacked;
- no required symbol/anchor packet existed, so the failure occurred before a
  real expansion rather than after an expansion request;
- the v0.5 quality comparison had zero regressions, Japanese full relation
  recall stayed `1.0`, and privacy/determinism checks passed.

The five production/test commits were reverted in reverse order:

- `5f3ce64` reverted Task 6 `a082ef4`;
- `fa77e1e` reverted Task 5 `ebd1670`;
- `5249d27` reverted Task 4 `a7f005f`;
- `d3b6e64` reverted Task 3 `e095634`;
- `0dd456d` reverted Task 2 `70d7b51`.

After the reverts, `internal/` and `cmd/` had no diff from baseline commit
`e9d6ec5`. Closure verification passed 666 tests in 46 packages, `go vet ./...`
reported no issues, and `git diff --check` passed. The selected four-case
repeat-1 closure smoke ran exactly once, produced 4 cases and 24 quality
results, and compared with v0.5 as compatible with zero regressions. Its 44
selected attribution rows had zero semantic differences from
`docs/benchmarks/attribution-v0.6.json`: 20 `retrieval_missing`, 20
`packing_dropped`, and 4 `packed`.

The closure report hashes were `d2a285912f6e7efb65f0be8447cceaf11f9bb74c`,
`fe5a7c86941e5079f77b212849a1273255730d34`,
`9aca8bc755a73efb487b8e86401e3ec529b5f2a1`, and
`354cc2f938b26142a8e198ef585ad5cd375c2d5d`. No v0.7 quality baseline is
created. The next measured failure category is the split between query-to-file
scope coverage and post-retrieval packet selection; it requires a separately
planned milestone rather than a second v0.7 adjustment.

Post-closure GitHub Actions run
[`33456207885`](https://github.com/AoiKagase/FocalSpan/actions/runs/33456207885)
completed successfully at commit `52d82e613b885e00b7636282255b9780f8800051`.
The actual test/vet, Linux race, Windows amd64, Linux amd64, Darwin arm64, and
public benchmark smoke jobs all succeeded. The manual full benchmark job was
skipped by design, so no eight-case repeat-3 v0.7 evaluation was executed.
The eight temporary candidate/closure reports were removed during final
verification; `.focalspan.json` and `PLAN_v0.7.md` remain preserved untracked.
