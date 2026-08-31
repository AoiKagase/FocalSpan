# FocalSpan Candidate Attribution and Coverage v0.6 Implementation Plan

> **For agentic workers:** Execute this plan task-by-task. Each behavior change
> uses RED, expected RED confirmation, minimal GREEN, focused tests,
> `go test ./... -count=1`, `git diff --check`, and an explicit-path commit.

**Goal:** Add a source-free development-only pre-packet attribution trace,
measure exactly where required evidence is lost, select one retriever or linker
candidate-coverage defect from that evidence, and prove one bounded production
improvement without changing normal CLI/MCP output or the Evidence Packet wire
contract.

**Architecture:** `internal/search` will expose additional content-free stage
facts only when its existing trace flag is requested. A benchmark-only adapter
in `internal/app` and `internal/benchmark` will join those facts to the existing
human labels, classify retrieval, linking, ranking, and packing outcomes, and
write a separate sanitized attribution section. Normal `focalspan` CLI/MCP
paths continue to call the non-trace methods. After a frozen diagnostic run,
the Decision Log will select exactly one retriever or linker change and freeze
its cases, labels, and thresholds before production code changes.

**Tech Stack:** Go 1.27 as currently verified; standard library; existing
`internal/{app,benchmark,benchcli,evidence,linker,query,rank,search,store}`;
SQLite/FTS5 through the existing store; local Git CLI with separated arguments.

**Spec:** This root plan is the sole active specification and ExecPlan. It is
governed by `PLANS.md` and preserves the architecture in `docs/design.md`, the
v0.5 measurements in `docs/evaluation.md`, and the frozen baseline in
`docs/benchmarks/results-v0.5.json`.

**Plan ID:** `v0.6-candidate-attribution-and-coverage`

**Status:** Active.

**Baseline:** `56d0f84` is the v0.5 completion-evidence commit created after
GitHub Actions run `33361467769` passed at product commit `ca54f11`. The v0.5
plan is archived byte-for-byte as
`docs/superpowers/plans/completed/2026-08-31-v0.5-real-repository-evaluation.md`.

## Global Constraints

- Treat the current checkout as the only source of truth. At Task 0 start,
  `master` and `origin/master` both pointed to `ca54f11`; the worktree had no
  tracked change and contained only the pre-existing untracked
  `.focalspan.json`.
- Never read, modify, stage, commit, delete, move, or overwrite the starting
  `.focalspan.json`.
- Do not use `git reset`, `git restore`, `git checkout --`, `git clean`,
  `git stash`, or `git add .`. Stage only the paths named by the current task.
- Do not add source text, source segments, secrets, usernames, environment
  values, or absolute paths to attribution data, reports, logs, errors, tests,
  or checked-in artifacts.
- Attribution may contain only logical repository ID, case/profile/budget,
  repository-relative path, symbol name/kind, expectation kind, retriever or
  relation stage, sanitized reason code, and one-based position/rank.
- Keep attribution inside `internal/benchmark`, `internal/benchcli`, and the
  minimum content-free adapter in `internal/search`/`internal/app`. Product
  packages must not import benchmark packages.
- Do not add ranking, candidate, attribution, or token-saving fields to normal
  CLI output, MCP text, MCP structured content, or `focalspan.context.v1`.
- Do not change public `focalspan` commands, the five MCP tool names, Evidence
  Packet fields, SQLite schema, or source-fidelity behavior.
- Never execute benchmarked repository code, tests, generators, hooks, package
  managers, or build tools. Add no network, remote clone, external LLM,
  embedding, vector search, or persistent cross-run cache.
- Target diffs remain diagnostics. They are never automatic required labels.
- Do not hard-code public corpus commit IDs, case IDs, queries, paths, or
  symbols in production code.
- After attribution, implement exactly one retriever or linker improvement.
  Do not simultaneously change parser behavior, ranking weights, relation
  semantics outside the selected linker defect, packing, Evidence compilation,
  labels, thresholds, or budgets.
- Evidence Packet metadata compaction remains out of scope until candidate
  coverage and at least one expansion anchor measurably improve.
- Preserve one index build per historical case and share it across profiles and
  budgets. Do not reintroduce profile-level index rebuilds.
- During development run only the two selected smoke cases with repeat 1. Run
  one eight-case repeat-1 diagnostic after the trace is stable. Run the
  eight-case repeat-3 candidate comparison once, at the final candidate gate.
  Record the reason and count before any retry.

