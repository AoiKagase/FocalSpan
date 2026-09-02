# FocalSpan v0.18 検索・relation SQL batch化 実装計画

**Goal:** 複数の検索値とrelation anchorを1回のSQLite queryへまとめ、MCP/CLIの
結果、Evidence wire、relation provenanceを変えずにDB round-tripとquery latencyを
削減する。v0.15 baselineのquality/wireを厳密に維持し、v0.17で得たnegative
evidence（候補集合変更によるwire回帰）を再発させない。

**Architecture:** `internal/store`のprivate query builderだけを変更する。入力値や
anchorには決定的なordinalを付けたSQLite `VALUES`/CTEを使い、既存の1値ずつの
append順とtie-breakをSQLのORDER BYで再現する。検索結果の型・fusion・ranking・
Evidence compilerは変更しない。relationのlegacy candidateとprovenance hitは
それぞれ全anchorを1 queryで取得するが、最終sort/dedupとmergeは既存処理を維持する。

**Spec:** ユーザー承認済み「FocalSpan token効率改善候補・優先順位」rank 8
「検索・relation SQLのbatch化」。rank 9 schema v2 relation linking、TokenEstimator、
検索cap再試行、per-row Evidence選択fallbackは別マイルストーンで扱う。

## Purpose / Big Picture

現在のStoreはexact/qualified/prefix/path検索で値ごとにSQLを発行し、relationの
legacy/provenance取得でもanchorごとにSQLを発行する。自然文queryでは同じ処理を
複数回繰り返すため、結果が少ない場合でもSQLite round-tripが増える。値とanchorを
batch化し、候補の集合と順序を同一に保ったまま待ち時間とDB負荷を測定可能な範囲で
削減する。

## Baseline and Gates

- 現行baselineはv0.15（focused/2048 packing dropped `7`、packed labels `5`、
  累積wire `12,304`、useful efficiency `0.4064`）。
- v0.17はintent cap候補をwire回帰で棄却済み。v0.18では同じquality/wire候補を
  再利用・再実行しない。
- historical suiteは候補につき1回だけ、`default` profile、repeat 1、
  attribution/diagnosis有効で実行する。
- `compatible=true`、`regressions=0`、recall/role/fidelity/relation/budget/
  determinism/known resend/MCP契約の非回帰を必須とする。
- candidateのperformance rowsで、変更対象profileのquery medianがbaseline比
  20%以上改善することを採用条件とする。改善なし、quality回帰、または結果byte
  不一致なら候補をrevertしてbaselineを維持する。

## Global Constraints

- 公開MCP tool、CLI、`focalspan.context.v1`、JSON key、retriever ID、SQLite
  schema、`known_handles`を変更しない。
- 検索の候補集合、入力値の優先順、RRF weight、ranking、Evidence packing、
  relation resolution/provenance、source fidelity、deterministic orderingを変えない。
- SQL値はすべてparameter bindingし、入力が空、重複、キャンセル、malformedでも
  既存の戻り値とエラー分類を維持する。
