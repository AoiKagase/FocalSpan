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
