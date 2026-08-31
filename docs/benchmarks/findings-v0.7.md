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