---

## Purpose / Big Picture

The v0.5 public history suite found required-symbol recall 0, required-path
mean recall 0.125, and nine missing expansion anchors. Raising the Evidence
budget from 1024 to 4096 recovered nothing. The existing report proves final
packet absence but cannot say whether an expected label never entered a raw
retriever list, appeared only through unresolved linking, entered retrieval but
fell below the ranked candidate limit, or was ranked and then omitted by the
Evidence compiler.

v0.6 first makes those boundaries observable without source content. Every
required path, required symbol, and expansion anchor receives one terminal
classification:

    retrieval_missing
    linking_unresolved
    ranking_dropped
    packing_dropped
    packed

`linking_unresolved` is used only when the trace contains a matching relation
candidate with lexical/unresolved provenance, or when a planned relation has a
retrieved exact anchor but no matching relation candidate. It is never guessed
for an ordinary lexical miss. `ranking_dropped` requires a raw retriever hit and
no final ranked position. `packing_dropped` requires a final ranked position and
no Evidence item. `packed` requires an exact packet match. If the expected
symbol is not indexed, the case is invalid for production selection and is
recorded as `label_not_indexed`, not reassigned to a convenient stage.

The diagnostic result selects one defect family. The plan then freezes the
affected labels and an acceptance threshold before the first production test.
The goal is not a broad relevance retune; it is one demonstrated movement of
frozen evidence from the chosen missing stage toward the packet while all
legacy, wire, privacy, and deterministic contracts remain intact.

---

## Progress

- [x] Task 0 starting checkout, required documents, remote state, and latest
  GitHub Actions evidence inspected. (2026-08-31T07:23:33Z; run `33361467769`.)
- [x] v0.5 completion evidence committed as `56d0f84`, then its final root plan
  copied byte-for-byte for archive transition.
- [x] Task 0 plan archive/index/new active-plan transition committed.
  (2026-08-31T07:26:00Z; commit `6582dc5`.)
- [x] Task 1 current baseline, fixtures, v0.5 report, and suite frozen.
  (2026-08-31T07:38:26Z; 657 tests/46 packages, vet, 8-case validation,
  and two-case repeat-1 comparison passed.)
- [x] Task 2 source-free attribution schema and classifiers implemented.
  (2026-08-31; focused 7 tests, package 46 tests, full 664 tests/46 packages,
  and diff check passed.)
- [x] Task 3 internal search/app/benchmark trace adapter implemented without
  normal-output changes. (2026-08-31; 105 focused package tests, 157 dedicated
  CLI/MCP/Evidence tests, and 664 full tests/46 packages passed; diff check
  passed.)
- [x] Task 4 two-case smoke and one eight-case repeat-1 attribution diagnostic
  completed. (2026-08-31; diagnostic 1 run, retry 0; 95 labels classified.)
- [x] Task 5 exactly one improvement target and acceptance contract frozen in
  the Decision Log before production code changes. (2026-08-31; docs-only.)
- [x] Task 6 selected retriever improvement implemented with TDD. (2026-08-31;
  focused 2, search 21, regression eval 1, full 667 tests/46 packages passed.)
- [x] Task 7 bounded verification completed; the frozen expansion-anchor gate
  failed, so the candidate was rejected and work stopped. (2026-08-31.)
- [ ] Task 8 one final eight-case repeat-3 candidate comparison completed.
- [ ] Task 9 final local/remote verification, documentation, and retrospective
  completed.

---

## Surprises & Discoveries

- 2026-08-31: The latest remote run was newer than the v0.5 documents. Run
  `33361467769` at `ca54f11` passed Linux test/vet/race, all three CGO-free
  builds, and the two-case repeat-1 smoke/compare. The manual full job was
  skipped as designed.
- 2026-08-31: `internal/search.SearchDetailed` already has an opt-in trace, but
  it currently retains only candidates that survive ranking/limit. It does not
  retain content-free identities from every raw retriever list, and
  `internal/app` neither requests nor forwards the trace.
- 2026-08-31: `internal/evidence.CompileResult` reports aggregate selected and
  omitted counts but not omitted identities. Packing attribution can therefore
  be derived safely by exact identity comparison between ranked candidate trace
  and the final Packet; the Evidence wire schema need not change.
- 2026-08-31: Link resolution provenance already exists on relation candidates
  as resolved versus lexical `RelationContext`. The trace can sanitize this to
  reason codes without exposing source content or linker internals.
