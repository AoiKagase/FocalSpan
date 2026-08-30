# FocalSpan Structural Extraction Evaluation

## Dataset

The checked-in fixtures cover the original Go/PHP behavior and the three
first-class structural profiles:

- `testdata/repos/authsample`: Go authentication service, caller, test,
  configuration, documentation, and an unrelated report.
- `testdata/repos/phpsample`: PHP namespace/use, classes and methods, PHPUnit,
  `.inc`, and mixed HTML/PHP coverage.
- `testdata/repos/cppsample`: C and C++ authentication code, headers/includes,
  callers, tests, and a legacy `.c` function.
- `testdata/repos/csharpsample`: C# namespace/types, interface references,
  partial class declarations, callers, and xUnit/NUnit/MSTest-style tests.
- `testdata/repos/jstssample`: JavaScript/TypeScript, JSX/TSX, ESM/CommonJS,
  imports/exports, callers, nested tests, and an unrelated report.

The matching cases are `testdata/eval/cases.jsonl`, `php-cases.jsonl`,
`cpp-cases.jsonl`, `csharp-cases.jsonl`, and `jsts-cases.jsonl` under
`testdata/eval/`.

The original Go fixture is intentionally ordinary code, and production ranking
contains no fixture-specific names or query branches. The added language
fixtures use the same evaluator and do not add fixture-specific ranking rules.

## Metrics

For every case, the development-only `focalspan-eval` binary runs the same query twice and records:

- hit@1, hit@3, hit@5 for expected symbols;
- expected symbol recall and expected path recall;
- forbidden-path violations;
- final token-budget compliance;
- median estimated tokens;
- reduction ratio, where the baseline is the estimated token count of the full
  candidate files returned by retrieval;
- deterministic output equality between repeated runs.

The MVP acceptance thresholds are:

| Metric | Threshold |
| --- | ---: |
| Budget compliance | 100% |
| Fixture expected symbol hit@5 | 100% |
| Forbidden path violations | 0 |
| Deterministic result | 100% |
| Fixture median reduction ratio | <= 0.25 |
| Source result path and line range | present for every item |
| Unrelated full-file return | none |

## Reproduction

From the repository root:

```text
focalspan update --rebuild --root testdata/repos/authsample
go run ./cmd/focalspan-eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
focalspan update --rebuild --root testdata/repos/phpsample --quiet
go run ./cmd/focalspan-eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
focalspan update --rebuild --root testdata/repos/cppsample --quiet
go run ./cmd/focalspan-eval --root testdata/repos/cppsample --cases testdata/eval/cpp-cases.jsonl --json
focalspan update --rebuild --root testdata/repos/csharpsample --quiet
go run ./cmd/focalspan-eval --root testdata/repos/csharpsample --cases testdata/eval/csharp-cases.jsonl --json
focalspan update --rebuild --root testdata/repos/jstssample --quiet
go run ./cmd/focalspan-eval --root testdata/repos/jstssample --cases testdata/eval/jsts-cases.jsonl --json
```

The evaluation output is the evidence for the thresholds. A failed or
unexecuted command remains explicitly unverified; it is not converted into a
pass by documentation.

`go test -race ./...` remains environment-blocked in this Windows run: the
default environment reports `CGO_ENABLED=0`, `CC=gcc`, and no `gcc` executable.
The normal test, vet, and CGO-free native/cross-build checks are recorded
separately and do not imply race-test coverage.

## PHP fixture results

The PHP fixture was evaluated separately from the Go fixture with four cases:
expired-token production code, its middleware caller, PHPUnit coverage, and a
`.inc` bootstrap include. The measured result was:

| Metric | PHP result |
| --- | ---: |
| hit@1 / hit@3 / hit@5 | 0.25 / 1.00 / 1.00 |
| Symbol recall / path recall | 1.00 / 1.00 |
| Budget compliance | 1.00 |
| Forbidden path violations | 0 |
| Deterministic result | 1.00 |
| Median estimated tokens | 179 |
| Median reduction ratio | 0.05550387596899225 |

The individual callers case had a 0.2708 reduction ratio; the reported median
remained within the `<= 0.25` acceptance threshold. Every returned item had
an existing fixture path and a valid source line range, and
`unrelated/Report.php` was absent from all four bundles.

## Template fixture results

The template fixture is `testdata/repos/templatesample` and its cases are in
`testdata/eval/template-cases.jsonl`. The verified run after indexing produced:

| Metric | Template result |
| --- | ---: |
| hit@1 / hit@3 / hit@5 | 0.80 / 1.00 / 1.00 |
| Symbol recall / path recall | 1.00 / 1.00 |
| Budget compliance | 1.00 |
| Forbidden path violations | 0 |
| Deterministic result | 1.00 |
| Median estimated tokens | 87 |
| Median reduction ratio | 0.026568430453226165 |

