# FocalSpan v0.17 intent別retriever cap / noise制御 実装計画

**Goal:** query intentに応じて内部retrieverの取得上限を適応させ、fusion前の
ノイズと処理量を減らす。公開MCP/CLI、`focalspan.context.v1`、retriever ID、
source fidelity、relation provenance、deterministic orderingを維持し、v0.15
baselineのwire/labelを非回帰にする。

**Architecture:** `internal/search.RetrieverSet.Retrieve`だけでprivateなcap
profileを選択する。各store queryへ渡すlimitをplan intentとretrieval modeから
決定し、fusion、ranking、Evidence compiler、公開trace schemaは変更しない。
capは既存上限以下に限定し、path/FTS fallbackを消さない。relation anchorの
取得はbase retrieverのcap適用後に従来どおり実行する。

**Spec:** ユーザー承認済み「FocalSpan token効率改善候補・優先順位」rank 7
「intent別retriever cap / noise制御」。rank 8のSQL batch化、schema v2 relation
linking、TokenEstimator変更は別マイルストーンであり、この計画では扱わない。

## Purpose / Big Picture

現在はqualified/exact/prefix/FTS/path/relationを固定上限（50/50/50/100/50/100）
で取得し、最大400件をfusionする。definitionや軽量relation queryでは、上位候補
に寄与しないprefix/FTS/path行がfusion対象を膨らませる可能性がある。intent別の
保守的なcapでSQL結果とfusion入力を狭め、同一queryの候補品質を変えずに内部処理
量を削減する。

## Baseline and Gates

- v0.15 baseline: focused/2048 `packing_dropped=7`、packed labels `5`、累積wire
  `12,304`、useful efficiency `0.4064`、median metadata overhead `0.9222`。
- historical suiteは候補につき1回だけ、`default` profile、repeat 1、attribution/
  diagnosis有効で実行する。
- `compatible=true`、`regressions=0`、wire/label/recall/fidelity/relation/
  budget/determinism/known resend/MCP契約の非回帰を必須とする。
- query latencyは候補のperformance rowsで計測し、profileごとのquery medianが
  baseline比20%以上改善した場合だけ採用する。改善がない、またはqualityが
  回帰した場合は候補をrevertし、v0.15を維持する。

## Global Constraints

- 公開MCP tool、CLI、`focalspan.context.v1`、JSON key、retriever ID、SQLite
  schema、`known_handles`を変更しない。
- Search結果の順序、RRF weight、ranking、relation resolution、source fidelity、
  Evidence packingを変更しない。
- cap profileは既存固定cap以下。FTS-onlyはFTSだけ、no-relationsはrelationなし、
  relation intentはanchor fallbackを従来どおり保持する。