- SQL batchが使えない条件では既存の逐次経路へ安全にfallbackする。SQLiteの
  parameter上限を越える巨大入力を無制限に1 queryへ連結しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`は変更・stageしない。
- benchmark reports、binaries、repository-local cachesはゲート後に削除する。

## Context and Orientation

- `internal/store/store.go`の`SearchExactSymbols`、`SearchQualifiedSymbols`、
  `SearchSymbolPrefixes`、`SearchPaths`はlookup値ごとに`QueryContext`を呼ぶ。
- 同ファイルの`RelatedCandidates`と`RelatedCandidateHits`はanchorごとにlegacy/
  provenance queryを呼び、`RelatedCandidateHits`は最後に決定的sort/dedupする。
- `internal/store/store_test.go`と`internal/store/relation_test.go`は検索順序、
  unresolved relation、PHP/Rust/Python/module/path semanticsを固定する既存fixture。
- `internal/search/retrieval.go`は両relation経路をmergeするため、Storeの返却内容
  と順序を変えずにround-tripだけを減らす必要がある。
- `internal/benchmark`のperformance rowsとcompareはquality比較とは別にquery
  medianを出力する。

## Plan of Work

### Task 0: Freeze transition

- [x] v0.17 root planを`docs/superpowers/plans/completed/2026-09-02-v0.17-intent-retriever-cap.md`へbyte-identicalにアーカイブする。
- [x] archiveとroot v0.18 planだけをdocumentation-only transition commitにする。
  ユーザー所有dirty filesはstageしない。

### Task 1: RED tests for batch equivalence

**Files:** `internal/store/store_test.go`、`internal/store/relation_test.go`、必要に
応じてprivate test helper。

- [ ] 複数値のexact/qualified/prefix/path結果が、同じ値を逐次投入した既存経路と
  handle、順位、limit、dedupを完全一致することを固定する。
- [ ] 複数anchorのlegacy relation candidatesとprovenance hitsで、direction、
  resolved、confidence、source、anchor handle、最終sort/dedupが完全一致することを
  固定する。
- [ ] empty/duplicate values、unknown relation、missing anchor、cancelled context、
  SQLite parameter-size fallbackの結果とエラーを固定する。
- [ ] query round-trip削減を検証できるprivate計測フックまたはSQLite trace testを
  追加する。ただし公開出力や通常traceへdebug fieldを追加しない。

### Task 2: Minimal GREEN implementation

**Files:** `internal/store/store.go`および必要最小限のprivate helper。

- [ ] lookup値をordinal付きparameter CTEへまとめ、各検索メソッドの既存条件、
  per-value fallback、append順、limit clamp、エラー wrappingを保ったまま1 query
  で処理する。
- [ ] relation anchorをparameter CTEへまとめ、legacy/provenanceの各SQLをbatch化
  する。caller/testのcamel-case patternはanchorとの対応を失わず、既存の unresolved
  match semanticsを保持する。
- [ ] batch結果は既存scan型へ変換し、最終sort/dedupと`mergeRelationCandidates`の
  入力契約を変更しない。SQL error/cancellation時はconnectionを確実にcloseする。
- [ ] parameter上限または空入力では既存逐次経路へ戻し、DB schemaや公開interfaceを
  変更しない。

### Task 3: Static and equivalence verification

- [ ] gofmt、`git diff --check`。
- [ ] repository-local cacheで`go test ./... -count=1`、`go vet ./...`。
- [ ] native、CGO-free Windows amd64/Linux amd64/Darwin arm64 buildを一時出力し
  削除する。
- [ ] `go test -race ./...`を実行し、MinGW制約ならUNVERIFIEDと記録する。
- [ ] fixture Evidence evaluatorでcoverage/role/fidelity/relation/budget/
  determinism、known resend、delta ratio、metadata overheadを再測定する。
- [ ] batch前後の代表fixtureでserialized result bytesとrelation rowsが完全一致する
  ことを確認する。

### Task 4: Candidate benchmark gate

- [ ] historical `focalspan-history-v0.5`を候補につき1回だけ実行し、v0.15 baselineと
  比較する。同じ候補の再実行は禁止する。
- [ ] focused/2048のpacking dropped非増加、packed labels `>=5`、wire `<=12,304`、
  useful efficiency `>=0.4064`を要求する。
- [ ] comparisonの`compatible=true`、`regressions=0`、quality/invariant非回帰を
  要求する。
- [ ] 対象profileのquery medianが20%以上改善し、非対象profileに20%超のslowdownが
  ないことを確認する。
- [ ] privacy scan、artifact hashes、実測値をsource-free
  `docs/benchmarks/findings-v0.18.md`へ記録する。未達ならbaselineを作らない。

### Task 5: Closure and recovery

- [ ] 全ゲート合格時だけ製品commit、findings、`results-v0.18.{json,md}`を確定する。
- [ ] gate不合格時は製品commitだけをreverse-order `git revert`し、findingsを残し、
  v0.15 baselineを維持する。失敗候補のhistorical benchmarkは再実行しない。
- [ ] generated reports、binaries、cachesを削除し、ユーザー所有dirty filesを保持する。
- [ ] Progress、Discoveries、Decision Log、Outcomesを実測値で更新し、v0.17 archiveは
  編集しない。

## Validation and Acceptance

同一入力に対しbatch経路と逐次経路のserialized candidate/result bytesが同一で、
relation rowsの全field、source provenance、sort/dedup、error/cancellation semanticsが
同一であること。公開MCP/CLI/Evidence契約、wire budget、known_handles、deterministic
orderingが非回帰で、historical比較が`compatible=true`/`regressions=0`となり、変更対象
profileのquery medianが20%以上改善することをすべて満たす。

## Idempotence and Recovery

batch builderはpureなparameter/SQL生成でindex/databaseを変更しない。同じ入力は同じ
ordinal、SQL bind順、sort結果になる。parameter上限やunsupported relationでは逐次経路
へ戻る。candidate gate不合格時は製品commitだけをordinary revertし、source-free findings
とbaselineを保持する。

## Interfaces and Dependencies

公開interfaceとschemaは変更しない。`code_context`、`code_expand`、`code_impact`、
CLI evidence、`focalspan.context.v1`、legacy `ContextBundle`を維持する。変更対象は
`internal/store`のprivate SQL batchingと回帰テストで、benchmark toolingは計測用途だけ
に限定する。

## Progress

- [x] 2026-09-02T11:15Z: v0.17 negative planをbyte-identical archiveへ保存した。
- [x] 2026-09-02T11:15Z: v0.18 documentation-only transition planを作成した。
- [x] RED equivalence tests。
- [x] GREEN SQL batching implementation（候補 `a15584f` を実装後、ゲート不合格で `9c8e005` にrevert）。
- [x] Static/equivalence verification。
- [x] Candidate benchmark gate（quality/wireは合格、全profile latency gateは不合格）。
- [x] Closure and recovery（baselineを維持し、候補artifactを削除）。

実行記録（UTC）:

- 2026-09-02T12:44Z: RED/GREEN、全体テスト（702 tests / 46 packages）、vet、native/CGO-free build、fixture不変条件を確認した。raceはMinGW制約でUNVERIFIED。
- 2026-09-02T12:59Z: historical suiteを候補につき1回実行。`compatible=true`、`regressions=0`、wire/efficiency/packing同値、latency gateはlegacyのみ合格。
- 2026-09-02T13:01Z: `a15584f` の候補実装を保存し、直後に `9c8e005` でrevertした。
- 2026-09-02T13:04Z: source-free findingsとartifact hashを記録し、候補artifactの削除準備を完了した。

## Surprises & Discoveries

- SQL batch化はcandidateと逐次baselineで結果byte、relation row、Evidence契約を一致させられた。
- 2026-09-02のhistorical benchmarkでは`compatible=true`、`regressions=0`、wire/efficiency/packing同値だった。
- latency改善はfull `14.1%`、FTS `14.6%`、no-relations `0.0%`、legacy `20.2%`で、全profile 20% gateを満たさなかった。
- 候補benchmark reportのquality rowsはpath/symbol recallを改善せず、batch化は取得品質を変えないことを確認した。

## Decision Log

- 2026-09-02: v0.17の次順位としてrank 8 SQL batch化を開始する。
- 2026-09-02: schema v2 relation linkingとTokenEstimator変更は同時実装しない。
- 2026-09-02: SQL batch化は検索・relationのround-trip削減に限定し、ranking/packing/wire
  を調整しない。
- 2026-09-02: 結果同値でも全profile latency 20% gateを満たさない候補は採用せず、
  `a15584f`を`9c8e005`でrevertしてv0.15 baselineを維持する。
- 2026-09-02: v0.18のhistorical suiteは候補につき1回のみ実行し、同一候補を再実行しない。

## Outcomes & Retrospective

v0.18は棄却で完了した。検索・relation batch候補の結果完全一致とstatic verification
は確認できたが、通常profileのlatency gate未達により製品コードはbaselineへ戻した。
実測値、privacy scan、artifact hashは`docs/benchmarks/findings-v0.18.md`へ記録した。
