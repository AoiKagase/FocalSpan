# FocalSpan Real-Repository Evaluation v0.5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a durable execution-plan lifecycle and add a reproducible, privacy-safe real-repository benchmark that measures whether FocalSpan retrieves the right evidence, at the right role and budget, before any further ranking, linker, parser, or Evidence Packet optimization is attempted.

**Architecture:** Keep `PLAN.md` as the one mutable active ExecPlan, move durable authoring and lifecycle rules to `PLANS.md`, and preserve completed plans as immutable snapshots under `docs/superpowers/plans/completed/`. Add a development-only benchmark executable that materializes historical Git snapshots into temporary directories, runs the existing FocalSpan index/query/evidence pipeline without executing repository code, evaluates manually labeled requirements across token budgets and retrieval profiles, and emits sanitized deterministic quality reports plus separately identified timing measurements.

**Tech Stack:** Go 1.26+, standard library `os/exec`, `archive/tar`, `encoding/json`, `time`, existing FocalSpan `internal/app`, `internal/eval`, `internal/evidence`, `internal/search`, `internal/gitx`, and `internal/repository` packages, SQLite/FTS5 through the existing store, Git CLI invoked without a shell, Markdown and JSON report artifacts.

**Spec:** This plan is self-contained and must be maintained according to the `PLANS.md` policy created by Task 0. Existing product architecture remains documented in `docs/design.md`; verified historical measurements remain in `docs/evaluation.md`.

**Plan ID:** `v0.5-real-repository-evaluation`

**Status:** Active after this file replaces the completed v0.4 plan.

**Baseline:** `950a3b74b59ec65d372695c6a28489202c9bf1ee` (recorded from the checkout before Task 0 edits on 2026-08-31 UTC).

## Global Constraints

- Treat the current checkout as the only source of truth. Preserve every already-merged language extractor, project-metadata resolver, linker, query planner, Evidence Packet behavior, Codex MCP registration command, and test.
- Do not use `git reset`, `git restore`, `git checkout --`, `git clean`, or `git stash`. Do not discard a dirty working tree.
- Do not change production retrieval weights, parser heuristics, relation-resolution rules, Evidence Packet fields, public MCP tool names, or public CLI behavior in this milestone unless a correctness defect prevents the benchmark from observing existing behavior. A necessary defect fix must be isolated, tested, and recorded in the Decision Log.
- Do not add network access, remote repository cloning, external LLM calls, embeddings, vector search, model-as-judge evaluation, compiler execution, package-manager execution, repository build/test execution, or runtime execution of benchmarked source.
- The benchmark may invoke only the local Git executable and the FocalSpan Go code in-process. It must never invoke the benchmarked repository's scripts, hooks, build tools, tests, generators, or package managers.
- Never mutate the benchmarked repository's branch, index, worktree, refs, configuration, hooks, submodules, or Git LFS state. Historical snapshots must be materialized with read-only Git commands into a temporary directory.
- All archive extraction must reject absolute paths, drive-qualified paths, NUL bytes, and `..` traversal. Symlink entries must be skipped with a diagnostic rather than created.
- Public benchmark suites and checked-in reports must contain repository-relative paths, logical repository IDs, commit IDs, and metrics only. They must not contain absolute local paths, usernames, environment-variable values, source text, secrets, or file contents.
- Private repository mappings belong in `.focalspan-bench.json`, which must be ignored by Git. Benchmark reports default to redacting absolute paths.
- Keep the public `focalspan` executable unchanged. New benchmark commands live in the development-only `cmd/focalspan-bench` executable.
- Reuse the current application, evaluation, Evidence Packet, token measurement, query planning, search, ranking, packing, and relation code. Do not create a second search implementation for the benchmark.
- Quality output must be deterministic for identical source, suite, profile, and FocalSpan revision. Wall-clock timings are explicitly volatile and must not participate in byte-for-byte determinism checks.
- Every behavior begins with a failing test, then the minimum implementation, then a targeted test, then `go test ./...`.
- Build artifacts, extracted snapshots, temporary indexes, and benchmark working directories must be removed after successful or failed runs.
- Keep `CGO_ENABLED=0` builds working for Windows amd64, Linux amd64, and macOS arm64.
- Do not weaken, remove, rename, or silently reinterpret existing evaluation cases or v0.3/v0.4 acceptance checks.
- This milestone measures current behavior. It must finish with an evidence-based recommendation for the next milestone, not with speculative production tuning.

---

## Purpose / Big Picture

FocalSpan now has dedicated extractors for the user's common languages, a conservative cross-file linker, a deterministic query planner and retrieval fusion pipeline, and the `focalspan.context.v1` Evidence Packet. The checked-in fixture suites demonstrate budget compliance, deterministic output, source fidelity, relation validity, and good hit-at-five results on small repositories.

The remaining uncertainty is whether those results generalize to real repositories and realistic maintenance questions. Small fixtures can accidentally reward narrow ranking rules, contain few ambiguous symbol names, and omit the long files, generated code, project layouts, partial definitions, deep test trees, and cross-language naming collisions that dominate actual work.

After this milestone, a maintainer can select a historical change in a local Git repository, write a natural-language question that would have been asked before that change, label the source locations that were genuinely needed, and run:

    go run ./cmd/focalspan-bench run \
      --suite testdata/benchmark/focalspan-history.json \
      --profile default \
      --json-out .focalspan-bench/results.json \
      --markdown-out .focalspan-bench/results.md

The command will materialize each base revision without changing the repository, index it with the current FocalSpan implementation, query it at multiple token budgets, optionally perform one relation expansion with `known_handles`, and report retrieval coverage, rank, role accuracy, relation validity, wire-token cost, duplicate-source cost, budget compliance, deterministic output, changed-path diagnostic recall, and local timing measurements.

The purpose is not to claim that files changed by a later commit are automatically the only correct context. Historical diffs are candidate evidence and diagnostics. Acceptance labels remain explicit and human-reviewed. A path that did not exist at the base revision cannot be required evidence.

The next production plan must be selected from measured failure categories:

- candidate-generation or relation misses justify retriever/linker work;
- correct candidates ranked too low justify ranking work;
- correct ranked candidates omitted from the packet justify packing or semantic-zoom work;
- excessive metadata cost justifies Evidence Packet compaction;
- concentrated language-specific misses justify extractor hardening.

No one should tune all of those systems at once.

---

## Execution-Plan Lifecycle Policy

Task 0 codifies this policy in repository-root `PLANS.md`. The policy is included here so this plan remains executable before that file exists.

### Canonical files

- `PLANS.md` is the durable policy. It defines what an ExecPlan must contain, how it is updated, and how plans transition.
- `PLAN.md` is the only active ExecPlan. It is mutable while work is active and remains in the root after completion until the next plan transition.
- `docs/superpowers/plans/completed/` stores immutable snapshots of completed plans.
- `docs/superpowers/plans/superseded/` stores plans stopped before completion because their goal or architecture was replaced.
- `docs/superpowers/specs/` stores durable design specifications when a milestone needs a separate approved design.
- `docs/superpowers/plans/README.md` is a short index. It points to the active plan and lists archived plans; it does not duplicate their task content.
- `docs/implementation-plan.md` becomes a short compatibility pointer to `PLAN.md`, `PLANS.md`, and the archive. It must no longer contain a second active or stale implementation checklist.

### Transition rules

1. There is at most one active root `PLAN.md`.
2. A completed plan is not overwritten without first preserving its final checked state, Decision Log, discoveries, validation evidence, and retrospective in `docs/superpowers/plans/completed/`.
3. A superseded plan is archived under `superseded/` with an explicit reason and the successor plan ID.
4. The new root plan names its plan ID, purpose, baseline procedure, scope exclusions, concrete validation, and completion conditions.
5. A plan transition is one reviewable commit containing the archive snapshot, the new root plan, index updates, and any policy update. Product code is not mixed into that transition commit.
6. The active plan is a living document. Progress timestamps, discoveries, decisions, and outcomes are updated as work proceeds.
7. Acceptance criteria cannot be silently weakened. A changed criterion requires a dated Decision Log entry explaining the evidence and consequence.
8. Future ideas do not accumulate in the active plan. They stay in `docs/design.md` under Roadmap or in a later spec.
9. Completed archives are immutable. A factual correction is a new clearly marked addendum commit, not a rewrite of historical progress.
10. At the end of this plan, leave the fully completed v0.5 `PLAN.md` in the root. Archive it only when the next plan is introduced.

### Plan size and content rules

- One plan covers one measurable milestone.
- The plan must be understandable from the current tree and the plan alone.
- It names exact repository-relative files, interfaces, commands, and observable results.
- `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` remain current.
- Checkboxes record execution state; prose explains purpose and design.
- The plan does not paste source files wholesale or duplicate `docs/design.md`.
- The plan records actual command results rather than predicted pass counts.
- A plan that only creates scaffolding without a working end-to-end behavior is not complete.

---

## Context and Orientation

The current repository is organized around these relevant boundaries:

- `internal/app` owns the root-bound application service used by CLI and MCP.
- `internal/query` normalizes a natural-language question and creates a deterministic query plan.
- `internal/search` runs independent retrievers and retrieval modes.
- `internal/rank` applies intent-aware ranking and stable ordering.
- `internal/evidence` compiles ranked candidates into `focalspan.context.v1`, including roles, fidelity, source segments, relations, limitations, next actions, `known_handles`, and wire-token accounting.
- `internal/eval` evaluates checked-in fixture cases. It already supports legacy and Evidence contracts.
- `internal/evalcli` is the development evaluator adapter.
- `internal/gitx` contains safe Git invocation and diff parsing used by product behavior.
- `internal/repository` detects roots and enforces path containment.
- `internal/store` persists symbols, chunks, relations, and FTS5 content.
- `docs/evaluation.md` contains verified v0.2, v0.3, and v0.4 measurements.
- `testdata/repos` and `testdata/eval` contain small, checked-in fixture repositories and cases.