Path-only cases use `expected_paths` for hit@N when no expected symbol is
declared; path recall still requires every expected path. The run verified
embedded JavaScript source lines against the original `.tpl`, static imports
and inheritance candidates, and no full `unrelated/report.tpl` result.

## First-class structural profile results

The following measurements were rerun from the current checkout with the
matching fixture and JSONL cases. Each case is queried twice by the evaluator.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| C/C++ (includes C) | 5 | 0.40 / 0.80 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 146 | 0.1667 |
| C# | 5 | 0.60 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 92 | 0.1215 |
| JavaScript/TypeScript | 6 | 0.6667 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 121 | 0.15419501133786848 |

All three profiles meet the existing thresholds: hit@5 100%, budget
compliance 100%, no forbidden-path violations, deterministic output, and
median reduction at or below 0.25. The cases validate structural symbol/path
recall and relation retrieval; they do not claim compiler-grade type
inference, overload resolution, virtual dispatch, or package/module graph
resolution.

## Retrieval Quality v0.2 pre-change baseline

This baseline was measured from the current checkout on 2026-08-29 before
the v0.2 retrieval changes. Each fixture was indexed immediately before its
case set was evaluated with the existing FTS-first implementation.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Go/auth | 4 | 0.75 / 1.00 / 1.00 | 1.00 / 0.875 | 1.00 | 1 | 1.00 | 157 | 0.03597 |
| PHP | 4 | 0.50 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 173 | 0.05364 |
| Smarty/template | 5 | 0.80 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 88 | 0.04449 |
| C/C++ | 5 | 0.40 / 0.80 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 146 | 0.16667 |
| C# | 5 | 0.60 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 92 | 0.12153 |
| JavaScript/TypeScript | 6 | 0.8333 / 0.8333 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 145 | 0.20195 |

The Go/auth result includes one forbidden-path violation in the existing
baseline. The v0.2 acceptance record must report whether that regression is
removed or remains; it must not silently replace this baseline. Full per-case
JSON output is retained in the rollout evidence for this run.

## Retrieval Quality v0.2 post-change acceptance

This is the measured result from the current checkout on 2026-08-29. Each
fixture root was indexed immediately before its case set, and each case was
queried twice by the evaluator. The unresolved Go test-name matching regression
test is included in this run.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Go/auth | 4 | 0.50 / 1.00 / 1.00 | 1.00 / 0.875 | 1.00 | 0 | 1.00 | 175 | 0.04009 |
| PHP | 4 | 0.25 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 179 | 0.05550 |
| Smarty/template | 5 | 0.80 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 110 | 0.02456 |
| C/C++ | 5 | 0.40 / 0.80 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 137 | 0.14952 |
| C# | 5 | 0.20 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 93 | 0.13559 |
| JavaScript/TypeScript | 6 | 0.8333 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 138 | 0.17602 |

Every existing full-mode profile meets the hit@5, budget, forbidden-path,
determinism, and median-reduction thresholds. Go/auth path recall remains
0.875 because the impact case expects two paths while its syntax-only impact
result returns one; this is recorded rather than hidden by changing the case or
threshold.

Compared with the pre-change table, hit@1 decreased for Go/auth (0.75 to
0.50), PHP (0.50 to 0.25), and C# (0.60 to 0.20). The new fusion and intent
profiles move structurally supported relation candidates ahead of some former
first-place lexical candidates; hit@3/hit@5, symbol/path recall, budget
compliance, and determinism remain within the recorded acceptance results. No
case or threshold was removed to conceal these ordering changes.

## Polyglot Coverage v0.3 starting worktree baseline

This is the measured starting-worktree record for the v0.3 implementation on
2026-08-30. The checkout was `codex/mcp-missing-registration` at `b84880a`,
with the language-detection changes already present as uncommitted worktree
changes. Because those changes could not be discarded under the worktree
preservation rule, these values are a starting-worktree baseline rather than a
pre-Task-1 source baseline. Each fixture was indexed immediately before its
case set, and each case was queried twice by `focalspan-eval`.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Go/auth | 4 | 0.50 / 1.00 / 1.00 | 1.00 / 0.875 | 1.00 | 0 | 1.00 | 175 | 0.04009163802978236 |
| PHP | 4 | 0.25 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 179 | 0.05550387596899225 |
| Smarty/template | 5 | 0.80 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 110 | 0.024559053360125028 |
| C/C++ | 5 | 0.40 / 0.80 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 137 | 0.14952279957582185 |
| C# | 5 | 0.20 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 93 | 0.13559322033898305 |
| JavaScript/TypeScript | 6 | 0.8333333333333334 / 1.00 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 138 | 0.1760204081632653 |