- traceは既存のretriever countを反映するだけで、新しいdebug/token fieldを追加しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`は変更・stageしない。
- benchmark reports、binaries、repository-local cachesはゲート後に削除する。

## Context and Orientation

- `internal/search/retrieval.go`に固定capと`RetrieverSet.Retrieve`がある。
- `internal/search/retrieval_test.go`のrecording storeはretriever呼び出し順と
  relation anchorを検証する。limit記録を追加してcapを検証する。
- `internal/search/search.go`はretrieval結果を最大400件でfusionし、`Limit`は
  公開結果件数でありretriever capとは別である。
- `internal/search/fusion.go`のRRF weightとsort tie-breakは凍結する。
- `internal/benchmark`のperformance rowsとcompareがquery latency警告を出す。

## Plan of Work

### Task 0: Freeze transition

- [ ] v0.16 root planを`docs/superpowers/plans/completed/2026-09-02-v0.16-guidance-shared-budget.md`へbyte-identicalにアーカイブする。
- [ ] archiveとroot v0.17 planだけをdocumentation-only transition commitにする。
  ユーザー所有dirty filesはstageしない。

### Task 1: RED tests for intent caps

**Files:** `internal/search/retrieval_test.go`、必要に応じて`internal/search/search_test.go`。

- [ ] definition/callers/callees/tests/imports/exports/references/impact/defaultの
  cap profileを固定する。
- [ ] FTS-only/no-relations/full modeの呼び出し集合、limit、relation anchor、
  empty-term fallbackを固定する。
- [ ] cap適用後も結果の決定順序、trace count、relation provenanceが変わらない
  fake-store回帰を追加する。

### Task 2: Minimal GREEN implementation

**Files:** `internal/search/retrieval.go`と必要最小限のprivate tests。

- [ ] private `retrieverCaps`とintent/mode別選択関数を追加する。
- [ ] 各store queryへ既存cap以下のlimitを渡し、取得リストの型・順序・エラー
  wrappingを維持する。
- [ ] relation取得はcap済みbase listsからanchorを作り、legacy/provenance merge
  と既存relation capを維持する。
- [ ] cap 0や未知intentでは安全な既存capへfallbackし、公開traceに設定値を出さない。

### Task 3: Static verification

- [ ] gofmt、`git diff --check`。
- [ ] repository-local cacheで`go test ./... -count=1`、`go vet ./...`。
- [ ] native、CGO-free Windows amd64/Linux amd64/Darwin arm64 buildを一時出力し
  削除する。
- [ ] `go test -race ./...`を実行し、MinGW制約ならUNVERIFIEDと記録する。
- [ ] fixture evaluatorでcoverage/role/fidelity/relation/budget/determinism、
  known resend、delta ratioを再測定する。

### Task 4: Candidate benchmark gate

- [ ] historical `focalspan-history-v0.5`を候補につき1回実行し、v0.15 baselineと
  比較する。
- [ ] focused/2048のpacking dropped非増加、packed labels 5以上、wire `<=12,304`、
  useful efficiency `>=0.4064`を要求する。
- [ ] quality/invariant comparisonの`compatible=true`、`regressions=0`、および
  profileごとのquery latency 20%以上改善を要求する。
- [ ] privacy scan、artifact hashes、実測値をsource-free
  `docs/benchmarks/findings-v0.17.md`へ記録する。未達ならbaselineを作らない。

### Task 5: Closure and recovery

- [ ] 全ゲート合格時だけ製品commit、findings、`results-v0.17.{json,md}`を確定する。
- [ ] gate不合格時は製品commitだけをreverse-order `git revert`し、findingsを残し、
  v0.15 baselineを維持する。同じ候補benchmarkは再実行しない。
- [ ] generated reportsとcandidate cacheを削除し、ユーザー所有dirty filesを保持する。
- [ ] Progress、Discoveries、Decision Log、Outcomesを実測値で更新し、v0.16 archiveは
  編集しない。

## Validation and Acceptance

同一入力は決定的で、検索候補の順序とRRF scoreが既存と同一であること。cap適用後も
必要path/symbol/anchorのrecall、relation validity、source fidelity、budget、
known resend、公開trace/MCP契約が非回帰であること。historical比較は
`compatible=true`、`regressions=0`、wire/efficiency gate合格、かつquery median
20%以上改善をすべて満たすこと。

## Idempotence and Recovery

cap選択はpureなin-memory関数でindex/databaseを変更しない。limitは呼び出しごとに
再計算され、同じplan/modeで同じ値になる。candidate gate不合格時は製品commitだけを
ordinary revertし、v0.15 baselineとsource-free findingsを保持する。

## Interfaces and Dependencies

公開interfaceは変更しない。`code_context`、`code_expand`、`code_impact`、CLI
evidence、`focalspan.context.v1`、legacy `ContextBundle`を維持する。変更は
`internal/search`のprivate cap profileと回帰テストに限定する。

## Progress

- [x] 2026-09-02T10:37Z: v0.16 negative candidateを確定し、v0.15 baselineを維持した。
- [x] 2026-09-02T10:37Z: v0.16 planをbyte-identical archiveへコピーした。
- [ ] v0.17 documentation-only transition commit。
- [ ] RED tests。
- [ ] GREEN implementation。
- [ ] Static verification。
- [ ] Candidate benchmark gate。
- [ ] Closure and recovery。

## Surprises & Discoveries

- v0.16ではaggregate wire改善だけでは不十分で、MCP 5行のper-row wire回帰により
  strict gateを失敗した。v0.17もquality非回帰をwire/latencyと同列に扱う。
- 固定capは`retrieval.go`に集中しており、intent別profileをprivateに追加できる。
  ただしbase listを狭めるとrelation anchorやfallbackを失うため、REDで呼び出し
  limitとanchor経路を先に固定する。

## Decision Log

- 2026-09-02: 次順位としてrank 7 intent別retriever cap / noise制御を開始する。
- 2026-09-02: SQL batch化とschema v2 relation linkingは別系統として同時実装しない。
- 2026-09-02: capは既存上限以下に限定し、公開trace/schema/RRF weightは変更しない。

## Outcomes & Retrospective

未完了。cap profileの候補値、quality/latency実測、採用または棄却の根拠を完了時に
追記する。