- 2026-08-31: The v0.6 starting checkout at `6582dc5` is two documentation
  commits ahead of `origin/master`. Fresh tests passed 657 tests in 46 packages;
  vet and diff check passed; all eight historical labels validated; and the
  bounded two-case repeat-1 run produced 12 quality results with compatible
  true and zero regressions. No full quality suite was run.
- 2026-08-31: The attribution schema needs no floating-point values: positions,
  ranks, and budgets are integers. JSON therefore cannot encode NaN or Infinity,
  while output validation rejects absolute/non-normalized paths, control
  characters, unknown retrievers/relation states, and unpaired stage/reason
  codes before serialization.
- 2026-08-31: Raw retriever lists can contain the same identity once per
  retriever. The trace intentionally preserves every list occurrence in
  retriever execution order with a one-based position; later attribution keeps
  all matching hits rather than prematurely deduplicating stage evidence.
- 2026-08-31: The eight-case diagnostic produced no not-indexed, linker, or
  ranking terminal rows. Across 40 Evidence-profile results, 55 of 95 labels
  were retrieval misses, 35 were ranked but omitted by packing, and 5 required
  paths were packed. No required symbol or expansion anchor was packed.
- 2026-08-31: The first full test after adding lexical path hints exposed a
  Japanese JSTS relation-recall regression from 1.0 to 0.6667. Broad path hits
  entered the eight-item relation-anchor pool before FTS candidates. Restricting
  lexical path hints to plans with no relations preserves the selected PHP
  hypothesis and the old relation behavior; explicit path hints remain enabled
  for every plan.
- 2026-08-31: The bounded candidate moved the four selected required-path rows
  from retrieval missing to packing dropped, but all four `Run` symbols and all
  four expansion anchors remained retrieval missing. Selected misses improved
  12 to 8, yet packed anchors remained 0; the frozen gate failed.

---

## Decision Log

- 2026-08-31: Use a separate benchmark attribution schema rather than adding
  debug metadata to `focalspan.context.v1`. This preserves the public wire
  contract and makes privacy/invariant tests local to the development tool.
- 2026-08-31: Extend the existing opt-in search trace instead of adding a
  second retrieval implementation. The searcher is the only point that sees raw
  retriever lists, fused candidates, and ranked candidates together.
- 2026-08-31: Derive packing outcome by exact path/symbol/kind matching between
  ranked trace and Packet. Do not expose Evidence compiler internal source
  variants or utilities.
- 2026-08-31: Preserve expectation and trace slice order in attribution output;
  do not sort through maps. A required path matches any identity at that exact
  path, while a symbol or expansion anchor matches exact path/name and, when
  supplied, exact kind. Classify `linking_unresolved` only when all matching raw
  hits are unresolved relation hits; an ordinary or resolved hit makes an
  unranked label `ranking_dropped`.
- 2026-08-31: `QueryEvidence` and `QueryEvidenceAttributed` share one private
  validation, retrieval, and compile path. The only switch is the internal
  search `Trace` flag; benchmark code receives the compile result and trace,
  while ordinary callers continue to receive the original `CompileResult`.
- 2026-08-31: Indexed-label checks belong in the benchmark engine adapter, not
  product APIs. They use existing capped exact-symbol/path store searches and
  exact path/name/optional-kind filtering once per historical case, after the
  shared index build and before profile/budget queries.
- 2026-08-31: Select the path retriever's missing lexical path signal. Freeze
  12 `php-extractor-integration` retrieval-missing rows: required path,
  `Run`, and its `callers` expansion anchor across the three full-profile
  budgets and the no-relations profile. The query contains normalized `index`,
  but `SearchPaths` currently receives only explicit path-shaped terms.
  Production scope is exactly `internal/search/retrieval.go` and
  `retrieval_test.go`; FTS-only is excluded because it bypasses this retriever.
- 2026-08-31: Freeze the selected numeric gates before implementation:
  retrieval misses decrease from 12 to at most 11; at least one selected label
  advances; at least one of four selected expansion-anchor rows becomes
  `packed`; no selected row becomes not-indexed; no diagnostic label moves to
  an earlier stage; affected recall or executable anchors increase; and all
  quality, legacy, wire, privacy, deterministic, test, vet, and run-count gates
  remain green.