The Japanese ablation run also completed from the same starting worktree. The
full mode achieved hit@3/hit@5 `1.0/1.0` for `ja-auth` and `0.6667/1.0` for
`ja-jsts`; `fts-only` and `no-relations` remained respectively
`0.6667/0.6667` and `0.3333/0.3333`. Full-mode relation recall was `1.0` for
both fixtures, while relation-bearing recall for both reduced modes was `0.0`.
All runs were budget-compliant and deterministic.

### Japanese ablation comparison

The following reports are the actual `--ablation all --json` results. Aggregate
relation recall counts cases without an expected relation as 1.0; the
relation-bearing cases themselves are shown separately in the final column.

| Cases | Mode | hit@3 | hit@5 | intent recall | relation recall | relation-bearing recall |
|---|---|---:|---:|---:|---:|---:|
| ja-auth (3) | full | 1.0000 | 1.0000 | 1.0000 | 1.0000 | 1.0000 |
| ja-auth (3) | fts-only | 0.6667 | 0.6667 | 1.0000 | 0.3333 | 0.0000 |
| ja-auth (3) | no-relations | 0.6667 | 0.6667 | 1.0000 | 0.3333 | 0.0000 |
| ja-jsts (3) | full | 0.6667 | 1.0000 | 1.0000 | 1.0000 | 1.0000 |
| ja-jsts (3) | fts-only | 0.3333 | 0.3333 | 1.0000 | 0.0000 | 0.0000 |
| ja-jsts (3) | no-relations | 0.3333 | 0.3333 | 1.0000 | 0.0000 | 0.0000 |

The full Go result returns `http/middleware.go` for callers and
`auth/service_test.go` for tests; the latter depends on the camel-case
`ValidateExpiredToken` to `ValidateToken` unresolved relation match. The full
JS/TS result returns `src/http/auth-middleware.ts` for both callers and
imports, while FTS-only and no-relations miss those relation-bearing paths.
All Japanese modes remained budget-compliant and deterministic.

## Verification status

On 2026-08-29 the current checkout passed `gofmt`, `go test ./...`, and
`go vet ./...`. CGO-free builds passed for the native host plus Windows amd64,
Linux amd64, and Darwin arm64. Race coverage remains environment-unverified,
not a pass: with `CGO_ENABLED=1` and
`CC=C:\msys64\ucrt64\bin\gcc.exe`, both the repository race run and the
control command `go test -race runtime/race` fail before package tests because
the Windows Go toolchain's `runtime/cgo` invocation exits with status 2. This
host-level cgo failure is tracked separately from the CLI redesign.

The fixture CLI flow passed with each root prepared immediately before
evaluation: `setup`, `status --json`, a budgeted positional JSON query, and
quiet `update`. MCP integration tests cover impact analysis, and a bounded
stdio startup smoke also passed with protocol-only stdout. The current checkout
retains five MCP tools (`code_context`, `code_expand`, `code_impact`,
`code_restart`, and `code_status`); this is the existing restart extension and
does not claim the original four-tool MVP surface.

## Interpretation

The fixtures measure retrieval, structural relation use, deduplication, and
packing, not semantic call resolution. The C/C++, C#, and JavaScript/TypeScript
extractors use pure-Go lexers/parsers and bounded recovery. Resolved relations
are limited to unique local qualified/name matches; unresolved calls,
imports/includes/exports, and references are labeled with their lexical target
and confidence. Future semantic providers may improve recall without changing
the packer or output contract.

## Polyglot Coverage v0.3 pre-change baseline

This is the fresh baseline measured from checkout `ec6f86e` on 2026-08-30,
before Task 1 of the polyglot coverage plan. The starting worktree had an
existing modification to `PLAN.md` and an untracked `.focalspan.json`; neither
was changed or included in the baseline commit. The temporary baseline binary
was built from the current checkout, and every fixture was indexed immediately
before its case set was evaluated.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Go/auth | 4 | 0.5 / 1 / 1 | 1 / 0.875 | 1 | 0 | 1 | 175 | 0.04009163802978236 |
| PHP | 4 | 0.25 / 1 / 1 | 1 / 1 | 1 | 0 | 1 | 179 | 0.05550387596899225 |
| Smarty/template | 5 | 0.8 / 1 / 1 | 1 / 1 | 1 | 0 | 1 | 110 | 0.024559053360125028 |
| C/C++ | 5 | 0.4 / 0.8 / 1 | 1 / 1 | 1 | 0 | 1 | 137 | 0.14952279957582185 |
| C# | 5 | 0.2 / 1 / 1 | 1 / 1 | 1 | 0 | 1 | 93 | 0.13559322033898305 |
| JavaScript/TypeScript | 6 | 0.8333333333333334 / 1 / 1 | 1 / 1 | 1 | 0 | 1 | 138 | 0.1760204081632653 |