A **historical task** in this plan means a question evaluated against the parent or earlier base revision of a real Git change. The later target revision is used only to derive diagnostics and help humans label the case. FocalSpan always queries the base snapshot, because that is the source state a developer would have seen before implementing the change.

A **required path** is a file that a human reviewer judges necessary to answer or safely act on the question and that exists at the base revision. An **optional path** is useful but not essential. A **forbidden path** is known irrelevant evidence whose selection signals noise. A changed path from the target diff is not automatically any of those.

A **profile** is one fixed combination of retrieval mode, presentation contract, evidence mode, and budget policy. Profiles allow an ablation comparison without changing production code.

A **quality report** contains deterministic counts and ratios. A **performance report** contains durations and byte sizes that can vary across machines. They are serialized separately so timing noise cannot break deterministic comparison.

---

## Public Benchmark Schema

Create `internal/benchmark/model.go` with these exact public-to-the-package concepts. Field order in Go structs is part of deterministic JSON output and should remain stable.

    package benchmark

    const SuiteSchemaV1 = "focalspan.benchmark.v1"

    type Suite struct {
        Schema string `json:"schema"`
        Name string `json:"name"`
        Description string `json:"description,omitempty"`
        Cases []Case `json:"cases"`
    }

    type Case struct {
        ID string `json:"id"`
        Repository string `json:"repository"`
        BaseRef string `json:"base_ref"`
        TargetRef string `json:"target_ref"`
        Query string `json:"query"`
        Budgets []int `json:"budgets"`
        ExpectedIntent string `json:"expected_intent,omitempty"`
        RequiredPaths []string `json:"required_paths,omitempty"`
        OptionalPaths []string `json:"optional_paths,omitempty"`
        ForbiddenPaths []string `json:"forbidden_paths,omitempty"`
        RequiredSymbols []SymbolExpectation `json:"required_symbols,omitempty"`
        Expand []ExpandExpectation `json:"expand,omitempty"`
        Tags []string `json:"tags,omitempty"`
    }

    type SymbolExpectation struct {
        Path string `json:"path"`
        Name string `json:"name"`
        Kind string `json:"kind,omitempty"`
        Role string `json:"role,omitempty"`
    }

    type ExpandExpectation struct {
        Relation string `json:"relation"`
        From SymbolExpectation `json:"from"`
        Budget int `json:"budget"`
        RequiredPaths []string `json:"required_paths,omitempty"`
        RequiredSymbols []SymbolExpectation `json:"required_symbols,omitempty"`
        ForbiddenPaths []string `json:"forbidden_paths,omitempty"`
    }

    type RepositoryRegistry struct {
        Schema string `json:"schema"`
        Repositories map[string]string `json:"repositories"`
    }

`Repository == "self"` resolves to the Git root containing the benchmark executable's current working directory. Every other ID is resolved from a registry file or repeated `--repo ID=PATH` override. Suite files never store local absolute paths.

Path labels use repository-relative forward-slash form. Symbol labels require exact name plus path; a kind or role further narrows the expectation but never broadens it.

---

## Default Profiles

Create these named profiles in `internal/benchmark/profile.go`:

    type Profile struct {
        Name string
        RetrievalMode search.RetrievalMode
        Contract string
        EvidenceMode evidence.Mode
        Budgets []int
        RunExpansion bool
    }

    var DefaultProfiles = []Profile{
        {
            Name: "full-evidence-focused",
            RetrievalMode: search.RetrievalFull,
            Contract: "evidence",
            EvidenceMode: evidence.ModeFocused,
            Budgets: []int{1024, 2048, 4096},
            RunExpansion: true,
        },
        {
            Name: "fts-evidence-focused",
            RetrievalMode: search.RetrievalFTSOnly,
            Contract: "evidence",
            EvidenceMode: evidence.ModeFocused,
            Budgets: []int{2048},
            RunExpansion: false,
        },
        {
            Name: "no-relations-evidence-focused",
            RetrievalMode: search.RetrievalNoRelations,
            Contract: "evidence",
            EvidenceMode: evidence.ModeFocused,
            Budgets: []int{2048},
            RunExpansion: false,
        },
        {
            Name: "full-legacy-source",
            RetrievalMode: search.RetrievalFull,
            Contract: "legacy",
            EvidenceMode: evidence.ModeSource,
            Budgets: []int{2048},
            RunExpansion: false,
        },
    }

If the current checkout uses different exported constants, add a narrow adapter in `internal/benchmark`; do not rename production types merely to match this plan.

---

## File Map

Create or modify these files. Adjust a filename only when the current checkout already has an equivalent focused file; record the adjustment in the Decision Log.

- Create `PLANS.md`: durable ExecPlan policy.
- Modify `AGENTS.md`: require reading `PLANS.md` and root `PLAN.md` for multi-package work.
- Create `docs/superpowers/plans/README.md`: plan index.
- Create `docs/superpowers/plans/completed/2026-08-30-v0.4-llm-evidence-contract.md`: immutable copy of the completed prior root plan.
- Create `docs/superpowers/plans/completed/2026-08-28-v0.1-bootstrap.md`: archive of the old long-form `docs/implementation-plan.md`.
- Replace `docs/implementation-plan.md`: short planning index and compatibility pointer.
- Create `internal/benchmark/model.go`, `model_test.go`: suite schema and validation.
- Create `internal/benchmark/profile.go`, `profile_test.go`: fixed benchmark profiles.
- Create `internal/benchmark/git.go`, `git_test.go`: no-shell Git adapter and ref validation.
- Create `internal/benchmark/snapshot.go`, `snapshot_test.go`: safe `git archive` materialization.
- Create `internal/benchmark/diff.go`, `diff_test.go`: target-change diagnostics.
- Create `internal/benchmark/engine.go`, `engine_test.go`: adapter to existing app/eval/evidence behavior.
- Create `internal/benchmark/runner.go`, `runner_test.go`: case/profile/budget execution.
- Create `internal/benchmark/metrics.go`, `metrics_test.go`: deterministic quality metrics.
- Create `internal/benchmark/report.go`, `report_test.go`: JSON and Markdown reports.
- Create `internal/benchmark/compare.go`, `compare_test.go`: baseline regression comparison.
- Create `internal/benchmark/scaffold.go`, `scaffold_test.go`: human-reviewable case proposal.
- Create `internal/benchcli/run.go`, `run_test.go`: development CLI.
- Create `cmd/focalspan-bench/main.go`: development executable.
- Create `testdata/benchmark/schema-valid.json`, `schema-invalid.json`: schema fixtures.
- Create `testdata/benchmark/focalspan-history.json`: public self-history suite.
- Create `testdata/benchmark/golden/`: deterministic report goldens.
- Create `docs/benchmarks/README.md`: workflow, privacy, labeling guide.
- Create `docs/benchmarks/results-v0.5.json`: checked-in deterministic public-suite quality report.
- Create `docs/benchmarks/results-v0.5.md`: checked-in human summary without source text.
- Create `docs/benchmarks/findings-v0.5.md`: failure distribution and next-milestone decision.
- Modify `.gitignore`: ignore `.focalspan-bench.json` and `.focalspan-bench/`.
- Modify `docs/design.md`, `docs/evaluation.md`, and `README.md`: document the benchmark and measured limits.
- Create `.github/workflows/ci.yml`: Linux tests/race/public benchmark plus cross-platform builds.

---

## Progress

Update this section at every stopping point. Add UTC timestamps to completed entries and split partially completed work into completed and remaining statements.

- [x] Plan transition and durable `PLANS.md` policy committed. (2026-08-31T00:25:50Z; archive hash and planning links verified.)
- [ ] Existing v0.3/v0.4 tests and evaluation baselines recorded without production changes.
- [ ] Benchmark schema, validation, and profile definitions implemented.
- [ ] Safe historical snapshot and diff diagnostics implemented.
- [ ] Benchmark engine and multi-budget runner implemented.
- [ ] Deterministic quality metrics and separate performance measurements implemented.
- [ ] Development CLI, private registry handling, and scaffold flow implemented.
- [ ] Query-plus-expand delta evaluation implemented.
- [ ] Public FocalSpan history corpus labeled and validated.
- [ ] Failure attribution and report comparison implemented.
- [ ] Linux race CI and cross-platform build CI implemented.
- [ ] Public benchmark results and evidence-based v0.6 recommendation committed.
- [ ] Full verification completed and recorded.

---

## Surprises & Discoveries

- 2026-08-31 UTC: The checkout advanced from the proposed `b2139c5` baseline to `950a3b7` before Task 0 began. The previously observed Codex registration edits are now committed at that HEAD, so the current checkout remains the source of truth; only `.focalspan.json` remained as a pre-existing untracked file.

No implementation discoveries have been recorded at plan creation. Add concise observations with command output or test evidence as they arise. Do not delete earlier entries; correct them with a later entry.

---

## Decision Log

- 2026-08-31: Use `950a3b74b59ec65d372695c6a28489202c9bf1ee` as the actual v0.5 baseline because the checkout advanced before execution. This follows the current-checkout source-of-truth rule and preserves commit `950a3b7` as pre-existing history.

- **Decision:** Use one active root `PLAN.md`, a durable root `PLANS.md`, and immutable completed/superseded archives.
  **Rationale:** Overwriting a completed plan loses an easy-to-review history, while keeping many active-looking plans creates ambiguity for Codex. One active file preserves the simple workflow; archives preserve evidence.
  **Date/Author:** 2026-08-30 / project planning.