- 2026-08-31: Implement the selected retriever change without changing the
  parser or store: append normalized words after explicit path hints only when
  `plan.Relations` is empty. This keeps one bounded `SearchPaths` call and
  prevents broad lexical candidates from becoming structural relation anchors.
- 2026-08-31: Treat the Task 7 result as a valid negative hypothesis and stop.
  Do not adjust path-store result allocation, limits, fusion/ranking, or packing
  in v0.6; any disposition of the experimental commit or successor hypothesis
  requires a new approved plan.
- 2026-08-31: Freeze the improvement only after the eight-case repeat-1 trace.
  Choose the stage with the greatest number of actionable misses; a tie selects
  retrieval because it is earlier in the pipeline and requires fewer semantic
  assumptions. `label_not_indexed` cases are excluded from this choice and
  require a later extractor milestone.
- 2026-08-31: The exact selected cases, labels, reason code, production files,
  and numeric pass threshold must be appended here and committed in Task 5.
  No production file may change in the same commit.

---

## Outcomes & Retrospective

Task 0 closed v0.5 truthfully and established this plan. Record v0.6 measured
coverage, the selected single change, rejected alternatives, benchmark run
counts, remaining misses, and next-milestone recommendation here as each gate
completes. Do not claim improvement, Linux race, remote CI, or full-suite
success without the corresponding fresh command or Actions evidence.

Task 1 froze the starting state at `6582dc5`: Go 1.27.0 on Windows amd64 with
CGO enabled, v0.5 report object `f914facbfbf55c450fd26769bdc7bd6a992112dc`,
and v0.5 archive object `281211f6754bd3b1e45a7b321d8aaab1a1a27094`.
The ignored temporary two-case artifacts were removed; `.focalspan.json`
remained the sole untracked path.

Task 2 established `focalspan.benchmark-attribution.v1` entirely inside
`internal/benchmark`. The initial focused test failed at compile time because
the attribution API was absent. The minimal implementation then passed 7
focused tests, all 46 `internal/benchmark` tests, and `go test ./... -count=1`
with 664 tests in 46 packages; `git diff --check` passed. The schema has no
source-content field and does not alter normal CLI, MCP, or Evidence output.

Task 3 extended the opt-in search trace with raw relative identity/position,
sanitized relation state, kind, and final ranked position. Tests prove dropped
raw candidates remain observable, relation implementation names and candidate
content do not serialize, and traced and ordinary compilation produce identical
Packet JSON. The benchmark engine adapter remains unused by the normal runner,
so its existing one-index-per-case lifecycle is unchanged. Search/app/benchmark
focused packages passed 105 tests; the full repository passed 664 tests in 46
packages, the dedicated CLI/MCP/Evidence packages passed 157 tests, and diff
check passed.

Task 4 joined indexed, raw-retrieved, ranked, and packed identities only when
the new benchcli attribution outputs are requested. The development two-case
repeat-1 smoke ran once and returned 12 quality results, compatible true and
zero regressions; its 25 labels were retrieval misses. The planned eight-case
repeat-1 diagnostic then ran once with retry count zero and returned 48 quality
results, also compatible true with zero regressions. Its 40 Evidence-profile
results classified 95 labels as 55 retrieval missing, 35 packing dropped, and
5 packed. Privacy, finite-value, JSON, LF, and residue checks passed; temporary
quality/workspace artifacts were removed. Focused benchmark/benchcli tests
passed 61 tests, the full suite passed 665 tests in 46 packages, and diff check
passed. No full repeat-3 candidate run has occurred.

Task 5 applied the frozen 55-versus-0 retrieval/linking selection rule and then
bounded the production hypothesis to one general defect and 12 measured rows.
It rejected linker changes, packing/Evidence work, FTS/fusion limit increases,
ranking changes, parser changes, and corpus aliases. This decision changed only
PLAN/findings; no production file changed.

Task 6 first proved the missing behavior with `path hints=[[]]`, then added the
minimal lexical path input. The first full suite caught a relation-anchor
regression; a separate RED test reproduced broad hints entering a relation plan,
and the implementation was constrained without weakening the eval. The two
focused tests, the Japanese JSTS regression test, all 21 search tests, and 667
full tests in 46 packages passed; diff check passed. Candidate benchmark gates
remain Task 7 and have not yet been claimed.