The exact Japanese ablation aggregates were:

| Cases | Mode | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction | Intent / relation / kind recall |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| ja-auth (3) | full | 1 / 1 / 1 | 1 / 1 | 1 | 0 | 1 | 143 | 0.03276059564719359 | 1 / 1 / 1 |
| ja-auth (3) | fts-only | 0.6666666666666666 / 0.6666666666666666 / 0.6666666666666666 | 1 / 0.6666666666666666 | 1 | 0 | 1 | 98 | 0.02245131729667812 | 1 / 0.3333333333333333 / 1 |
| ja-auth (3) | no-relations | 0.6666666666666666 / 0.6666666666666666 / 0.6666666666666666 | 1 / 0.6666666666666666 | 1 | 0 | 1 | 98 | 0.02245131729667812 | 1 / 0.3333333333333333 / 1 |
| ja-jsts (3) | full | 0.3333333333333333 / 0.6666666666666666 / 1 | 1 / 1 | 1 | 0 | 1 | 83 | 0.11559888579387187 | 1 / 1 / 1 |
| ja-jsts (3) | fts-only | 0.3333333333333333 / 0.3333333333333333 / 0.3333333333333333 | 1 / 0.3333333333333333 | 1 | 0 | 1 | 138 | 0.19220055710306408 | 1 / 0 / 1 |
| ja-jsts (3) | no-relations | 0.3333333333333333 / 0.3333333333333333 / 0.3333333333333333 | 1 / 0.3333333333333333 | 1 | 0 | 1 | 138 | 0.19220055710306408 | 1 / 0 / 1 |

The baseline commands also produced `go test ./...` with 263 passing tests,
`go vet ./...` with no issues, and a successful CGO-free native build. Race
coverage and the later cross-build/evaluation matrix are not implied by this
baseline section.

## .NET WinForms/WPF/XAML Task 5 result

The `dotnetsample` fixture was indexed with `focalspan setup` and evaluated
with `go run ./cmd/focalspan-eval --root testdata/repos/dotnetsample
--cases testdata/eval/dotnet-cases.jsonl --json` on 2026-08-30. The public CLI
does not expose the retired `index`/`eval` commands, so the development
evaluator was used after the fixture index was rebuilt. Each of the six cases
was queried twice.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| .NET WinForms/WPF/XAML/RESX | 6 | 0.8333 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 105 | 0.22410147991543342 |

The six cases cover the WPF code-behind handler and binding, XAML resource
dictionary, WinForms designer initializer and load handler, and a ViewModel
validation test. All returned paths existed and no unrelated fixture path was
returned. The fixture also exercises RESX keys, metadata, type/mimetype
references, and binary-value omission; those resource assertions are covered
by the package tests rather than a separate evaluation query.

## Polyglot Coverage v0.3 final acceptance

On 2026-08-30, every regular fixture root was rebuilt immediately before its
evaluation with `extractors-v5-polyglot`. The full per-profile JSON summaries
are recorded in `docs/evaluation-v0.3.json`; the evaluator queried every case
twice. The table below is copied from those current-checkout runs.

| Profile | Cases | hit@1 / hit@3 / hit@5 | Symbol / path recall | Budget | Forbidden | Deterministic | Median tokens | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Go/auth | 6 | 0.6667 / 1.0000 / 1.0000 | 1.0000 / 0.9167 | 1.0000 | 0 | 1.0000 | 172 | 0.04007084348018596 |
| PHP | 4 | 0.2500 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 179 | 0.05550387596899225 |
| Smarty/template | 5 | 0.8000 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 87 | 0.026568430453226165 |
| C/C++ | 8 | 0.5000 / 0.8750 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 94 | 0.15639269406392695 |
| C# | 5 | 0.6000 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 107 | 0.15507246376811595 |
| JavaScript/TypeScript | 6 | 0.6667 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 121 | 0.15419501133786848 |
| .NET WinForms/WPF/XAML/RESX | 6 | 0.8333 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 95 | 0.20833333333333334 |
| Rust | 5 | 0.4000 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 94 | 0.19542619542619544 |
| Python | 5 | 1.0000 / 1.0000 / 1.0000 | 1.0000 / 1.0000 | 1.0000 | 0 | 1.0000 | 78 | 0.22857142857142856 |