- **Decision:** Make v0.5 an evaluation milestone rather than another production optimization milestone.
  **Rationale:** All checked-in fixture suites currently meet their core v0.3/v0.4 gates, but fixture success does not identify the dominant failure mode on real repositories. Tuning before measuring risks overfitting and makes attribution impossible.
  **Date/Author:** 2026-08-30 / project planning.

- **Decision:** Use local Git history and `git archive`, not clones or worktrees.
  **Rationale:** `git archive` reads committed objects without changing the user's branch, index, worktree list, hooks, or configuration. It also permits a network-free benchmark.
  **Date/Author:** 2026-08-30 / project planning.

- **Decision:** Treat target diffs as diagnostics and label candidates, not automatic truth.
  **Rationale:** A later patch may touch formatting, generated files, release notes, or an implementation chosen by one developer. The minimal context needed to reason about the task is a human judgment.
  **Date/Author:** 2026-08-30 / project planning.

- **Decision:** Keep the benchmark executable development-only.
  **Rationale:** Public FocalSpan remains a small context compiler. Historical corpus generation, repeated measurement, and report comparison are maintainer workflows and should not expand the user-facing CLI contract.
  **Date/Author:** 2026-08-30 / project planning.

- **Decision:** Separate deterministic quality data from volatile timing data.
  **Rationale:** Ranking and packet output must compare byte-for-byte; wall-clock measurements vary by operating system, filesystem cache, CPU, and antivirus activity.
  **Date/Author:** 2026-08-30 / project planning.

---

## Outcomes & Retrospective

This plan has not begun implementation. At completion, replace this paragraph with the measured public history results, the number and language distribution of valid cases, observed failure categories, CI status, remaining limitations, and the selected v0.6 direction. Do not describe retrieval quality as improved unless a later production change and a controlled comparison demonstrate it.

---

### Task 0: Transition Plans Without Losing History

**Files:**
- Create: `PLANS.md`
- Create: `docs/superpowers/plans/README.md`
- Create: `docs/superpowers/plans/completed/2026-08-30-v0.4-llm-evidence-contract.md`
- Create: `docs/superpowers/plans/completed/2026-08-28-v0.1-bootstrap.md`
- Modify: `docs/implementation-plan.md`
- Modify: `AGENTS.md`
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: the current root plan, the old `docs/implementation-plan.md`, existing `docs/superpowers/plans/` and `docs/superpowers/specs/`
- Produces: one unambiguous active plan, immutable prior-plan snapshots, and durable instructions that future Codex sessions can follow without conversation history

- [x] **Step 1: Record the exact starting state**

Run from the repository root:

    git status --short
    git diff --stat
    git rev-parse HEAD
    git log -8 --oneline
    go version
    go env GOOS GOARCH CGO_ENABLED

Copy the output into the implementation session's final report and add the HEAD value to this plan's `Baseline` line. Do not alter pre-existing changes.

- [x] **Step 2: Locate and archive the completed v0.4 plan**

Run:

    git log -G "^# FocalSpan LLM Evidence Contract v0\.4 Implementation Plan$" --format=%H -- PLAN.md

Use the first commit whose `PLAN.md` contains the completed v0.4 title and checked tasks. Verify before writing:

    git show <selected-commit>:PLAN.md

The output must begin with:

    # FocalSpan LLM Evidence Contract v0.4 Implementation Plan

Write that exact blob, without formatting or checkbox changes, to:

    docs/superpowers/plans/completed/2026-08-30-v0.4-llm-evidence-contract.md

Verify byte identity by comparing the selected Git blob and archive with `git hash-object`. They must have the same object ID.

- [x] **Step 3: Archive the original long bootstrap implementation plan**

Read `docs/implementation-plan.md`. If it is still the completed bootstrap plan headed `# FocalSpan Implementation Plan`, copy its exact current bytes to:

    docs/superpowers/plans/completed/2026-08-28-v0.1-bootstrap.md

Then replace `docs/implementation-plan.md` with this concise content:

    # FocalSpan Planning Index

    The only active execution plan is [`../PLAN.md`](../PLAN.md).

    Durable plan-authoring and lifecycle rules are in [`../PLANS.md`](../PLANS.md).
    Completed and superseded plans are indexed in
    [`superpowers/plans/README.md`](superpowers/plans/README.md).

    Product architecture is documented in [`design.md`](design.md), and measured
    results are documented in [`evaluation.md`](evaluation.md).

If the file no longer matches the bootstrap description, record the discrepancy and archive its current content under a filename that includes its actual milestone name. Do not overwrite an existing archive.

- [x] **Step 4: Create `PLANS.md`**

Write the policy from this plan's `Execution-Plan Lifecycle Policy` section into `PLANS.md`, then add these required ExecPlan sections:

    Purpose / Big Picture
    Progress
    Surprises & Discoveries
    Decision Log
    Outcomes & Retrospective
    Context and Orientation
    Plan of Work or task-oriented equivalent
    Concrete Steps
    Validation and Acceptance
    Idempotence and Recovery
    Interfaces and Dependencies

State that progress timestamps use UTC, archives are immutable, acceptance changes require a Decision Log entry, and the root plan remains after completion until the next transition.

- [x] **Step 5: Update `AGENTS.md` without bloating it**

Add a short durable section:

    # ExecPlans

    For work spanning multiple packages, a public contract, or more than one
    session, read `PLANS.md` and execute the repository-root `PLAN.md`.
    `PLAN.md` is the sole active plan. Keep its Progress, discoveries, decisions,
    and outcomes current; archive it only when introducing its successor.

Do not paste task-specific benchmark rules into `AGENTS.md`.

- [x] **Step 6: Create the plan archive index**

Create `docs/superpowers/plans/README.md` with:

    # FocalSpan Execution Plan Archive

    Active plan: [`../../../PLAN.md`](../../../PLAN.md)
    Policy: [`../../../PLANS.md`](../../../PLANS.md)

    ## Completed

    - `2026-08-28-v0.1-bootstrap.md`
    - `2026-08-30-v0.4-llm-evidence-contract.md`
    - `../2026-08-28-php-structural-extraction.md`

    ## Superseded

    No superseded plans are archived at this time.

Explain in one paragraph that archived files are historical evidence and are not active instructions.

- [x] **Step 7: Verify planning links and archive uniqueness**

Run:

    git grep -n "docs/implementation-plan.md"
    git grep -n "PLAN.md"
    git grep -n "PLANS.md"
    git diff --check

Verify:

- every link resolves;
- no document other than root `PLAN.md` calls itself the current active plan;
- the archived v0.4 plan is not modified by later tasks;
- `docs/superpowers/plans/superseded/` may be absent until the first superseded plan, but the policy names it.

- [x] **Step 8: Commit the plan transition separately**

Commit only plan governance and documentation transition files:

    git add PLANS.md PLAN.md AGENTS.md docs/implementation-plan.md docs/superpowers/plans
    git commit -m "docs: establish execution plan lifecycle"

Do not include benchmark implementation code in this commit.

---

### Task 1: Capture the Untuned v0.5 Baseline

**Files:**
- Modify: `docs/evaluation.md`
- Create: `docs/benchmarks/README.md`
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: all current tests, all checked-in fixture evaluations, Evidence comparison evaluation, current CLI/MCP smoke behavior
- Produces: a truthful pre-benchmark record that later tasks must not misrepresent as an improvement

- [ ] **Step 1: Run the current static and unit baseline**

Run:

    git diff --check
    go test ./...
    go vet ./...

If one fails before benchmark code is added, record it under `Surprises & Discoveries` with the exact failing package and message. Do not attribute it to v0.5.

- [ ] **Step 2: Run the current Evidence contract checks**

Run the current checked-in Evidence comparison command documented by the repository. At minimum, execute the equivalent of:

    go run ./cmd/focalspan-eval \
      --root testdata/repos/evidencesample \
      --cases testdata/eval/evidence-cases.jsonl \
      --contract compare \
      --json

Adapt only the executable path if the current checkout names the development evaluator differently. Record expected coverage, role accuracy, fidelity validity, relation validity, wire-budget compliance, deterministic output, forbidden violations, known resends, duplicate-source ratio, metadata overhead, Evidence/legacy ratio, and two-step delta ratio.

- [ ] **Step 3: Run every current language fixture evaluation**

Discover case files from the current tree rather than a stale hard-coded list:

    find testdata/eval -maxdepth 1 -type f -name "*cases.jsonl" -print

On Windows PowerShell, use:

    Get-ChildItem testdata/eval -Filter "*cases.jsonl"

Run each case file against its documented fixture root with a freshly rebuilt index. Record which suites have a numeric historical baseline and which only have a current measurement. Do not call a current-only measurement an improvement.

- [ ] **Step 4: Create the benchmark documentation shell**

Create `docs/benchmarks/README.md` with these headings and prose:

    # FocalSpan Real-Repository Benchmarks
    ## What the benchmark measures
    ## What it does not prove
    ## Privacy model
    ## Historical task labeling
    ## Running the public self-history suite
    ## Running private local suites
    ## Interpreting quality and timing reports
    ## Choosing the next optimization milestone

State explicitly that no source text is written to reports and that a target diff is not automatic ground truth.

- [ ] **Step 5: Record the v0.5 starting record**

Append `## Real-Repository Evaluation v0.5 starting baseline` to `docs/evaluation.md`. Include:

