# FocalSpan Real-Repository Findings v0.5

## Scope and reproducibility

The public `focalspan-history-v0.5` suite measured eight human-labeled cases at
FocalSpan commit `be153f5ae5c40fb04f3daf5608211482dced7d25`. Four queries are
English and four mix Japanese with code identifiers. The default matrix
contains full Evidence at 1024, 2048, and 4096 tokens, plus FTS-only Evidence,
no-relations Evidence, and legacy presentation at 2048 tokens: 48 quality
results in total.

Two complete `--repeat 3` runs returned exit code 0. Their deterministic JSON
reports had the same Git object hash,
`f914facbfbf55c450fd26769bdc7bd6a992112dc`; `focalspan-bench compare`
reported `compatible: true` and zero regressions. Timing values are deliberately
excluded from that comparison.

## Hard invariants

Across the checked-in report:

- invalid cases: 0;
- budget-compliance failures: 0;
- nondeterministic results: 0;
- invalid relation endpoints reported by Evidence results: 0;
- initial and expansion forbidden-path violations: 0;
- known-handle resends observed: 0;
- NaN or infinite values: 0;
- absolute local paths, source fields, and retained benchmark workspaces: 0.

The known-handle result needs an important qualification. All three labeled
expansion cases failed to select their exact anchor at all three full-Evidence
budgets, producing nine `expansion_anchor_missing` codes. No expansion packet
was therefore executed, so delta-token ratio and resend behavior were not
measurable on this public corpus. Zero resends is true for executed packets,
but it is not evidence that the public history cases exercised the two-step
path successfully.

## Recall by budget and profile

Full Evidence did not improve when the budget rose. At each of 1024, 2048, and
4096 tokens, required-path mean recall was 0.125, its median was 0, required-
symbol mean and median recall were 0, hit@1/hit@3/hit@5 were all 0.125, and
mean reciprocal rank was 0.125. FTS-only and no-relations Evidence at 2048 had
the same recall and rank aggregates. The evidence profile therefore showed no
budget, relation-mode, or FTS/full separation on these labels.

The legacy profile is retained as a presentation control. Its benchmark row
does not compute the Evidence-specific path, symbol, intent, role, or wire
metrics, so its zero values must not be read as a measured legacy retrieval
comparison.

| Case theme | Required path | Required symbol | Target rank | Intent | Failure summary at full Evidence / 2048 |
|---|---:|---:|---:|---:|---|
| PHP extractor integration | 0 | 0 | 0 | 1 | target, path, symbol, expansion anchor |
| C/C++ extractor registry | 0 | 0 | 0 | 1 | target, path, symbol |
| JavaScript/TypeScript search integration | 0 | 0 | 0 | 1 | target, path, symbol, expansion anchor |
| Rust registry integration | 0 | 0 | 0 | 1 | target, path, symbol |
| .NET structural registry | 0 | 0 | 0 | 0 | intent, target, path, symbol |
| Japanese query normalization | 0 | 0 | 1 | 1 | path, symbol |
| Project metadata indexing | 1 | 0 | 0 | 1 | target, symbol |
| MCP Evidence output | 0 | 0 | 0 | 1 | target, path, symbol, expansion anchor |

Across all 40 Evidence-profile results, stable failure-code counts were:

| Failure code | Count |
|---|---:|
| `required_symbol_missing` | 40 |
| `required_path_missing` | 35 |
| `target_not_selected` | 35 |
| `expansion_anchor_missing` | 9 |
| `intent_mismatch` | 5 |

The benchmark exposes selected packet labels but intentionally has no safe
pre-packet ranking trace. These results prove that required evidence was absent
from the final packet; they cannot distinguish a retriever/linker miss from a
candidate that ranked but was later excluded by packing. No
`ranked_candidate_not_packed` code was emitted.

## English and Japanese comparison

