# 検索・relation SQL batch化 v0.18 findings

## Scope and execution

v0.18は、`internal/store`のexact / qualified / prefix / path検索とrelation
legacy / provenance queryをordinal付きSQL batchへまとめる候補だった。公開MCP
tool、CLI、`focalspan.context.v1`、JSON key、retriever ID、Evidence source
fidelity、relation provenance、deterministic orderingは変更していない。

候補実装は製品コミット `a15584f` に保存して検証した。採用ゲート不合格のため、
候補コードは通常のrevertコミット `9c8e005` で取り消した。v0.15 baseline
（累積wire `12,304`、useful evidence `5`、効率 `0.4064`）を維持し、v0.18の
quality baselineは作成していない。

## Static and equivalence verification

- RED equivalence tests: 旧逐次実装で失敗を確認。
- GREEN targeted tests and `go test ./... -count=1`: 702 tests / 46 packages passed。
- `go vet ./...`: passed。
- `gofmt` and `git diff --check`: passed。
- native and CGO-free Windows amd64/Linux amd64/Darwin arm64 builds: passed。
- `go test -race ./...`: **UNVERIFIED** because the local MinGW toolchain does
  not support 64-bit mode (`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`).

専用Evidence fixtureではcoverage、role、source fidelity、relation validity、
budget、deterministic outputの不変条件がすべて `1`、forbidden violationと
known-handle resendが `0` だった。公開レスポンス、Evidence wire、relation rowの
同値性にも回帰はなかった。

## Candidate gate status

Historical `focalspan-history-v0.5` suiteは、候補につき1回だけ、`default`
profile、repeat 1、attribution有効、diagnosis有効で実行した。比較結果は
`compatible=true`、`regressions=0` で、quality/wire/packingはbaselineと同値だった。

| Metric | v0.15 baseline | v0.18 candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels (focused/2048) | 7 | 7 | no increase (pass) |
| packed labels | 5 | 5 | retain all (pass) |
| cumulative estimated wire tokens | 12,304 | 12,304 | `<= 12,304` (pass) |
| useful evidence | 5 | 5 | `>= 5` (pass) |
| useful evidence / 1,000 tokens | 0.4064 | 0.4064 | `>= 0.4064` (pass) |
| median metadata overhead | 0.9181 | 0.9181 | no regression (pass) |
| comparison regressions | 0 | 0 | must be 0 (pass) |

Query medianのprofile比較は次のとおりだった。latency採用条件は変更対象の
全profileで20%以上の改善であり、legacy以外は未達だった。

| Profile group | Baseline median | Candidate median | Change | Gate |
|---|---:|---:|---:|---|
| full-evidence-focused | 32 ms | 27.5 ms | -14.1% | fail |
| fts-evidence-focused | 20.5 ms | 17.5 ms | -14.6% | fail |
| no-relations-evidence-focused | 30.5 ms | 30.5 ms | 0.0% | fail |
| full-legacy-source | 123.5 ms | 98.5 ms | -20.2% | pass |

候補はwireとqualityを壊さなかったが、全profile latency gateを満たさないため
棄却した。同じhistorical benchmarkはこの候補について再実行しない。

## Privacy and artifact hashes

生成した6 artifactを、絶対ローカルパス、ユーザー名由来のパス、秘密情報、
private-key markerのパターンでscanした結果、該当なしだった。artifactは
この所見へのhash記録後に削除した。ユーザー所有の`AGENTS.md`、`.focalspan.json`、
`TASKS.md`はstageしない。

| Artifact | SHA-256 |
|---|---|
| candidate quality JSON | `513c8440605a9406cbf1460f1963a77ac8e5b7814f333002b51b823872136962` |
| candidate quality Markdown | `31bc389b3cbf06764389556ab8168fc3c3d4dca2148f5b0e1606928aeee36604` |
| candidate attribution JSON | `425dd767bcdef04e791c31065616a94468be49b4d54d576f82fb9232f7826da2` |
| candidate attribution Markdown | `7b82cf947316c5c6f1aeb32dbe47f773d3d228433af260701ed2cbe948d2567c` |
| candidate diagnosis JSON | `1dea51b6ea2a6593da2771d3aeee7c3393b5484efeb5d1195b58e3923b794287` |
| candidate diagnosis Markdown | `5c7f22e42ea5245d9701ec2d6de43ec73c45b9ddda432967a6ec3a7f14d131bc` |

## Retrospective

SQL batch化は検索・relationの結果を変えず、legacy source profileでは有意な改善を
示した。しかし通常のfull / FTS / no-relations profileでは20%条件に届かず、
候補全体を採用できない。次候補ではprofile別の測定ノイズとDB round-trip比率を
先に分解し、全profile gateを満たせる見込みがない限り実装を保持しない。

schema v2 relation linking、TokenEstimator、検索cap、Evidence packingの変更は
v0.18に混ぜず、別マイルストーンとして扱う。