- date in UTC;
- exact commit;
- operating system and architecture;
- Go version;
- whether the worktree was initially dirty;
- unit/vet result;
- current Evidence metrics;
- number of checked-in language suites;
- unverified race coverage, if any.

- [ ] **Step 6: Commit only baseline documentation**

Run:

    git add docs/evaluation.md docs/benchmarks/README.md PLAN.md
    git commit -m "docs: record real-repository benchmark baseline"

---

### Task 2: Define and Validate the Benchmark Schema

**Files:**
- Create: `internal/benchmark/model.go`
- Create: `internal/benchmark/model_test.go`
- Create: `internal/benchmark/profile.go`
- Create: `internal/benchmark/profile_test.go`
- Create: `testdata/benchmark/schema-valid.json`
- Create: `testdata/benchmark/schema-invalid.json`

**Interfaces:**
- Produces: `benchmark.LoadSuite`, `benchmark.ValidateSuite`, `benchmark.LoadRegistry`, `benchmark.ResolveProfiles`, and the schema types defined earlier
- Consumes: existing `search.RetrievalMode` and `evidence.Mode` values without changing them

- [ ] **Step 1: Write failing valid-suite load tests**

Create `TestLoadSuiteValid` using a fixture equivalent to:

    {
      "schema": "focalspan.benchmark.v1",
      "name": "valid",
      "cases": [
        {
          "id": "callers",
          "repository": "self",
          "base_ref": "HEAD~1",
          "target_ref": "HEAD",
          "query": "what calls ValidateToken?",
          "budgets": [1024, 2048],
          "expected_intent": "callers",
          "required_paths": ["internal/app/service.go"],
          "required_symbols": [
            {
              "path": "internal/app/service.go",
              "name": "ValidateToken",
              "role": "target"
            }
          ],
          "expand": [
            {
              "relation": "callers",
              "from": {
                "path": "internal/app/service.go",
                "name": "ValidateToken"
              },
              "budget": 1200,
              "required_paths": ["internal/mcpserver/server.go"]
            }
          ]
        }
      ]
    }

Assert exact field preservation and forward-slash paths.

- [ ] **Step 2: Write failing validation table tests**

Cover these errors with stable message fragments:

- unsupported or empty schema;
- empty suite name;
- no cases;
- duplicate case IDs;
- empty repository/base/target/query;
- identical base and target refs;
- no budgets;
- budget below 256 or above 64000;
- unsorted or duplicate budgets;
- absolute path;
- path containing `..`;
- backslash path;
- same path in required and forbidden lists;
- symbol expectation missing path or name;
- unsupported role;
- unsupported relation;
- expansion budget outside the accepted range;
- duplicate expectation.

Validation must return all case-specific errors in deterministic order rather than stopping after a random map iteration.

- [ ] **Step 3: Implement exact schema types and normalization**

Implement:

    func LoadSuite(path string) (Suite, error)
    func ValidateSuite(s Suite) error
    func LoadRegistry(path string) (RepositoryRegistry, error)
    func NormalizeSuite(s Suite) Suite

`NormalizeSuite` may trim strings and sort/deduplicate tags, but it must not silently repair invalid paths, refs, budgets, or expectations.

- [ ] **Step 4: Add profile tests**

Assert that `ResolveProfiles("default")` yields the four profiles defined in this plan in stable order. Also support selecting one or more comma-separated exact profile names. Unknown profiles return an error listing valid names.

- [ ] **Step 5: Add a schema round-trip determinism test**

Load `schema-valid.json`, marshal with `json.MarshalIndent`, unmarshal, normalize, marshal again, and assert byte equality. Timing fields do not exist in suite files.

- [ ] **Step 6: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestLoadSuite|TestValidateSuite|TestProfiles" -count=1
    go test ./...
    git diff --check
    git add internal/benchmark testdata/benchmark
    git commit -m "feat: define real-repository benchmark schema"

---

### Task 3: Materialize Safe Read-Only Git Snapshots

**Files:**
- Create: `internal/benchmark/git.go`
- Create: `internal/benchmark/git_test.go`
- Create: `internal/benchmark/snapshot.go`
- Create: `internal/benchmark/snapshot_test.go`
- Create: `internal/benchmark/diff.go`
- Create: `internal/benchmark/diff_test.go`

**Interfaces:**
- Produces:

      type CommandResult struct {
          Stdout []byte
          Stderr []byte
      }

      type CommandRunner interface {
          Run(ctx context.Context, dir string, name string, args ...string) (CommandResult, error)
      }

      type Snapshot struct {
          RepositoryID string
          Commit string
          Root string
          SkippedSymlinks []string
          FileCount int
      }

      type Snapshotter interface {
          Materialize(ctx context.Context, repositoryID, repositoryPath, ref, destination string) (Snapshot, error)
      }

      type ChangeSet struct {
          BaseCommit string
          TargetCommit string
          Files []ChangedFile
      }

      type ChangedFile struct {
          OldPath string
          NewPath string
          Status string
          Binary bool
          Ranges []LineRange
      }

- Consumes: local Git executable only; may reuse parsing helpers from `internal/gitx` when their semantics match

- [ ] **Step 1: Write a no-shell command-runner test**

Use a fake executable or argument-recording fake runner and assert arguments remain separate for:

    git rev-parse --verify <ref>^{commit}
    git archive --format=tar <commit>
    git diff --unified=0 --no-ext-diff --find-renames <base> <target> --

A ref containing spaces or shell metacharacters is passed as one argument and never executed by a shell.

- [ ] **Step 2: Write temporary-repository snapshot tests**

Create a temporary Git repository in the test, commit:

    src/main.go
    docs/readme.md
    nested/data.txt

Materialize `HEAD` and assert:

- destination contains the three regular files with exact bytes;
- `.git` is absent;
- source repository status, HEAD, index checksum, branch, and worktree list are unchanged;
- returned commit is the full verified commit ID;
- running materialization twice into separate empty destinations produces identical file trees.

- [ ] **Step 3: Write archive traversal rejection tests**

Feed handcrafted tar entries to the extraction helper:

    /absolute
    C:/drive
    ../escape
    safe/../../escape
    name-with-NUL
    safe/link (symlink)
    safe/file

Assert unsafe entries fail the extraction, symlinks are skipped and reported, and `safe/file` is extracted only when no fatal traversal entry exists. No file may appear outside the destination.

- [ ] **Step 4: Implement snapshot materialization**

Implement:

    func ResolveCommit(ctx context.Context, runner CommandRunner, repo, ref string) (string, error)
    func ExtractGitArchive(r io.Reader, destination string) (fileCount int, skippedSymlinks []string, err error)
    func NewGitSnapshotter(runner CommandRunner) Snapshotter

Use `git archive --format=tar <full-commit>`. Stream stdout into `archive/tar`; do not buffer an unbounded archive in memory. If the existing runner only returns bytes, add a bounded streaming runner interface for archive use:

    type StreamCommandRunner interface {
        Stream(ctx context.Context, dir, name string, stdout io.Writer, args ...string) (stderr []byte, err error)
    }

Cap retained stderr diagnostics at 16 KiB.

- [ ] **Step 5: Write diff-oracle tests**

Create commits covering modified, added, deleted, renamed, and binary files. Assert `CollectChanges` returns stable slash-normalized paths and zero-context line ranges. Added files are marked `added`; they are not later accepted as required base evidence.

- [ ] **Step 6: Implement read-only diff collection**

Implement:

    func CollectChanges(
        ctx context.Context,
        runner CommandRunner,
        repositoryPath, baseRef, targetRef string,
    ) (ChangeSet, error)

Resolve both refs to commits first. Use existing `internal/gitx` parsing when possible. Add status parsing only where existing code does not expose rename/add/delete information needed by the benchmark.

- [ ] **Step 7: Test cancellation and cleanup**

Cancel during archive streaming. Assert the subprocess is terminated, partial destination is removed by the caller, the source repository is unchanged, and the error wraps `context.Canceled`.

- [ ] **Step 8: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestGit|TestSnapshot|TestArchive|TestCollectChanges" -count=1
    go test ./internal/gitx ./internal/benchmark
    go test ./...
    git diff --check
    git add internal/benchmark
    git commit -m "feat: materialize safe historical snapshots"

---

### Task 4: Build the Benchmark Engine Adapter

**Files:**
- Create: `internal/benchmark/engine.go`
- Create: `internal/benchmark/engine_test.go`
- Modify: `internal/eval/eval.go` only if a small reusable pure metric helper is currently private
- Modify: `internal/eval/eval_test.go` only for such a helper

**Interfaces:**
- Produces:

      type EngineFactory interface {
          Open(root string, retrievalMode search.RetrievalMode) (Engine, error)
      }

      type Engine interface {
          Build(ctx context.Context) (IndexMeasurement, error)
          QueryLegacy(ctx context.Context, req app.QueryRequest) (model.ContextBundle, error)
          QueryEvidence(ctx context.Context, req app.QueryRequest) (evidence.Packet, error)
          ExpandEvidence(ctx context.Context, req app.ExpandRequest) (evidence.Packet, error)
          Close() error
      }

      type IndexMeasurement struct {
          Files int
          Symbols int
          Chunks int
          Relations int
          DatabaseBytes int64
          Duration time.Duration
      }

- Consumes: the current root-bound `app.Service` and existing evaluation/evidence paths

- [ ] **Step 1: Write a fake-engine contract test**

Create a fake engine and assert the runner-facing interface can:

- build once;
- query legacy;
- query Evidence;
- expand Evidence;
- close exactly once;
- propagate context cancellation.

- [ ] **Step 2: Write an app-adapter integration test**

Materialize or copy the existing `authsample` fixture to a temporary root, open the real adapter, build the index, and query:

    ValidateToken の呼び出し元はどこですか