At full Evidence / 2048, the four Japanese-mixed cases had required-path mean
recall 0, required-symbol mean recall 0, target hit rate 0.25, and intent
accuracy 1.0. The four English cases had required-path mean recall 0.25,
required-symbol mean recall 0, target hit rate 0, and intent accuracy 0.75.
Because language and task theme are confounded in only eight cases, this is not
evidence of a language-specific regression. Misses occur in both groups, while
the only intent mismatch is the English .NET theme.

## Diff diagnostics, packet cost, and timing

The changed-path diagnostic mean was 0.0669 while human-required path mean
recall was 0.125; both medians were 0. The project-metadata case retrieved its
required path while changed-path recall was only 0.3333, and the PHP and .NET
cases returned some changed paths without satisfying required labels. This
supports retaining target diffs as diagnostics rather than automatic truth.

For full Evidence, median model-visible wire tokens were 259 and median
metadata overhead was 0.9242 at all three budgets. Median duplicate-source
ratio was 0. The overhead is high, but the decision order puts missing required
coverage ahead of compaction because increasing the budget did not recover any
additional labeled evidence.

On this Windows run, full-Evidence snapshot time ranged from 164 to 454 ms,
index time from about 6.7 to 333.4 seconds, and median query time from 12 to 66
ms. The later historical revisions dominate runtime. These values are local
context only: antivirus, filesystem cache, CPU scheduling, and operating system
make them unsuitable for quality regression status.

## v0.6 direction

**Primary direction:** improve and instrument retriever/linker candidate
coverage for real-history integration points and exact symbols. The dominant
observable failure is missing required evidence and missing exact expansion
anchors, not budget pressure; v0.6 should first add a development-only,
source-free pre-packet attribution trace, then use it to raise candidate recall
without embedding corpus-specific IDs or queries.

**Secondary candidate:** after candidate coverage and expansion anchors are
measurably restored, evaluate Evidence Packet metadata compaction. The measured
0.9242 median overhead justifies keeping this candidate, but compaction is not
the first move while required coverage is near zero.

No production parser, retrieval weight, relation resolver, ranker, packer, or
Evidence Packet contract was tuned during v0.5.

## Task 13 regression verification

The third and final full public-history run completed with eight cases and 48
quality results. Its quality rows and aggregates were identical to the
checked-in baseline; the only report difference was the later FocalSpan commit
ID. `compare` returned exit code 0 with `compatible: true` and zero regressions.
Candidate-report privacy searches found no source field, absolute local path,
NaN, or infinity, and retained benchmark workspace count was zero after
cleanup. No fourth full run was performed.

The full local unit suite and vet passed, all five requested CGO-free builds
passed, and all legacy and Evidence evaluations retained their recorded
metrics. Local Windows race remains unverified because the available C
compiler cannot build 64-bit `runtime/cgo`.

An externally triggered GitHub Actions run later passed all three CGO-free
build jobs but failed Linux test and race. The user cancelled its additional
public benchmark during measurement, before comparison. The saved GitHub CLI
credential was invalid, but an authenticated browser session later exposed the
logs: both jobs failed only because their depth-one checkout could not resolve
the historical refs required by two benchmark CLI tests. Test and race now use
`fetch-depth: 0`, protected by a focused workflow regression test. No post-fix
remote run has occurred, so no remote test, vet, race, or benchmark success is
claimed.

## Post-fix remote CI closure

GitHub Actions run [`33361467769`](https://github.com/AoiKagase/FocalSpan/actions/runs/33361467769)
later completed successfully at `ca54f11`. Authenticated logs confirmed Linux
`go test ./...`, `go vet ./...`, and `go test -race ./...`; CGO-free Windows
amd64, Linux amd64, and Darwin arm64 builds; and the bounded two-case repeat-1
benchmark flow. The smoke validated 2 cases with 0 invalid, produced 12 quality
results, and compared as compatible with 0 regressions. The manual full job was
skipped, so no additional eight-case repeat-3 result is claimed.
