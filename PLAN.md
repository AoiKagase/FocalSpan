# FocalSpan Failure-Layer Attribution v0.8 Implementation Plan

> **For agentic workers:** Execute this plan task-by-task. Every behavior
> change follows RED, explicit RED confirmation, minimal GREEN, focused
> verification, `go test ./... -count=1`, `go vet ./...`, `git diff --check`,
> and explicit-path commits. Keep Progress, discoveries, decisions, and
> outcomes current while working.

**Goal:** Add a development-only diagnosis report that separates missing
repository evidence into path-scope, symbol-match, ranking, and packing
failure layers without changing production retrieval or model-visible output.

**Architecture:** Reuse the existing source-free search trace and v1
attribution result. A separate diagnosis compiler summarizes whether the
expected path appeared in any retriever before classifying the exact expected
identity. Benchmark CLI output is opt-in and never reaches normal CLI, MCP, or
`focalspan.context.v1` serialization.

**Tech Stack:** Go 1.27; standard library; existing
`internal/{benchmark,benchcli,app,search,evidence}` packages; current historical
benchmark harness and Git snapshot runner.

**Spec:** The user-approved “FocalSpan Failure-Layer Attribution v0.8” plan.
This root file is the sole active ExecPlan under `PLANS.md`.

**Plan ID:** `v0.8-failure-layer-attribution`

**Status:** Active.

**Starting commit:** `77f42cf5532ae1d7655e9956e8288e9203c93f93`.

---

## Global Constraints

- Treat the current checkout as the only source of truth. Do not use
  `git reset`, `git restore`, `git checkout --`, `git clean`, or `git stash`.
- Preserve untracked `.focalspan.json` and `TASKS.md`. The pre-existing
  `PLAN_v0.7.md` disappeared during initial inspection and must not be
  recreated or deleted by this milestone.
- Keep `focalspan.benchmark-attribution.v1` byte-semantically unchanged.
- Add `focalspan.benchmark-diagnosis.v1` as a separate development contract.
- Do not change query normalization, retrieval, RRF, ranking, linking,
  packing, token accounting, MCP tools, server instructions, or
  `focalspan.context.v1`.
- Do not expose diagnosis, traces, scores, token-saving debug fields, query
  source, candidate source, unmatched symbols, absolute paths, usernames,
  environment values, or secrets through normal CLI/MCP output.
- Do not add network access, external LLM calls, embeddings, semantic
  providers, model-specific tokenizers, rerankers, repository-code execution,
  package restore, or new runtime dependencies.
- Every production behavior begins with a failing deterministic test.
- The eight-case repeat-1 diagnosis run executes at most once after static
  verification passes. An infrastructure failure before valid output may be
  retried once only after recording the reason.
- No `results-v0.8` quality baseline is created.

---

## Purpose / Big Picture

v0.6 classified 95 historical labels as 55 `retrieval_missing`, 35
`packing_dropped`, and 5 `packed`. v0.7 tested bounded path-scoped symbol
retrieval, but it advanced none of the required PHP/MCP identities and was
reverted. The remaining `retrieval_missing` category conflates two materially
different failures:

1. no retriever returned any candidate from the expected path;
2. a retriever reached the expected path but never returned the exact expected
   symbol identity.

v0.8 measures that boundary only. It does not tune a production subsystem.
The result selects exactly one later milestone using a frozen count rule.

---

## Interfaces

Keep the v1 attribution interfaces unchanged. Add a separate diagnosis model:

```go
const DiagnosisSchemaV1 = "focalspan.benchmark-diagnosis.v1"

const (
    DiagnosisLabelNotIndexed    = "label_not_indexed"
    DiagnosisPacked             = "packed"
    DiagnosisPackingDropped     = "packing_dropped"
    DiagnosisLinkingUnresolved  = "linking_unresolved"
    DiagnosisRankingDropped     = "ranking_dropped"
    DiagnosisSymbolMatchMissing = "symbol_match_missing"
    DiagnosisPathScopeMissing   = "path_scope_missing"
)

type DiagnosisPathHit struct {
    Retriever     string `json:"retriever"`
    FirstPosition int    `json:"first_position"`
    Count         int    `json:"count"`
}

type DiagnosisLabel struct {
    Expectation      string             `json:"expectation"`
    Path             string             `json:"path"`
    Symbol           string             `json:"symbol,omitempty"`
    Kind             string             `json:"kind,omitempty"`
    Relation         string             `json:"relation,omitempty"`
    AttributionStage string             `json:"attribution_stage"`
    DiagnosticStage  string             `json:"diagnostic_stage"`
    ReasonCode       string             `json:"reason_code"`
    PathHits         []DiagnosisPathHit `json:"path_hits,omitempty"`
    RetrieverHits    []AttributionHit    `json:"retriever_hits,omitempty"`
    RankedPosition   int                `json:"ranked_position,omitempty"`
    PackedPosition   int                `json:"packed_position,omitempty"`
}

type DiagnosisResult struct {
    Schema       string           `json:"schema"`
    CaseID       string           `json:"case_id"`
    RepositoryID string           `json:"repository_id"`
    Profile      string           `json:"profile"`
    Budget       int              `json:"budget"`
    Labels       []DiagnosisLabel `json:"labels"`
}
```

Add paired benchmark CLI flags:

```text
--diagnosis-json-out PATH
--diagnosis-markdown-out PATH
```

Either both are present or neither is present. Diagnosis and attribution may
be requested together; the runner performs one traced query per repeat.

---

## Classification Rules

`AttributionStage` is copied from the unchanged v1 attribution label.
`DiagnosticStage` uses this fixed priority:

1. `label_not_indexed` / `label_not_indexed` when the v1 stage is not indexed.
2. `packed` / `selected_in_packet` when packed position is positive.
3. `packing_dropped` / `omitted_by_packer` when ranked position is positive.
4. `linking_unresolved` / `relation_unresolved` when v1 reports unresolved.
5. `ranking_dropped` / `removed_before_rank` when exact retriever hits exist.
6. `symbol_match_missing` / `expected_path_retrieved_identity_missing` when
   the v1 stage is retrieval-missing but at least one raw retrieved candidate
   has exactly the expected repository-relative path.
7. `path_scope_missing` / `expected_path_not_retrieved` otherwise.

For a required-path label, every raw candidate on the expected path is already
an exact v1 identity match, so `symbol_match_missing` is not reachable.

Path-hit summaries are grouped by valid retriever in first-observed order.
Each group records the first raw retriever position and total candidates from
the expected path. They never copy identities from unmatched candidates.

---

## Plan of Work

### Task 0: Archive v0.7 and Start v0.8

- [x] Record starting branch, HEAD, status, task tracker, and baseline tests.
- [x] Copy the completed v0.7 root plan byte-for-byte to
  `docs/superpowers/plans/completed/2026-09-01-v0.7-path-scoped-symbol-retrieval.md`.
- [x] Verify source and archive SHA-256 are both
  `46F2B415AD08CFB6FC5B750723D15387111E87EE8966A2A985D288A55478F8A0`.
- [x] Replace root `PLAN.md` with this v0.8 plan and add the archive index link.
- [x] Run `git diff --check`, verify only intended plan files are staged, and
  commit `docs: start failure-layer attribution v0.8`.

### Task 1: Add the Diagnosis Classifier

- [x] Add table-driven RED tests for all seven diagnostic stages and the fixed
  priority order.
- [x] Add RED tests proving same-path/different-symbol becomes
  `symbol_match_missing`, no same-path candidate becomes `path_scope_missing`,
  and required-path labels cannot become `symbol_match_missing`.
- [x] Add RED tests for stable path-hit grouping, first position, count, and
  omission of unmatched symbols/source.
- [x] Confirm RED because diagnosis interfaces do not exist.
- [x] Implement `internal/benchmark/diagnosis.go` using only
  `AttributionInput` and unchanged v1 attribution labels.
- [x] Add deterministic JSON/Markdown renderers and strict validation for
  schema, stage/reason pairs, positions, counts, path safety, retrievers, and
  control characters.
- [x] Run focused benchmark tests and full `internal/benchmark` tests.
- [x] Commit `feat: classify benchmark failure layers`.

### Task 2: Integrate Runner and CLI Output