Task 7 passed 295 targeted legacy/Evidence/wire/privacy tests, then ran one
two-case repeat-1 candidate smoke. Quality remained compatible with zero
regressions and privacy checks passed, but only the four selected required-path
rows advanced, to packing dropped. Selected retrieval misses fell from 12 to 8;
`Run` and all four anchors remained retrieval missing, so executable anchors
stayed zero and packet recall did not improve. Full tests passed 667 tests in 46
packages, vet and diff check passed, and temporary artifacts were removed. The
frozen gate failure ends v0.6 production work before Task 8; no full repeat-3
candidate, push, or fresh remote CI was run.

---

## Context and Orientation

The relevant current flow is:

    internal/query.PlanQuery
      -> internal/search.RetrieverSet.Retrieve (raw RankedList per retriever)
      -> internal/search.fuseRankedLists (weighted RRF, cap 400)
      -> internal/rank.RankWithPlan
      -> app.Config.MaxCandidates limit
      -> internal/evidence.Compiler.Compile
      -> evidence.Packet

`internal/search/trace.go` already defines retriever IDs, retrieval
contributions, and ranked candidate traces. `internal/search/search.go` creates
the trace only when `SearchRequest.Trace` is true. `internal/app/evidence.go`
currently calls `SearchDetailed` without that flag and returns only the
candidate list to the compiler. `internal/benchmark/engine.go` returns only the
Packet. `internal/benchmark/runner.go` measures Packet labels and expansions.

The trace path will be additive:

    benchmark Engine.QueryEvidenceAttributed
      -> app.Service.QueryEvidenceAttributed (development-only internal API)
      -> search.SearchDetailed(... Trace: true)
      -> Evidence Compiler using the same ranked candidates
      -> benchmark.AttributionTrace exact-label join

The ordinary `QueryEvidence`, CLI positional query, and MCP handlers retain
their current calls and responses.

---

## Interfaces and Dependencies

Add content-free stage types in `internal/search/trace.go`:

    type StageCandidateTrace struct {
        Retriever        RetrieverID
        Position         int
        Path             string
        Symbol           string
        Kind             string
        Relation         string
        RelationResolved bool
    }

    type SearchTrace struct {
        Mode       RetrievalMode
        Lists      []RetrieverSummary
        Retrieved  []StageCandidateTrace
        Candidates []CandidateTrace
    }

`Retrieved` preserves retriever execution order and one-based list position.
It never contains handle, signature, content, score detail text, or absolute
path. Existing ranked `CandidateTrace` gains `Kind` and `RankedPosition` only.

Add a development-only result in `internal/app/evidence.go`:

    type AttributedEvidenceResult struct {
        Compile evidence.CompileResult
        Trace   search.SearchTrace
    }

    func (s *Service) QueryEvidenceAttributed(
        ctx context.Context,
        req EvidenceQueryRequest,
    ) (AttributedEvidenceResult, error)

This method shares query validation, update policy, ranking, and compilation
with `QueryEvidence`; it must not be called from `cmd/focalspan`, CLI, renderer,
or MCP packages.

Extend the benchmark engine only:

    QueryEvidenceAttributed(
        context.Context,
        app.EvidenceQueryRequest,
    ) (app.AttributedEvidenceResult, error)

Add `internal/benchmark/attribution.go`:

    const AttributionSchemaV1 = "focalspan.benchmark-attribution.v1"

    type AttributionResult struct {
        Schema       string
        CaseID       string
        RepositoryID string
        Profile      string
        Budget       int
        Labels       []AttributionLabel
    }

    type AttributionLabel struct {
        Expectation    string
        Path           string
        Symbol         string
        Kind           string
        Relation       string
        TerminalStage  string
        ReasonCode     string
        RetrieverHits  []AttributionHit
        RankedPosition int
        PackedPosition int
    }

`AttributionHit` contains only retriever, one-based position, and sanitized
relation state. JSON field order is stable. Output sorts by suite order,
profile order, budget, expectation order, then trace position; map iteration
must never determine serialized order.

No new third-party dependency or database migration is permitted.

---

## Plan of Work

### Task 0: Close v0.5 and Transition the Sole Active Plan

**Files:** final v0.5 `PLAN.md`, `docs/evaluation.md`,
`docs/benchmarks/findings-v0.5.md`; archive
`docs/superpowers/plans/completed/2026-08-31-v0.5-real-repository-evaluation.md`;
index `docs/superpowers/plans/README.md`; new root `PLAN.md`.

- [x] Verify branch, HEAD, fetched `origin/master`, merge/rebase state, tracked
  and untracked changes, and preserve `.focalspan.json`.
