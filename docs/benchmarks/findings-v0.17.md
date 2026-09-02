# Intent別retriever cap / noise制御 v0.17 findings

## Scope and execution

v0.17はquery intentごとにprivateなretriever capを選択し、fusion前の取得量を
抑える候補だった。公開MCP tool、CLI、`focalspan.context.v1`、JSON key、
retriever ID、source fidelity、relation provenance、deterministic orderingは
変更していない。

候補は製品コミット `cff306d` に保存してゲートを測定した。strict gate不合格の
ため通常のrevertコミット `6ba04a9` で製品変更だけを取り消した。v0.15 baseline
（累積wire `12,304`、useful efficiency `0.4064`）を維持し、v0.17のquality
baselineは作成していない。

## Static verification

- RED cap tests: 旧固定capを検出して失敗。
- GREEN targeted tests: 6 passed。
- `go test ./... -count=1`: 702 tests / 46 packages passed。
- `go vet ./...`: passed。
- `gofmt` and `git diff --check`: passed。
- native and CGO-free Windows amd64/Linux amd64/Darwin arm64 builds: passed。
- `go test -race ./...`: **UNVERIFIED** because local MinGW reports
  `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

The dedicated eight-case Evidence fixture retained coverage, role accuracy,
source fidelity, relation validity, budget compliance, and deterministic output
at `1`; forbidden violations and known-handle resend were `0`. The measured
two-step delta ratio was `0.555005305978069`, and median metadata overhead was
`0.344969199178645`.

## Candidate gate status

The historical `focalspan-history-v0.5` suite ran exactly once with the `default`
profile, repeat 1, attribution enabled, and diagnosis enabled. It produced 48
quality rows, 40 attribution results, and 40 diagnosis results. The comparison
with v0.15 returned `compatible=true`, but `regressions=4`:

- `php-extractor-integration` / `full-evidence-focused` / budgets 1024, 2048,
  and 4096: wire `254 -> 289` without required-recall improvement;
- `php-extractor-integration` / `no-relations-evidence-focused` / 2048:
  wire `254 -> 289` without required-recall improvement.

| Metric | v0.15 baseline | v0.17 candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels (focused/2048) | 7 | 7 | no increase (pass) |
| packed labels (all candidate rows) | 5 | 5 | retain all (pass) |
| cumulative estimated wire tokens | 12,304 | 12,459 | `<= 12,304` (fail) |
| useful evidence | 5 | 5 | `>= 5` (pass) |
| useful evidence / 1,000 tokens | 0.4064 | 0.4013 | `>= 0.4064` (fail) |
| median metadata overhead | 0.9222 | 0.8904 | no regression (pass) |
| comparison regressions | 0 | 4 | must be 0 (fail) |

Query-median performance was mixed. Profile-level medians (baseline to
candidate) were: `full-evidence-focused` 32 to 28.5 ms (10.9% improvement),
`fts-evidence-focused` 20.5 to 15 ms (26.8%),
`no-relations-evidence-focused` 30.5 to 23.5 ms (23.0%), and
`full-legacy-source` 123.5 to 98.5 ms (20.2%). The required 20% improvement for
every profile was therefore not met; the full-evidence profile remained below
the threshold and included individual slowdowns.

Because wire, efficiency, quality regression, and the all-profile latency gate
were not simultaneously satisfied, the candidate is rejected. The same
historical benchmark must not be rerun for this candidate.

## Privacy and artifact hashes

The generated reports contained no absolute local paths, usernames, `NaN`, or
`Infinity`. Temporary reports, attribution/diagnosis files, binaries, and local
Go caches are removed after this finding is committed. User-owned `AGENTS.md`,
`.focalspan.json`, and `TASKS.md` changes are not staged.

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `83d5ec42cb6e80b7a46bef50b0623a94892cdf5d1dd414f87f15441113318d33` |
| candidate quality Markdown | `2b57d3428b524df5fb979ee651b74360dd588e3b3827155b0f1fa6eb63470da3` |
| candidate attribution JSON | `77696b5a770d16a8c30681c779da5dc3351f79147da798d4c886f160ccbd66e9` |
| candidate attribution Markdown | `bc1e99786b3c074444e2b34672752f6ce786edff7e66cf6bc41a58694ed2b844` |
| candidate diagnosis JSON | `50ca6b811f306288660ed1f79c425fc3de7629b5e23ce661f538e66042bee51e` |
| candidate diagnosis Markdown | `fe1a50f8279d088e2d7562d2cf9356c5c769410b97d51f88a1c705a57c8c2e2f` |

## Retrospective

Lowering caps reduced query medians for several profiles, but it changed the
selected Evidence economics in the PHP case: the same missing recall became a
35-token wire increase. The next attempt must first prove per-row selection
non-regression (or use an explicit legacy fallback) before spending another
historical benchmark run. SQL batching and schema-v2 relation linking remain
separate future milestones.
