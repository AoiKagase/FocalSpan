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

For every case, `focalspan eval` runs the same query twice and records:

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
focalspan init
focalspan index --root testdata/repos/authsample
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
focalspan index --root testdata/repos/phpsample --quiet
focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
focalspan index --root testdata/repos/cppsample --quiet
focalspan eval --root testdata/repos/cppsample --cases testdata/eval/cpp-cases.jsonl --json
focalspan index --root testdata/repos/csharpsample --quiet
focalspan eval --root testdata/repos/csharpsample --cases testdata/eval/csharp-cases.jsonl --json
focalspan index --root testdata/repos/jstssample --quiet
focalspan eval --root testdata/repos/jstssample --cases testdata/eval/jsts-cases.jsonl --json
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

## Verification status

The current worktree also passed `go test ./...`, `go vet ./...`, and the
fixture CLI flow (`index`, `status --json`, budgeted JSON `query`, quiet
`update`, `impact --json`, `doctor --json`, and a bounded stdio `serve` startup
smoke). `CGO_ENABLED=0` builds passed for Windows amd64, Linux amd64, and
Darwin arm64. `go test -race ./...` remains unverified because this Windows
environment has no `gcc` C compiler (`runtime/cgo: C compiler "gcc" not
found`); this is an environment limitation, not a reported test assertion.

## Interpretation

The fixtures measure retrieval, structural relation use, deduplication, and
packing, not semantic call resolution. The C/C++, C#, and JavaScript/TypeScript
extractors use pure-Go lexers/parsers and bounded recovery. Resolved relations
are limited to unique local qualified/name matches; unresolved calls,
imports/includes/exports, and references are labeled with their lexical target
and confidence. Future semantic providers may improve recall without changing
the packer or output contract.
