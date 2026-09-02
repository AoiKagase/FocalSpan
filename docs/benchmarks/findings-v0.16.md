# Guidance共同budget化 v0.16 findings

## Scope and execution

v0.16はEvidence候補のselection trialへlimitations/next guidanceのwireを
含め、同一hard budget内で候補とguidanceを共同選択する試行だった。公開MCP
tool、CLI、`focalspan.context.v1`、JSON key、`known_handles`、source fidelity、
relation endpoint、deterministic orderingは変更していない。

候補実装は製品コミット `f940b84` に保存した後、厳格ゲート不合格のため
通常revertコミット `7b24f55` で取り消した。v0.15 baselineは維持し、v0.16
のquality baselineは作成していない。

Static verification:

- `go test ./... -count=1`: passed (696 tests, 46 packages) after revert;
- `go vet ./...`: passed;
- `gofmt` and `git diff --check`: passed;
- native and CGO-free Windows amd64/Linux amd64/Darwin arm64 builds: passed
  during candidate verification; and
- `go test -race ./...`: **UNVERIFIED** because local MinGW reports
  `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

The historical `focalspan-history-v0.5` suite ran exactly once with the
`default` profile, repeat 1, attribution enabled, and diagnosis enabled. It
completed with 48 quality rows, 40 attribution results, and 40 diagnosis
results. The comparison with v0.15 returned `compatible=true`, but reported
five wire regressions.

## Candidate gate status

| Metric | v0.15 baseline | v0.16 candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels (focused/2048) | 7 | 7 | no increase (pass) |
| packed labels (all candidate rows) | 5 | 5 | retain all (pass) |
| cumulative estimated wire tokens | 12,304 | 12,240 | `<= 12,304` (pass) |
| useful evidence | 5 | 5 | `>= 5` (pass) |
| useful evidence / 1,000 tokens | 0.4064 | 0.4085 | `>= 0.4064` (pass) |
| median metadata overhead | 0.9222 | 0.9222 | no regression (pass) |
| comparison regressions | 0 | 5 | must be 0 (fail) |

The five regressions were all `mcp-evidence-output` at the three focused
profiles/budgets listed below. Required recall and invariant scores did not
improve, while wire increased from 251 to 286 tokens:

- `full-evidence-focused` budgets 1024, 2048, and 4096;
- `fts-evidence-focused` budget 2048; and
- `no-relations-evidence-focused` budget 2048.

The comparison also reported two improvements: `php-extractor-integration`
`fts-evidence-focused` 2048 (`288 -> 224`) and all five
`dotnet-structural-registry` focused rows (`679 -> 644`). Despite the aggregate
wire reduction and efficiency increase, the per-row wire regressions violate
the non-regression gate, so the candidate is rejected.

The fixture contract and attribution/diagnosis privacy checks remained clean;
no absolute local paths, usernames, source/content sentinels, secrets, `NaN`,
or `Infinity` were present in the generated reports. Attribution and diagnosis
hashes were unchanged from the accepted v0.15 run.

## Artifact hashes

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `a55e2f365dc337b71c7d7db3ad84f6c6aa6c63bb16851b0b2aec24557040a526` |
| candidate quality Markdown | `6666a7af7c6f8f3a4d1f7aef5b3bc8e97d2a0d8afbb81998205c169828ed11a1` |
| candidate attribution JSON | `425dd767bcdef04e791c31065616a94468be49b4d54d576f82fb9232f7826da2` |
| candidate attribution Markdown | `7b82cf947316c5c6f1aeb32dbe47f773d3d228433af260701ed2cbe948d2567c` |
| candidate diagnosis JSON | `1dea51b6ea2a6593da2771d3aeee7c3393b5484efeb5d1195b58e3923b794287` |
| candidate diagnosis Markdown | `5c7f22e42ea5245d9701ec2d6de43ec73c45b9ddda432967a6ec3a7f14d131bc` |

The quality Markdown hash above is source-free benchmark evidence from the
single candidate run; the temporary report directory is removed after this
finding is committed.

## Retrospective

Counting guidance during trial changed variant economics: for the MCP case it
selected a verbatim implementation where the v0.15 selection used a compact
representation. That preserved recall and invariants but added 35 wire tokens
to each affected row. The candidate needs an explicit legacy-selection fallback
or equivalent per-row non-regression guard before any future benchmark run.