Assert:

- Evidence schema is `focalspan.context.v1`;
- the packet is budget compliant;
- at least one path is repository-relative;
- the adapter does not create files outside the temporary root except normal Go test temporary state;
- close removes no source file.

- [ ] **Step 3: Implement the adapter without duplicating search logic**

Add `appEngineFactory` and `appEngine`. Use the current application service's existing methods. If the current service does not accept a retrieval mode directly, use the same internal option or constructor already used by `internal/eval` ablation. Do not add a public CLI flag.

- [ ] **Step 4: Measure index state**

After a successful build, read counts and database size through existing status/store APIs and `os.Stat` on the temporary index. Do not query private SQLite tables directly if a store method already exists.

- [ ] **Step 5: Preserve current evaluation behavior**

If a metric helper moves from `internal/eval`, keep its old behavior and tests byte-for-byte. Do not merge benchmark case types with existing fixture `eval.Case`; the historical schema has different semantics.

- [ ] **Step 6: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestEngine|TestAppEngine" -count=1
    go test ./internal/app ./internal/eval ./internal/benchmark
    go test ./...
    git diff --check
    git add internal/benchmark internal/eval
    git commit -m "feat: adapt FocalSpan engine for historical benchmarks"

Stage `internal/eval` only if it changed.

---

### Task 5: Execute Cases Across Profiles and Budgets

**Files:**
- Create: `internal/benchmark/runner.go`
- Create: `internal/benchmark/runner_test.go`
- Create: `internal/benchmark/match.go`
- Create: `internal/benchmark/match_test.go`

**Interfaces:**
- Produces:

      type RunRequest struct {
          Suite Suite
          Repositories map[string]string
          Profiles []Profile
          Repeat int
          Workspace string
      }

      type Runner struct {
          Snapshotter Snapshotter
          EngineFactory EngineFactory
          Clock Clock
      }

      func (r *Runner) Run(ctx context.Context, req RunRequest) (RunReport, error)

- Consumes: validated suites, resolved repository paths, safe snapshots, existing FocalSpan engine

- [ ] **Step 1: Write exact expectation-matching tests**

Implement tests for:

    func MatchRequiredPaths(packet evidence.Packet, expected []string) MatchResult
    func MatchRequiredSymbols(packet evidence.Packet, expected []SymbolExpectation) MatchResult

Rules:

- paths match exact slash-normalized repository-relative strings;
- symbol name and path must both match;
- optional kind and role must also match when present;
- case is preserved for symbol names;
- duplicate packet items cannot satisfy one expectation twice;
- a synthetic outline may satisfy a path expectation but only satisfies a symbol expectation when its symbol, path, kind, and role match;
- invalid packet-local relation IDs do not count.

- [ ] **Step 2: Write runner sequencing tests with fakes**

For one case, two budgets, two profiles, and repeat count two, assert:

- snapshot is materialized once per case/base commit, not once per budget;
- index is built once per snapshot/profile only when retrieval mode requires a distinct service;
- each quality query runs twice;
- one warm-up query may run before timed repetitions but is excluded from quality counts;
- engines close and workspace cleanup occurs after success and error;
- case/profile/budget output order follows suite and profile order, never map order.

- [ ] **Step 3: Implement workspace layout**

Under the caller-provided temporary workspace, use:

    snapshots/<repository-id>/<base-commit>/
    runs/<case-id>/<profile-name>/
    reports/

Sanitize IDs for filenames with `[A-Za-z0-9._-]`; reject rather than silently rewrite an empty result. Never include the original absolute repository path in a generated filename.

- [ ] **Step 4: Implement query execution**

For Evidence profiles, create `app.QueryRequest` with the case query, selected budget, focused/source mode from the profile, and retrieval mode through the engine adapter. For legacy profile, call the existing legacy query path.

Run each quality query twice and compare canonical serialized outputs after removing fields documented as volatile. Evidence packets should already contain no timestamps. Any nondeterminism is a case failure.

- [ ] **Step 5: Collect query-plan intent**

Compare the observed planned primary intent with `ExpectedIntent` when specified. Obtain it from the existing query planner or packet intent; do not infer it from selected paths.

- [ ] **Step 6: Collect diff diagnostics without turning them into labels**

Call `CollectChanges` once per case. Record:

- paths changed by the target commit;
- changed paths that existed at base;
- changed paths selected in the packet;
- changed-path recall as a diagnostic;
- added paths as unavailable-at-base;
- deleted/renamed status.

Do not add changed paths to `RequiredPaths`.

- [ ] **Step 7: Add invalid-label checks against the materialized base**

Before querying, validate every required, optional, forbidden, and symbol path exists at base. A missing required or forbidden path makes the case invalid. A target-added path may appear only in diff diagnostics, never as required evidence.

- [ ] **Step 8: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestMatch|TestRunner" -count=1
    go test ./internal/benchmark
    go test ./...
    git diff --check
    git add internal/benchmark
    git commit -m "feat: run historical tasks across retrieval profiles"

---

### Task 6: Compute Deterministic Quality and Separate Performance Metrics

**Files:**
- Create: `internal/benchmark/metrics.go`
- Create: `internal/benchmark/metrics_test.go`
- Create: `internal/benchmark/report.go`
- Create: `internal/benchmark/report_test.go`
- Create: `testdata/benchmark/golden/quality-report.json`
- Create: `testdata/benchmark/golden/quality-report.md`

**Interfaces:**
- Produces:

      type QualityResult struct {
          CaseID string `json:"case_id"`
          Profile string `json:"profile"`
          Budget int `json:"budget"`
          TargetRank int `json:"target_rank"`
          ReciprocalRank float64 `json:"reciprocal_rank"`
          RequiredPathRecall float64 `json:"required_path_recall"`
          OptionalPathRecall float64 `json:"optional_path_recall"`
          RequiredSymbolRecall float64 `json:"required_symbol_recall"`
          IntentCorrect int `json:"intent_correct"`
          RoleAccuracy float64 `json:"role_accuracy"`
          RelationValid int `json:"relation_valid"`
          BudgetCompliant int `json:"budget_compliant"`
          Deterministic int `json:"deterministic"`
          ForbiddenViolations int `json:"forbidden_violations"`
          WireTokens int `json:"wire_tokens"`
          EvidenceTokens int `json:"evidence_tokens"`
          MetadataOverheadRatio float64 `json:"metadata_overhead_ratio"`
          DuplicateSourceRatio float64 `json:"duplicate_source_ratio"`
          ChangedPathRecall float64 `json:"changed_path_recall"`
          FailureCodes []string `json:"failure_codes,omitempty"`
      }

      type PerformanceResult struct {
          CaseID string `json:"case_id"`
          Profile string `json:"profile"`
          Budget int `json:"budget"`
          SnapshotMS int64 `json:"snapshot_ms"`
          IndexMS int64 `json:"index_ms"`
          QueryMS []int64 `json:"query_ms"`
          DatabaseBytes int64 `json:"database_bytes"`
          Files int `json:"files"`
          Symbols int `json:"symbols"`
          Chunks int `json:"chunks"`
          Relations int `json:"relations"`
      }

      type RunReport struct {
          Schema string `json:"schema"`
          Suite string `json:"suite"`
          FocalSpanCommit string `json:"focalspan_commit"`
          Quality []QualityResult `json:"quality"`
          Aggregate AggregateQuality `json:"aggregate"`
          Performance []PerformanceResult `json:"performance,omitempty"`
      }

- [ ] **Step 1: Write metric unit tests**

Use hand-built packets to verify:

- target rank `0` means absent and reciprocal rank `0`;
- rank one yields reciprocal rank `1`;
- path/symbol recall uses unique expectations;
- no expected optional paths yields optional recall `1`;
- forbidden duplicates count each selected evidence item once;
- role accuracy considers only matched expectations with an expected role;
- every relation endpoint must reference a packet-local evidence ID;
- budget compliance uses measured model-visible wire cost;
- metadata overhead is `(wire-evidence)/wire`, clamped to `[0,1]`;
- duplicate-source ratio counts repeated verbatim/excerpt source bytes, not repeated signatures or metadata;
- changed-path recall is diagnostic and does not affect required recall.

- [ ] **Step 2: Add aggregate metrics**

Aggregate by profile and budget. Include:

- case count and invalid-case count;
- hit-at-1, hit-at-3, hit-at-5;
- mean reciprocal rank;
- median required path recall;
- median required symbol recall;
- intent accuracy;
- role accuracy;
- budget compliance;
- deterministic output;
- total forbidden violations;
- median wire tokens;
- median metadata overhead;
- median duplicate-source ratio;
- median changed-path recall.

Use stable integer/rational accumulation where possible, then format floats consistently.

- [ ] **Step 3: Separate quality and timing serialization**

`MarshalQuality` excludes the `performance` field and is byte-deterministic. `MarshalFullReport` includes timings. Golden determinism tests compare only `MarshalQuality`.

- [ ] **Step 4: Implement sanitized Markdown rendering**

The Markdown report includes repository IDs, case IDs, commit IDs, labels, ranks, ratios, failure codes, and timings. It must not include:

- absolute repository or temporary paths;
- source text;
- query result source segments;
- usernames;
- environment values.

Queries may be included because suite authors explicitly wrote them, but escape Markdown tables safely.

- [ ] **Step 5: Add golden report tests**

Generate the JSON and Markdown goldens from fixed fake results. Run twice and assert identical bytes and LF newlines.

- [ ] **Step 6: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestMetric|TestAggregate|TestReport|TestGolden" -count=1
    go test ./internal/benchmark
    go test ./...
    git diff --check
    git add internal/benchmark testdata/benchmark/golden
    git commit -m "feat: report benchmark quality and performance"