- [x] Inspect Actions run `33361467769` and authenticated logs for test, vet,
  Linux race, three builds, and two-case smoke/compare.
- [x] Commit only final v0.5 evidence documents as `56d0f84`.
- [x] Copy the final v0.5 root plan to the completed archive and verify both
  files have Git blob hash `281211f6754bd3b1e45a7b321d8aaab1a1a27094`.
- [x] Update the archive index, install this root plan, run
  `git diff --check`, stage only archive/index/root plan, and commit
  `docs: start candidate attribution milestone v0.6`. (Commit `6582dc5`.)

### Task 1: Freeze the v0.6 Starting Baseline

**Files:** modify `PLAN.md`; create
`docs/benchmarks/findings-v0.6.md` as the living measurement record.

- [x] Record status/branch/HEAD/origin delta, Go environment, and the v0.5
  report object hash.
- [x] Run `go test ./... -count=1`, `go vet ./...`, and `git diff --check`.
- [x] Validate all eight public cases without running the full quality matrix.
- [x] Run the existing two-case repeat-1 smoke to a temporary report and compare
  it with the same rows of `docs/benchmarks/results-v0.5.json`.
- [x] Record exact results and cleanup in PLAN/findings; commit only those docs.

### Task 2: Define the Source-Free Attribution Schema and Classification

**Files:** create `internal/benchmark/attribution.go` and
`attribution_test.go`. Keep serialization with the schema because no legacy
report or metrics type needs to change.

- [x] Write failing privacy tests whose candidates contain source and absolute
  path sentinels; assert output contains neither and rejects absolute labels.
- [x] Verify expected RED because the attribution API is absent.
- [x] Implement the stable schema and terminal-stage precedence defined above.
- [x] Test exact required path, symbol, optional kind, and expansion-anchor
  matching plus constrained `linking_unresolved` classification.
- [x] Test deterministic ordering, finite numbers, sanitized reason codes, LF
  goldens, and JSON round-trip.
- [x] Run focused tests, full tests, and diff check; commit only schema/report.

### Task 3: Add the Opt-In Pre-Packet Trace Adapter

**Files:** modify `internal/search/{trace.go,search.go,search_test.go}`,
`internal/app/{evidence.go,evidence_test.go}`, and
`internal/benchmark/{engine.go,engine_test.go}`.

- [x] Write a failing search test requiring raw retriever identity/position,
  sanitized relation state, and ranked position for dropped/surviving items.
- [x] Verify RED, then populate only whitelisted fields when `Trace` is true.
- [x] Write failing app tests for `QueryEvidenceAttributed` and unchanged normal
  Packet/MCP JSON with no trace/ranking/candidate/debug fields.
- [x] Verify RED, then share the smallest query/compile path so normal and
  attributed calls produce identical Packet bytes.
- [x] Extend benchmark engine/fakes and retain one index per case.
- [x] Run focused tests, full tests, diff check; commit named adapter files.

### Task 4: Integrate Attribution and Measure the Frozen Corpus

**Files:** modify benchmark runner/report, benchcli run/tests, PLAN/findings;
create `docs/benchmarks/attribution-v0.6.{json,md}`.

- [x] Write failing runner tests requiring labels for every required path,
  required symbol, and expansion anchor in each Evidence result; legacy gets no
  fabricated attribution.
- [x] Verify RED, implement exact joins and deterministic source-free output.
- [x] Run the two development cases at repeat 1; require unchanged comparison
  and clean privacy fields.
- [x] Record before the diagnostic: eight cases, repeat 1, planned count 1,
  reason = classify frozen labels.
- [x] Run that eight-case repeat-1 diagnostic once; do not rebuild v0.5.
- [x] Scan for source, absolute path, username, environment, NaN/Infinity, and
  generated residue; record stage counts and commit code/artifacts.

### Task 5: Freeze One Improvement Decision Before Production Code

**Files:** modify only `PLAN.md` and `docs/benchmarks/findings-v0.6.md`.

- [x] Exclude `label_not_indexed`; count actionable retrieval/linking misses.
- [x] Select the greater count, retrieval on tie; choose the smallest coherent
  subset explained by one general defect and record rejected alternatives.
- [x] Freeze affected identities, baseline stage/count, exact production/test
  files, and gates: at least one label advances, chosen misses decrease, none
  move backward, affected coverage or executable anchors increase, and all
  compare/privacy/wire/legacy invariants stay green.