- [x] Add runner RED tests requiring diagnosis-only and combined
  attribution+diagnosis runs to use the traced query route once per repeat.
- [x] Add CLI RED tests for paired flags, one-sided flag rejection,
  deterministic source-free files, and unchanged attribution-only behavior.
- [x] Confirm RED before changing runner or CLI code.
- [x] Extend `RunRequest` and `RunReport` with development-only diagnosis
  state excluded from quality JSON.
- [x] Reuse a single `AttributionInput` and v1 label result to compile both
  reports when both outputs are requested.
- [x] Write diagnosis outputs only after quality and optional attribution
  outputs have validated successfully.
- [x] Add MCP and Evidence regression assertions showing trace off/on leaves
  normal structured output byte-identical and diagnosis identifiers absent.
- [x] Run `go test ./internal/benchmark ./internal/benchcli ./internal/app
  ./internal/evidence ./internal/mcpserver -count=1`.
- [x] Commit `feat: emit opt-in benchmark diagnosis`.

### Task 3: Run the Frozen Historical Measurement

- [x] Run all static tests, vet, and diff check before the benchmark.
- [x] Validate all eight cases in `testdata/benchmark/focalspan-history.json`.
- [x] Run the eight-case repeat-1 benchmark exactly once with quality,
  Markdown, v1 attribution, diagnosis JSON, and diagnosis Markdown outputs.
- [x] Compare quality with `docs/benchmarks/results-v0.5.json`; require
  compatibility and zero regressions.
- [x] Require exactly 95 v1 labels and exact v0.6 stage compatibility:
  55 retrieval-missing, 35 packing-dropped, 5 packed, and zero other stages.
- [x] Require every v1 retrieval-missing row to map to exactly one of
  `path_scope_missing` or `symbol_match_missing`.
- [x] Scan outputs for source/content fields, absolute paths, username,
  environment names/values, secret sentinels, NaN, and Infinity.
- [x] For `full-evidence-focused` budget 2048, count the four unmet layers and
  select the maximum; ties use this upstream order:
  `path_scope_missing`, `symbol_match_missing`, `ranking_dropped`,
  `packing_dropped`.
- [x] Record commands, hashes, counts, selected next layer, and limitations in
  `docs/benchmarks/findings-v0.8.md`, `docs/evaluation.md`, and this plan.
- [x] Remove temporary benchmark outputs after recording verified hashes.
- [x] Commit `docs: record failure-layer diagnosis v0.8`.

### Task 4: Final Verification and Closure

- [x] Run formatting only on changed Go files.
- [x] Run `git diff --check`, `go test ./... -count=1`, and `go vet ./...`.
- [x] Run CGO-free native focalspan/focalspan-bench builds and Windows amd64,
  Linux amd64, and Darwin arm64 focalspan cross-builds to an ignored temporary
  directory, then remove it.
- [x] Run existing fixture/Evidence contract tests and focused normal-output
  privacy tests.
- [x] Verify `focalspan.benchmark-attribution.v1`, production search/rank/pack,
  MCP tool definitions, and `focalspan.context.v1` remain unchanged.
- [x] Verify status contains no generated reports, indexes, binaries, or
  temporary workspaces and preserves user-owned untracked files.
- [ ] Push only after local verification, inspect the actual CI run, and record
  test, vet, Linux race, three cross-builds, and smoke conclusions. Do not call
  skipped jobs successful.
- [ ] Complete Progress, discoveries, decisions, and outcomes; commit final
  documentation if CI evidence changes tracked files.

---

## Frozen Acceptance Criteria

- Existing v1 attribution JSON/Markdown remains byte-semantic compatible.
- Diagnosis classification covers every label exactly once and is deterministic.
- All 55 historical retrieval misses split into the two new upstream layers.
- Diagnosis contains no unmatched identity or source material.
- Normal CLI, MCP, Evidence, and quality report output remains unchanged.
- v0.5 comparison reports compatible with zero regressions.
- Budget, fidelity, relation validity, known-handle suppression, determinism,
  and forbidden-path invariants remain passing.
- One and only one next primary layer is chosen by the frozen count rule.
- No production optimization or `results-v0.8` baseline is introduced.

---

## Progress