---

### Task 7: Add Query-Plus-Expand Delta Measurement

**Files:**
- Modify: `internal/benchmark/runner.go`
- Modify: `internal/benchmark/metrics.go`
- Modify: `internal/benchmark/report.go`
- Modify: `internal/benchmark/runner_test.go`
- Modify: `internal/benchmark/metrics_test.go`

**Interfaces:**
- Extends `QualityResult` with:

      ExpandRequiredPathRecall float64 `json:"expand_required_path_recall,omitempty"`
      ExpandRequiredSymbolRecall float64 `json:"expand_required_symbol_recall,omitempty"`
      ExpandForbiddenViolations int `json:"expand_forbidden_violations,omitempty"`
      ExpandRelationValid int `json:"expand_relation_valid,omitempty"`
      CumulativeWireTokens int `json:"cumulative_wire_tokens,omitempty"`
      CumulativeWireTokensWithoutKnown int `json:"cumulative_wire_tokens_without_known,omitempty"`
      DeltaTokenRatio float64 `json:"delta_token_ratio,omitempty"`
      KnownResendCount int `json:"known_resend_count,omitempty"`

- [ ] **Step 1: Write anchor-selection tests**

The expansion anchor must be selected only by exact `From.Path` plus `From.Name`, optionally narrowed by kind. Never choose the first evidence item when the exact anchor is absent.

Expected failures:

    anchor_missing
    anchor_ambiguous
    unsupported_relation

- [ ] **Step 2: Write a known-handle delta test**

Create an initial packet containing target, caller, and test. Expand `callers` twice:

1. with all initial handles passed as `known_handles`;
2. without known handles.

Assert:

- known version resends none of the initial handles;
- packet-local relations do not dangle;
- cumulative wire tokens are query wire plus expand wire;
- `DeltaTokenRatio = withKnown / withoutKnown`;
- no divide-by-zero yields NaN or infinity.

- [ ] **Step 3: Implement expansion execution**

For every `ExpandExpectation` and Evidence profile with `RunExpansion`, locate the exact anchor, call existing `ExpandEvidence`, and pass all handles already visible in the initial packet. Run the control expansion without known handles only for measurement; do not expose it to users.

- [ ] **Step 4: Match expansion expectations**

Use the same exact path/symbol rules as initial queries. Required expansion labels are independent of initial labels. A path already returned initially may still be required context, but it must not be retransmitted when known handles are used; count it as already satisfied only when the suite expectation explicitly targets the initial packet.

- [ ] **Step 5: Add aggregate delta metrics**

Report median delta-token ratio and total known resend count. Preserve the v0.4 invariant that known resend count is zero.

- [ ] **Step 6: Verify and commit**

Run:

    go test ./internal/benchmark -run "TestExpand|TestKnown|TestDelta" -count=1
    go test ./internal/evidence ./internal/app ./internal/benchmark
    go test ./...
    git diff --check
    git add internal/benchmark
    git commit -m "feat: benchmark multi-step evidence expansion"

---

### Task 8: Add Case Scaffolding and Private Repository Mapping

**Files:**
- Create: `internal/benchmark/scaffold.go`
- Create: `internal/benchmark/scaffold_test.go`
- Create: `internal/benchcli/run.go`
- Create: `internal/benchcli/run_test.go`
- Create: `cmd/focalspan-bench/main.go`
- Modify: `.gitignore`

**Interfaces:**
- Produces development commands:

      focalspan-bench validate
      focalspan-bench scaffold
      focalspan-bench run
      focalspan-bench compare

- [ ] **Step 1: Ignore private benchmark state**

Add exactly:

    .focalspan-bench.json
    .focalspan-bench/

Do not ignore checked-in `testdata/benchmark/` or `docs/benchmarks/`.

- [ ] **Step 2: Write repository-resolution tests**

Resolution precedence:

1. repeated `--repo ID=PATH`;
2. registry file passed with `--registry`;
3. `.focalspan-bench.json` in the FocalSpan repository root;
4. `self`, resolved to the current Git root.

Reject duplicate IDs with differing paths, missing IDs, non-directories, non-Git repositories, and NUL values. JSON output never includes resolved absolute paths.

- [ ] **Step 3: Write scaffold tests**

Command shape:

    focalspan-bench scaffold \
      --repository self \
      --base <base-ref> \
      --target <target-ref> \
      --query "where is MCP evidence output assembled?"

The output is one valid case proposal containing:

- resolved full commit IDs;
- query;
- default budgets `[1024,2048,4096]`;
- changed existing files in `candidate_paths`;
- added files in `unavailable_at_base`;
- empty required/optional/forbidden labels;
- no source text.

Because `candidate_paths` is scaffold-only metadata and not part of the final Suite `Case`, define:

    type CaseProposal struct {
        Case Case `json:"case"`
        CandidatePaths []string `json:"candidate_paths"`
        UnavailableAtBase []string `json:"unavailable_at_base"`
    }

The user must review and move selected paths into explicit labels.

- [ ] **Step 4: Implement `validate`**

Command:

    focalspan-bench validate --suite <path> [--registry <path>] [--repo ID=PATH]

It validates schema, resolves refs, materializes each base snapshot, verifies every label exists at base, and prints one line per case plus a summary. `--json` emits a stable report without absolute paths.

- [ ] **Step 5: Implement `run`**

Command:

    focalspan-bench run \
      --suite <path> \
      --profile default \
      --repeat 3 \
      --json-out <path> \
      --markdown-out <path> \
      [--registry <path>] \
      [--repo ID=PATH] \
      [--keep-workspace]

Default behavior:

- use a newly created temporary workspace;
- redact absolute paths;
- remove workspace on success and failure;
- repeat quality output twice and timing measurement three times;
- write JSON and Markdown atomically;
- refuse to overwrite output unless `--force`;
- `--keep-workspace` is an explicit debugging option and prints the retained path to stderr, never into checked-in report content.

- [ ] **Step 6: Implement `compare`**

Initial command shape:

    focalspan-bench compare \
      --baseline <quality-report.json> \
      --candidate <quality-report.json> \
      [--json]

Task 10 defines comparison semantics. Wire the command now with a fake comparator test, then complete semantics there.

- [ ] **Step 7: Keep stdout/stderr disciplined**

- normal command output and JSON go to stdout;
- errors and retained workspace diagnostics go to stderr;
- no source text is logged;
- command errors return nonzero;
- validation error output identifies case ID and field.

- [ ] **Step 8: Verify and commit**

Run:

    go test ./internal/benchcli ./internal/benchmark -count=1
    go run ./cmd/focalspan-bench validate --suite testdata/benchmark/schema-valid.json
    go test ./...
    git diff --check
    git add .gitignore cmd/focalspan-bench internal/benchcli internal/benchmark
    git commit -m "feat: add development benchmark command"

---

### Task 9: Create a Public Self-History Corpus

**Files:**
- Create: `testdata/benchmark/focalspan-history.json`
- Create: `testdata/benchmark/focalspan-history-labels.md`
- Modify: `docs/benchmarks/README.md`

**Interfaces:**
- Consumes: FocalSpan's own Git history
- Produces: a source-free, reproducible real-history suite that exercises integration points across extraction, retrieval, linking, MCP, and Evidence compilation

- [ ] **Step 1: Identify candidate target commits by feature, not by recency alone**

Use `git log --all --oneline --decorate` and `git show --stat` to find commits that introduced or completed these eight themes:

1. PHP/`.inc` structural extraction integration.
2. C/C++ first-class extractor integration.
3. JavaScript/TypeScript first-class extraction or module relations.
4. Rust first-class extraction.
5. .NET WinForms/WPF/XAML/RESX integration.
6. Japanese query planning and relation retrieval.
7. Static project metadata plus conservative linker integration.
8. LLM Evidence Packet or `known_handles` integration.

For each theme, select a target commit and an ancestor base immediately before the relevant implementation. A merge commit may use its first parent only when that parent is the actual pre-feature tree.

- [ ] **Step 2: Enforce label feasibility**

For each case:

- every required, optional, and forbidden path must exist at base;
- new files introduced by target cannot be required;
- required symbols must exist in the base index;
- the query must describe a maintainer question that could reasonably be asked before the target change;
- at least one required path must be an existing integration point changed by the target;
- forbidden paths must be plausibly confusable, not arbitrary junk.

Use `focalspan-bench scaffold` to inspect candidates, then manually label.

- [ ] **Step 3: Write English and Japanese queries**

At least three of the eight cases use Japanese mixed with an exact code identifier, for example:

    PHPの.inc判定はどこで決まりますか
    code_contextのEvidence Packetはどこで組み立てられますか
    Rust extractorをregistryへ追加する場所はどこですか

Do not use target-only symbol names that did not exist at base. The remaining cases may use English.

- [ ] **Step 4: Add at least three expansion expectations**

Cover three different relations from:

    callers
    callees
    imports
    references
    tests
    children

Each anchor must exist at base and be uniquely labeled by path and symbol.

- [ ] **Step 5: Document human labeling rationale**

In `testdata/benchmark/focalspan-history-labels.md`, for each case record:

- why the question is realistic;
- why each required path is necessary;
- why optional paths are not required;
- why forbidden paths are irrelevant;
- which target-diff paths were deliberately not labeled;
- confirmation that all labels exist at base.

Do not include source code or absolute paths.

- [ ] **Step 6: Validate the suite**

Run:

    go run ./cmd/focalspan-bench validate \
      --suite testdata/benchmark/focalspan-history.json