- [x] Commit only PLAN/findings. If no coherent defect exists, stop and present
  measured options before changing production.

### Task 6: Implement Exactly One Retriever or Linker Improvement

**Conditional scope frozen by Task 5:** retriever work may touch only the exact
subset of `internal/query`, `internal/search`, or matching store search method;
linker work may touch only the exact subset of `internal/linker` or relation
lookup. Neither may touch rank, evidence, parsers, packer, labels, or v0.5 data.

- [x] Name the mutation caught and derive literal expectations independently.
- [x] Write and run the focused failing test; record expected RED.
- [x] Implement the minimum corpus-independent rule.
- [x] Run focused GREEN, adjacent packages, full tests, and diff check.
- [x] Update living-plan sections and commit only selected code/tests plus PLAN.

### Task 7: Verify the Bounded Candidate Before the Full Suite

- [x] Run affected packages and attribution privacy/wire tests.
- [x] Run all legacy fixture evaluations and the Evidence compare suite without
  weakening checked-in values.
- [x] Run the same two-case repeat-1 smoke and v0.5 comparison.
- [x] Require Task 5 frozen gates. They failed; record the negative hypothesis
  and stop without a second production adjustment.
- [x] Run full tests, vet, diff check; commit the bounded verification record.

### Task 8: Run the One Final Full Candidate Comparison

**Files:** create `docs/benchmarks/results-v0.6.{json,md}`; modify
findings/evaluation/PLAN.

- [ ] Report before running: final candidate reason, selected improvement,
  eight cases, repeat 3, planned count 1.
- [ ] Run the suite once with attribution; compare with v0.5 and require
  compatible, zero regressions, and frozen improvement.
- [ ] Record infrastructure retry cause/count before any retry; never retry a
  valid unfavorable result.
- [ ] Scan privacy/finite values/residue; record exact deltas without changing
  labels/thresholds/baseline; commit result documents.

### Task 9: Final Verification, Remote Evidence, and Retrospective

**Files:** modify design/evaluation/benchmark README/findings/PLAN.

- [ ] Run focused tests, `go test ./... -count=1`, `go vet ./...`, and diff check.
- [ ] Build CGO-free Windows amd64, Linux amd64, Darwin arm64 into temp paths and
  remove all outputs.
- [ ] Re-run every legacy fixture and Evidence comparison including wire,
  fidelity, relation, known-handle, duplication, and normal-output privacy.
- [ ] Verify two-case smoke and the single final full result without another
  valid full run.
- [ ] Scan all state for leaks/residue and confirm untouched `.focalspan.json`.
- [ ] Update docs and all living sections; commit explicit docs only.
- [ ] When remote proof is necessary, push completed v0.6 commits, inspect the
  actual latest run, and record executed test/vet/Linux race/build/smoke jobs.
  Do not remotely rerun the manual full suite without a recorded need.

---

## Validation and Acceptance

- The v0.5 archive remains byte-identical to blob
  `281211f6754bd3b1e45a7b321d8aaab1a1a27094`; root PLAN is sole active.
- Every required path/symbol/anchor has deterministic source-free attribution.
- Artifacts contain no source, absolute path, username, environment, secret,
  NaN, or Infinity; normal CLI/MCP/Evidence contains no debug fields.
- Exactly one post-freeze retriever/linker behavior changes; no co-tuning.
- At least one frozen label advances, none regresses, and affected coverage or
  executable anchor count increases.
- Final comparison has zero quality/invariant regressions; all legacy/Evidence
  gates, full tests, vet, diff check, Linux race CI, and three builds pass with
  actual evidence.
- Run counts remain: repeated two-case smoke as needed, one eight-case repeat-1
  diagnostic, one valid eight-case repeat-3 final candidate.

---

## Idempotence and Recovery

- Tests and two-case smoke use fresh temporary workspaces and never overwrite
  v0.5 results. v0.6 reports use atomic writes to explicit destinations.
- Failed benchmark runs clean workspaces; retained debug paths stay on stderr.
- `label_not_indexed` is excluded and never silently relabeled.
- A nondeterministic/leaking trace is fixed before production work.
- A failed selected hypothesis is recorded and stops v0.6; no second subsystem
  is tuned. A valid regressing full result is evidence, not a reason to rerun.
- Unreadable remote CI remains unverified; configuration is not proof.
