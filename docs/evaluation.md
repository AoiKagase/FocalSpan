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
| hit@1 / hit@3 / hit@5 | 0.25 / 0.75 / 1.00 |
| Symbol recall / path recall | 1.00 / 1.00 |
| Budget compliance | 1.00 |
| Forbidden path violations | 0 |
| Deterministic result | 1.00 |
| Median estimated tokens | 1129 |
| Median reduction ratio | 0.1699 |

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
| Median estimated tokens | 834 |
| Median reduction ratio | 0.20 |

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
| JavaScript/TypeScript | 6 | 0.83 / 0.83 / 1.00 | 1.00 / 1.00 | 1.00 | 0 | 1.00 | 145 | 0.2020 |

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