Expected:

    cases: 8
    invalid: 0

The exact human formatting may differ, but both values must be present.

- [ ] **Step 7: Add anti-overfitting review**

Search production code for every case ID and distinctive query phrase:

    git grep -n "<case-id>"
    git grep -n "<distinctive-query-fragment>"

Matches are allowed only in benchmark data, tests directly loading that data, or documentation. No production ranking, parser, query, linker, or Evidence code may contain them.

- [ ] **Step 8: Commit corpus separately**

Run:

    git add testdata/benchmark/focalspan-history.json \
      testdata/benchmark/focalspan-history-labels.md \
      docs/benchmarks/README.md
    git commit -m "test: add FocalSpan historical task corpus"

---

### Task 10: Attribute Failures and Compare Reports

**Files:**
- Create: `internal/benchmark/failure.go`
- Create: `internal/benchmark/failure_test.go`
- Create: `internal/benchmark/compare.go`
- Create: `internal/benchmark/compare_test.go`
- Modify: `internal/benchmark/report.go`
- Modify: `internal/benchcli/run.go`
- Modify: `internal/benchcli/run_test.go`

**Interfaces:**
- Produces stable failure codes:

      label_invalid
      intent_mismatch
      target_not_selected
      target_below_5
      required_path_missing
      required_symbol_missing
      forbidden_selected
      relation_invalid
      role_mismatch
      ranked_candidate_not_packed
      budget_exceeded
      nondeterministic_output
      expansion_anchor_missing
      expansion_anchor_ambiguous
      expansion_required_missing
      known_handle_resent

- Produces:

      type Comparison struct {
          Compatible bool `json:"compatible"`
          Regressions []Regression `json:"regressions,omitempty"`
          Improvements []Regression `json:"improvements,omitempty"`
          Warnings []string `json:"warnings,omitempty"`
      }

- [ ] **Step 1: Write failure-code tests**

Given controlled case and packet outcomes, assert exact codes and deterministic ordering. A single case may have multiple codes. Do not attempt to infer compiler-level root cause.

- [ ] **Step 2: Distinguish ranking from packing where current internals permit it**

Use the current app/search trace only through an internal benchmark adapter. If the required candidate appears in the ranked pre-packet candidate list but not the Evidence Packet, emit `ranked_candidate_not_packed`. If the current checkout has no safe internal trace, do not add a public debug command; record only `required_*_missing` and document the attribution limit.

Any adapter added to `internal/app` must be unexported outside internal packages or explicitly named as development-only.

- [ ] **Step 3: Implement quality comparison**

Reports are compatible only when these match:

- suite name;
- case IDs;
- profile names;
- budgets;
- schema major version.

A candidate regression is:

- budget compliance falls;
- deterministic output falls;
- forbidden violations increase;
- required path or symbol recall falls;
- intent or role accuracy falls;
- relation validity falls;
- known resend count increases;
- median wire tokens increase by more than 10% without any required-recall improvement.

Performance changes are warnings only by default. A query or index median slowdown greater than 20% is a warning, not exit-code failure.

- [ ] **Step 4: Define compare exit codes**

- `0`: compatible and no quality regression;
- `2`: compatible with one or more quality regressions;
- `3`: incompatible schemas/suites or invalid report;
- `1`: I/O or command failure.

Human output lists exact case/profile/budget regressions. JSON output contains no ANSI escapes.

- [ ] **Step 5: Add source-free diagnostic detail**

For misses, report expected path/symbol, selected path/symbol/role list, and failure code. Never include source content, source segments, absolute paths, or full private registry data.

- [ ] **Step 6: Verify and commit**

Run:

    go test ./internal/benchmark ./internal/benchcli -run "TestFailure|TestCompare" -count=1
    go test ./...
    git diff --check
    git add internal/benchmark internal/benchcli
    git commit -m "feat: compare benchmark quality and classify misses"

---

### Task 11: Run the Public Benchmark and Freeze v0.5 Measurements

**Files:**
- Create: `docs/benchmarks/results-v0.5.json`
- Create: `docs/benchmarks/results-v0.5.md`
- Create: `docs/benchmarks/findings-v0.5.md`
- Modify: `docs/evaluation.md`
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: public history suite and completed benchmark executable
- Produces: truthful, source-free, reproducible current-behavior measurements and a data-driven recommendation

- [ ] **Step 1: Run the public suite from a clean benchmark workspace**

Build the development binary into a temporary path and run:

    go build -o .focalspan-bench/focalspan-bench ./cmd/focalspan-bench

On Windows use `.focalspan-bench/focalspan-bench.exe`.

Then run:

    .focalspan-bench/focalspan-bench run \
      --suite testdata/benchmark/focalspan-history.json \
      --profile default \
      --repeat 3 \
      --json-out docs/benchmarks/results-v0.5.json \
      --markdown-out docs/benchmarks/results-v0.5.md \
      --force

Remove the binary after the run. The command may use its own temporary snapshot workspace, but no snapshot is checked in.

- [ ] **Step 2: Verify deterministic quality output**

Run the same suite again to a second temporary JSON file. Compare deterministic quality sections with `focalspan-bench compare`. The comparison must return exit code zero. Timing differences are allowed.

- [ ] **Step 3: Verify hard invariants**

The public suite must have:

    invalid cases = 0
    budget compliance = 1.0
    deterministic output = 1.0
    relation validity = 1.0 for packets that contain relations
    forbidden violations = 0
    known resend count = 0
    NaN or infinite metrics = 0
    absolute paths in checked-in reports = 0
    source text in checked-in reports = 0

Do not invent a minimum required-path recall after seeing the result. Record the measured value honestly.

- [ ] **Step 4: Analyze failure distribution**

Write `docs/benchmarks/findings-v0.5.md` with:

- suite and profile summary;
- recall by budget;
- English versus Japanese query comparison;
- per-theme misses;
- failure-code counts;
- changed-path diagnostic versus human-required recall;
- delta-token behavior;
- metadata and duplicate-source cost;
- index/query timing context;
- limitations of using one repository's history.

- [ ] **Step 5: Select the next milestone using measured evidence**

Use this decision order:

1. If most missing required evidence never reaches the selected/ranked candidate set, recommend retriever or linker improvements.
2. If required evidence is ranked but excluded from the packet, recommend query-aware semantic zoom or utility-per-token packing.
3. If evidence coverage is good but metadata overhead is consistently high, recommend Evidence Packet compaction.
4. If misses cluster in one language or artifact type, recommend that extractor or project-metadata resolver.
5. If no category dominates, recommend expanding the private/local corpus before production tuning.

Name exactly one primary v0.6 direction and at most one secondary candidate. Do not start implementing either in this plan.

- [ ] **Step 6: Append verified results to `docs/evaluation.md`**

Add `## Real-Repository Evaluation v0.5` with:

- exact FocalSpan commit;
- suite case count;
- profile/budget matrix;
- hard invariant results;
- aggregate quality;
- delta metrics;
- timing caveat;
- link to findings;
- statement that no production retrieval tuning occurred in v0.5.

- [ ] **Step 7: Commit measurement artifacts**

Run:

    git add docs/benchmarks docs/evaluation.md PLAN.md
    git commit -m "docs: record real-repository benchmark results"

---

### Task 12: Add Continuous Verification and Linux Race Coverage

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/benchmarks/README.md`
- Modify: `PLAN.md`

**Interfaces:**
- Produces: automated unit/vet/race/cross-build and public benchmark regression checks
- Consumes: the checked-in v0.5 quality report as a baseline

- [ ] **Step 1: Add Linux test and race jobs**

Create a workflow triggered by pull requests and pushes to `master`. Use the repository's declared supported Go version. Jobs:

    test:
      ubuntu-latest
      go test ./...
      go vet ./...

    race:
      ubuntu-latest
      go test -race ./...

Do not claim the Windows-local race limitation is resolved until the Linux job actually passes in GitHub Actions.

- [ ] **Step 2: Add CGO-free build matrix**

Matrix:

    windows-latest / windows amd64
    ubuntu-latest / linux amd64
    macos-latest / darwin arm64

Run:

    go build ./cmd/focalspan

with `CGO_ENABLED=0`, `GOOS`, and `GOARCH` set for the target. Write artifacts to the runner temporary directory; do not upload binaries in this milestone.

- [ ] **Step 3: Add public benchmark smoke and comparison**

On Ubuntu:

    go run ./cmd/focalspan-bench validate \
      --suite testdata/benchmark/focalspan-history.json

Run the suite to a temporary candidate report and compare against `docs/benchmarks/results-v0.5.json`. Fail on quality regression exit code `2` or incompatibility `3`. Do not fail on timing warnings.

- [ ] **Step 4: Add workflow safety tests or static review**

Verify the workflow:

- never accesses private registry files;
- never needs network after normal Go dependency checkout/download;
- never runs benchmarked repository code;
- does not persist extracted snapshots;
- does not print source text;
- uses least required permissions:

      permissions:
        contents: read

- [ ] **Step 5: Document CI**

README's development section should name:

    go test ./...
    go test -race ./...
    go vet ./...
    go run ./cmd/focalspan-bench validate ...
    go run ./cmd/focalspan-bench run ...

State that `focalspan-bench` is a maintainer tool and not part of the end-user CLI contract.

- [ ] **Step 6: Commit CI separately**

Run local YAML review, then:

    git diff --check
    git add .github/workflows/ci.yml README.md docs/benchmarks/README.md PLAN.md
    git commit -m "ci: verify race builds and historical benchmark"

---

### Task 13: Final Verification, Documentation, and Plan Retrospective

**Files:**
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/benchmarks/README.md`
- Modify: `docs/benchmarks/findings-v0.5.md`
- Modify: `docs/superpowers/plans/README.md`
- Modify: `PLAN.md`
- Modify: `AGENTS.md` only if the durable plan rule is still incomplete