- [x] `2026-09-01` Starting state recorded at `77f42cf` on `master`; baseline
  `go test ./... -count=1` passed 666 tests in 46 packages.
- [x] `2026-09-01` v0.7 archived byte-for-byte with matching SHA-256.
- [x] `2026-09-01` Diagnosis classifier implemented via explicit RED and
  verified by 12 focused tests and 59 `internal/benchmark` tests.
- [x] `2026-09-01` Runner/CLI diagnosis output implemented via explicit RED;
  the specified five-package regression suite passed 254 tests.
- [x] `2026-09-01` Eight-case/default/repeat-1 measurement completed once at
  `8518fe8` with zero retries; 48 quality results were v0.5-compatible with
  zero regressions.
- [x] `2026-09-01` The 55 v1 retrieval misses split into 45 path-scope and 10
  symbol-match misses. Focused 2048 counts were 9/2/0/7, selecting
  `path_scope_missing` as the next primary layer.
- [x] `2026-09-01` Local closure passed 683 tests in 46 packages, 16 focused
  diagnosis/attribution/privacy tests, 205 contract tests in 5 packages, vet,
  diff check, two native builds, and three cross-builds; temporary outputs were
  removed and user-owned inputs preserved.
- [ ] Actual remote CI inspected.

---

## Surprises & Discoveries

- **2026-09-01:** The untracked `TASKS.md` describes an independent future
  relation-linking/schema-v2 task and explicitly excludes active `PLAN.md`
  changes. It remains untouched.
- **2026-09-01:** The initially observed untracked `PLAN_v0.7.md` disappeared
  before plan transition. FocalSpan did not delete or recreate it.
- **2026-09-01:** The first report-analysis pass treated top-level JSON arrays
  as one PowerShell object and could not use `Get-FileHash` in the RTK shell.
  The valid benchmark was not rerun; corrected array enumeration and .NET
  SHA-256 produced the recorded counts and hashes from the same six outputs.

---

## Decision Log

- **Decision:** Run v0.8 in the current master checkout.
  **Rationale:** The approved plan explicitly makes the current checkout the
  only source of truth, and the user directly requested implementation here.
  **Date/Author:** 2026-09-01 / Codex.
- **Decision:** Add a separate diagnosis schema rather than revise attribution
  v1. **Rationale:** Historical attribution is accepted evidence; diagnosis is
  additive development data with a narrower purpose.
  **Date/Author:** 2026-09-01 / project plan.
- **Decision:** Infer path-scope coverage only from raw retrieved paths already
  present in the source-free trace. **Rationale:** This separates the measured
  failure without adding probes or changing production retrieval.
  **Date/Author:** 2026-09-01 / project plan.
- **Decision:** Select `path_scope_missing` as the next primary layer.
  **Rationale:** At full Evidence focused budget 2048, the frozen counts were 9
  path-scope missing, 2 symbol-match missing, 0 ranking dropped, and 7 packing
  dropped; 9 is the unique maximum.
  **Date/Author:** 2026-09-01 / Codex.

---

## Outcomes & Retrospective

Implementation and the frozen local measurement are complete. The diagnosis
split 55 retrieval misses into 45 path-scope and 10 symbol-match misses; the
focused 2048 rule selected `path_scope_missing` from counts 9/2/0/7. Quality
remained v0.5-compatible with zero regressions, privacy/finite scans were clean,
and local verification passed. This milestone diagnoses a boundary only; it
does not claim token reduction or retrieval improvement. Remote CI remains to
be pushed and inspected before closure is complete.

---

## Idempotence and Recovery

- Classification and rendering are pure and deterministic.
- Benchmark snapshots remain temporary and source repositories are read-only.
- Existing output files require `--force`; paired diagnosis flags fail before
  benchmark execution when incomplete.
- If the valid repeat-1 report is produced, do not rerun it to change the
  measured selection.
- On infrastructure failure, preserve the error in the Decision Log before the
  one permitted retry.
- Do not modify or remove user-owned untracked files during cleanup.

---

## Dependencies

No new third-party or runtime dependency is permitted. `internal/benchmark`
owns diagnosis types, classification, validation, and rendering;
`internal/benchcli` owns flags and files; `internal/app` and `internal/search`
remain read-only providers of the existing trace.