**Interfaces:**
- Consumes: all v0.5 code, reports, CI configuration, existing product behavior
- Produces: a complete, reproducible milestone with a truthful next-step decision

- [ ] **Step 1: Update architecture documentation**

In `docs/design.md`, add a development evaluation section:

    local Git repository
      -> git archive base snapshot
      -> current FocalSpan index/query/evidence
      -> human labels plus target-diff diagnostics
      -> deterministic quality report
      -> separate timing report

State that the benchmark is not part of the product path and does not permit network or repository code execution.

- [ ] **Step 2: Update README without turning it into benchmark internals**

Add a short maintainer section linking to `docs/benchmarks/README.md`. Keep user quick-start material focused on `focalspan`.

- [ ] **Step 3: Verify plan-policy consistency**

Check:

- `PLANS.md` exists and describes the actual lifecycle;
- root `PLAN.md` is the only active plan;
- v0.4 archive is byte-identical to its Git blob;
- `docs/implementation-plan.md` is only a pointer;
- completed archives are not edited after Task 0;
- plan archive index links resolve.

- [ ] **Step 4: Run formatting, static checks, and all tests**

Run:

    gofmt -w .
    git diff --check
    go test ./...
    go vet ./...

Expected: all pass. Record actual package/test counts rather than copying old counts.

- [ ] **Step 5: Run race tests where supported**

Run:

    go test -race ./...

If the local Windows toolchain still cannot build race support, record it as unverified locally. Confirm the Linux CI result separately once available; do not call a configured but unrun workflow a pass.

- [ ] **Step 6: Run CGO-free native and cross-builds**

Direct all outputs to `.focalspan-bench/builds/` and remove them afterward:

    CGO_ENABLED=0 go build ./cmd/focalspan
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan
    CGO_ENABLED=0 go build ./cmd/focalspan-bench

Use the shell-appropriate environment syntax. Verify no binary remains in the repository root.

- [ ] **Step 7: Re-run every legacy language and Evidence evaluation**

Run all checked-in `testdata/eval/*cases.jsonl` against their documented fixture roots. Requirements:

    no lower hit@5 than the recorded current baseline
    no lower symbol/path recall
    no new forbidden violation
    budget compliance remains 1.0
    deterministic output remains 1.0
    Evidence source fidelity and relation validity remain 1.0
    known resend count remains 0

A benchmark-only milestone should not change these metrics. Investigate any change.

- [ ] **Step 8: Re-run the public history suite and compare**

Run to a temporary candidate report, then:

    go run ./cmd/focalspan-bench compare \
      --baseline docs/benchmarks/results-v0.5.json \
      --candidate <temporary-report>

Expected exit code: `0`.

- [ ] **Step 9: Check privacy and cleanup**

Run searches that would expose accidental local data:

    git grep -nE "([A-Za-z]:\\\\|/home/|/Users/|H:/|C:/Users/)" -- \
      docs/benchmarks testdata/benchmark

Review every match; expected matches are none unless a clearly synthetic example is documented. Also verify:

    git status --short

contains no extracted snapshot, private registry, database, generated binary, tar archive, or temporary report.

- [ ] **Step 10: Complete living-plan sections**

Update:

- `Progress` with UTC completion timestamps;
- `Surprises & Discoveries` with evidence;
- `Decision Log` for every changed acceptance or architecture choice;
- `Outcomes & Retrospective` with measured results and the chosen v0.6 direction.

Do not erase the initial decisions.

- [ ] **Step 11: Final self-review**

Review the final diff for:

- benchmark code accidentally entering public CLI/MCP;
- shell invocation;
- source-repository mutation;
- unsafe tar extraction;
- absolute-path leaks;
- source-text leaks;
- diff paths treated as automatic required labels;
- timing fields used in deterministic comparison;
- map-order nondeterminism;
- production ranking changes;
- weakened old evaluations;
- stale active-plan references;
- unchecked boxes marked without evidence.

Fix issues and rerun affected commands.

- [ ] **Step 12: Commit final documentation and verification record**

Run:

    git add README.md docs PLANS.md PLAN.md AGENTS.md
    git commit -m "docs: complete real-repository evaluation v0.5"

Stage `AGENTS.md` only if it changed after Task 0. Leave the completed root `PLAN.md` in place. Do not archive v0.5 until a successor plan is introduced.

---

## Concrete End-to-End Acceptance

From the FocalSpan repository root, a maintainer must be able to run:

    go run ./cmd/focalspan-bench validate \
      --suite testdata/benchmark/focalspan-history.json

and observe eight valid cases and zero invalid cases.

Then:

    go run ./cmd/focalspan-bench run \
      --suite testdata/benchmark/focalspan-history.json \
      --profile default \
      --repeat 3 \
      --json-out .focalspan-bench/candidate.json \
      --markdown-out .focalspan-bench/candidate.md \
      --force

must:

- leave the checked-out source repository unchanged;
- create no Git worktree or branch;
- execute no repository code;
- perform no network request;
- remove extracted snapshots unless `--keep-workspace` was explicitly provided;
- produce a source-free, absolute-path-free report;
- include every case/profile/budget in deterministic order;
- keep every Evidence result within its token budget;
- identify invalid labels before query execution;
- measure query-plus-expand delta behavior where specified.

Finally:

    go run ./cmd/focalspan-bench compare \
      --baseline docs/benchmarks/results-v0.5.json \
      --candidate .focalspan-bench/candidate.json

must return zero when the deterministic quality metrics match or improve and must ignore normal timing variance.

---

## Validation and Acceptance

The milestone is complete only when all statements are true:

- `PLANS.md` defines the durable lifecycle and root `PLAN.md` is the sole active plan.
- The completed v0.4 root plan and original bootstrap plan are preserved in the archive.
- The old `docs/implementation-plan.md` no longer competes with the active plan.
- The benchmark schema rejects ambiguous, unsafe, duplicate, or unavailable labels.
- Local repository paths are resolved outside suite files and are redacted from reports.
- Historical source is materialized with read-only Git commands into safe temporary directories.
- The benchmark never changes branches, refs, index, working files, hooks, submodules, or configuration.
- Tar traversal and symlink cases are tested.
- Existing FocalSpan product code performs indexing, retrieval, ranking, Evidence compilation, and expansion; the benchmark does not duplicate them.
- Default profiles compare full Evidence at three budgets, FTS-only, no-relations, and legacy presentation.
- Quality output is deterministic; timing output is separate.
- Initial and expansion expectations use exact path/symbol matching.
- Target diffs remain diagnostics rather than automatic ground truth.
- The public history suite contains eight valid human-reviewed cases and at least three expansions.
- At least three cases use Japanese mixed with code identifiers.
- Reports contain no source text or absolute local paths.
- Budget compliance, deterministic output, relation validity, forbidden-path safety, and known-handle no-resend hard invariants pass.
- The current measured recall is recorded without post-hoc threshold manipulation.
- The final findings select one primary v0.6 direction from measured failure categories.
- Linux CI runs `go test -race ./...`; CGO-free cross-platform builds remain configured.
- Existing language/evidence evaluations do not regress.
- No incomplete production path or panic-based stub remains.
- The completed plan's outcomes and actual command evidence are recorded.

---

## Idempotence and Recovery

- `validate` and `run` are safe to repeat. They materialize fresh temporary snapshots and never alter source repositories.
- Atomic output writes use a temporary sibling file followed by rename. A failed run leaves the previous report untouched unless `--force` was used and replacement completed.
- Workspace cleanup runs through `defer` after creation. Cancellation and errors remove partial snapshots unless `--keep-workspace` was explicit.
- If `git archive` fails, report repository ID, sanitized ref, and capped Git stderr; do not fall back to a worktree or clone.
- If a base label is invalid, stop that case before indexing and mark the suite invalid. Do not silently remove the label.
- If a target ref disappears after history rewriting, validation fails clearly. Update the suite through a reviewed data commit rather than resolving to a nearby commit.
- If checked-in quality reports become schema-incompatible, `compare` returns exit code `3`; create an explicitly versioned migration or new baseline rather than coercing fields.
- If Task 0 runs after the new `PLAN.md` has already been committed, locate the v0.4 plan through Git history by its exact title and archive that blob. Do not archive the new plan under the old filename.
- If the local environment cannot run race tests, preserve the local unverified status and use the Linux CI result. Do not weaken the requirement.
- If real-history recall is poor, complete the measurement milestone and document it. Do not change production weights inside v0.5 merely to make the report green.

---

## Interfaces and Dependencies

No new third-party Go dependency is expected. Use the standard library and current FocalSpan packages.

The benchmark package depends inward on product packages; product packages must not import `internal/benchmark` or `internal/benchcli`.

Allowed direction:

    cmd/focalspan-bench
      -> internal/benchcli
      -> internal/benchmark
      -> internal/app, internal/eval, internal/evidence,
         internal/search, internal/gitx, internal/repository

Forbidden direction:

    cmd/focalspan
      -> internal/benchmark

    internal/app or internal/evidence
      -> internal/benchmark

`CommandRunner` must use `exec.CommandContext` with separate arguments. No `cmd /c`, `sh -c`, PowerShell command string, or shell quoting parser is allowed.

The suite and report schemas are versioned independently:

    focalspan.benchmark.v1
    focalspan.benchmark-report.v1

A backward-incompatible schema change requires a new major schema identifier and a comparison incompatibility result; it must not silently reinterpret checked-in data.
