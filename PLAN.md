# FocalSpan v0.19 relation linking schema v2 実装計画

**Goal:** 導出SQLite indexへlookup projectionを追加し、relation linkingの候補探索と
書き込みを変更差分単位・単一transactionへ縮約する。現在のrelation解決優先順位、
曖昧候補を未解決のまま残す規則、source attribution、公開MCP/CLI/Evidence契約を
変更せず、`TASKS.md`に記録された現行規模の更新時間を削減する。

**Architecture:** schema v2はschema v1の派生cacheをin-place migrationせず、許可された
`setup`/`update`/明示auto-updateだけがtemporary sibling DBへ完全再構築する。正常な
status、検索、MCP startupは既存v1を変更せず、schema diagnosticを返す。v2には
`symbol_lookup`、`file_lookup`、`relation_lookup` projectionを持たせ、projectionは
候補集合を狭めるsuperset filterとしてだけ使う。最終判定は既存Go resolverを同じ
priorityで実行し、成功リンクは1 transactionでbatch更新する。

**Spec:** `TASKS.md`「Rebuild Relation Linking Around Schema v2」。これはtoken wire
改善候補とは別系統の更新性能マイルストーンであり、検索ranking、Evidence packing、
MCP schema、`known_handles`、public JSON keyは変更しない。

## Purpose / Big Picture

現行linkerは全symbolと全relationを読み、各未解決relationについて全symbolを走査し、
一意解決ごとにautocommitの`LinkRelation`を発行する。現行規模では約21,676件の未解決
relationと5,144 symbolsの組み合わせを許し、更新のwriting後に長い遅延が発生する。
v2は依存lookup keyで候補を先に絞り、変更ファイルに関係するrelationだけを再評価し、
書き込みを一括transactionにする。未変更updateではlinkingを完全にskipする。

## Baseline and Gates

- v0.15 token/wire baseline（focused/2048 packing dropped `7`、packed labels `5`、
  cumulative wire `12,304`、useful efficiency `0.4064`）を保持する。
- relation rows、unresolved count、priority、path/manifest semantics、Evidence wireは
  v1とserialized byte単位で同一であること。
- unchanged updateは `<=250ms` かつ現行比 `>=10x`、small related changeは `<=1s` かつ
  `>=5x`、full linkingは `<=5s` かつ `>=2x` を専用benchmarkで測定する。wall-clockは
  ordinary unit testの合否には使わず、決定的な比較結果として記録する。
- cancellation、SQL error、busy swap、integrity failureではv1を保持し、partial v2を
  公開しない。

## Global Constraints

- 公開MCP tool、CLI flag、`focalspan.context.v1`、legacy `ContextBundle`、JSON key、
  `known_handles`、search ranking、Evidence packingを変更しない。
- projectionは既存resolverが検査する候補のsupersetであり、候補を隠す近似検索にしない。
- `zero or multiple candidates => unresolved`、path alias、PHP PSR、Rust/Python module、
  manifest factの優先順とsource attributionを維持する。
- v1検出時、normal `store.Open` / status / search / MCP startupはDBを変更しない。
  自動rebuildはsetup、update、明示auto-update entry pointだけで行う。
- temporary DBは同じrepository内のsibling pathに作り、検証後だけ短い停止中にswapする。
  active userでrenameがbusyならretryable errorを返し、v1を削除・変更しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`、既存completed archiveは変更・stageしない。

## Context and Orientation

- `internal/store/store.go` がmigration、file/chunk/symbol/relationの更新、statusを担当する。
- `internal/store/migrations/001_initial.sql` が現行schema v1で、`meta.schema_version`は`1`。
- `internal/linker/linker.go` の`Link`と`resolveCandidates`が現在の全走査と優先順位を担当する。
- `internal/indexer/indexer.go` は`ApplyIndex`後にlinkerを呼び、completion/durationをlinking
  前に確定している。
- `internal/app/service.go` と `internal/cli/run.go` がsetup/update/status/query/MCPの
  Store open経路を共有する。
- `internal/store/*_test.go`、`internal/linker/*_test.go`、`internal/indexer/*_test.go`に
  relation semantics、progress、rollbackの既存fixtureがある。

## Plan of Work

### Task 0: Freeze transition

- [x] v0.18の最終checked PLANを`docs/superpowers/plans/completed/2026-09-02-v0.18-sql-batching.md`へbyte-identicalに保存する。
- [x] schema v2 root planとarchiveだけをdocumentation transition commitにする。
- [x] transition後は本PLANを唯一のactive ExecPlanとして使用する。

### Task 1: RED tests for schema safety and linker equivalence

**Files:** `internal/store/*_test.go`、`internal/linker/*_test.go`、
`internal/indexer/indexer_test.go`、`internal/app/service_test.go`、`internal/cli/*_test.go`。

- [ ] schema v1を検出したnormal open/status/search/MCP startupがDBを変更せず、typed
  schema diagnosticを返すことを固定する。
- [ ] allowed setup/update openだけがv2 temporary rebuildを開始し、swap前はv1 DBと
  sidecarを保持することを固定する。
- [ ] projection候補で既存fixtureの全relation row、handle、unresolved target、kind、
  confidence、source、ambiguityがv1 linkerと完全一致することを固定する。
- [ ] unchanged update、related/unrelated file change、deletion、manifest change、full
  rebuildで再評価relation集合とwrite transaction数を計測するprivate hookを追加する。
- [ ] cancellation、injected SQL error、integrity failure、busy swap、interrupted swap
  recoveryでv1保持とretry semanticsを固定する。

### Task 2: Schema v2 projection and guarded open

**Files:** `internal/store/migrations/002_schema_v2.sql`、`internal/store/store.go`、
private schema/open helpers、必要最小限の`internal/app/service.go`。

- [ ] `symbol_lookup`（normalized name/qualified key）、`file_lookup`（slash-normalized
  path、extensionless path、basename、language/module variants）、`relation_lookup`
  （unresolved target、origin file、dependency keys）をschema v2として作成する。
- [ ] schema versionをv2へ更新し、unknown/future versionは既存のunsupported error分類
  を維持する。
- [ ] normal openはv1を検出したらread-only diagnosticで停止し、migrationやrebuildを
  開始しない。update-only openはtemporary sibling DBへ新schemaを作る。
- [ ] temporary DBのintegrity check、schema version、required indexesを検証し、失敗時は
  v1をそのまま残してtemporary filesを掃除する。

### Task 3: Projection maintenance and dependency selection

**Files:** `internal/store/store.go`、`internal/store/links.go`、schema helper、
`internal/linker/linker.go`。

- [ ] file/symbol/relation insert/deleteと同じtransaction内でlookup projectionを更新し、
  normalized key生成をdeterministic pure helperにする。
- [ ] `ApplyIndex`がchanged file pathとlookup-key setをprivate resultとして返せるようにし、
  unchanged updateではゼロ candidate-resolutionを選択する。
- [ ] changed file起点relation、dependency key intersect relation、manifest fact変更時の
  unresolved全件、full rebuild全件だけを再評価対象にする。unrelated changeは触らない。
- [ ] projectionがsuperset filterであることをfixtureごとにassertし、legacy resolverの
  path/manifest handlingをGo側で継続する。

### Task 4: Batch relation linking and progress timing

**Files:** `internal/linker/linker.go`、`internal/store/links.go`、`internal/indexer/indexer.go`。

- [ ] Linkerは選択されたrelationだけを既存`resolveCandidates`へ渡し、成功リンクを
  private batch APIの単一transactionで更新する。0件または複数件は未解決のまま残す。
- [ ] batch cancellation/SQL failureは全更新をrollbackし、部分的なリンクを残さない。
- [ ] progress phaseに`linking n/m`を追加し、`IndexRun.CompletedAt`と`DurationMS`は
  linking成功後に確定する。公開CLIの既存phase文字列互換を壊さない。
- [ ] facts変更、extractor-version rebuild、full rebuildのトリガーを既存挙動と一致させる。

### Task 5: Atomic replacement and recovery

**Files:** `internal/store/store.go`、`internal/app/service.go`、`internal/cli/run.go`、
必要なprivate recovery helper。

- [ ] v2 temporary DBをcloseしてからatomic renameし、active userのbusyをretryable error
  として返す。swap前のv1を保持する。
- [ ] interrupted swapを次のallowed updateでdeterministically recoverし、成功後のみ
  旧generated DBとtemporary sidecarを削除する。
- [ ] normal startup/status/search/MCPはswapもcleanupも実行せず、schema diagnosticを
  stderr/health diagnosticsへ伝える。CLI stdout/MCP stdout protocolを汚さない。

### Task 6: Static, semantic, and structural verification

- [ ] failing RED testsを先に確認し、gofmt、`git diff --check`を通す。
- [ ] repository-local cacheで`go test ./... -count=1`、`go vet ./...`、`go test -race ./...`。
  raceがMinGW制約ならUNVERIFIEDとして記録する。
- [ ] native、CGO-free Windows amd64/Linux amd64/Darwin arm64 buildを一時出力し削除する。
- [ ] v1/v2代表fixtureのserialized relation rows、status diagnostics、progress event、
  rollback、MCP/CLI public outputを比較し、token/wire invariantsを再測定する。

### Task 7: Current-scale performance benchmark gate

- [ ] 約5,000 symbols、450 files、28,000 relations、21,000 unresolved relationsの決定的
  fixtureを使い、unchanged / small related / full rebuildを各1回測定する。
- [ ] 250ms/1秒/5秒、10x/5x/2xの全gate、candidate comparisonのrelation完全一致、
  transaction/candidate countの記録を満たす。timing-sensitive testはordinary suiteへ
  入れない。
- [ ] privacy scan、artifact hash、実測値をsource-free
  `docs/benchmarks/findings-v0.19.md`へ記録する。

### Task 8: Closure and recovery

- [ ] 全gate合格時のみschema v2製品commitと結果artifactを確定する。
- [ ] gate不合格時は候補製品commitだけをreverse-order revertし、findingsを残してv1
  baselineを維持する。同じbenchmarkは候補につき再実行しない。
- [ ] generated DB、temporary sidecar、binaries、cachesを削除し、ユーザー所有dirty files
  を保持する。
- [ ] Progress、Discoveries、Decision Log、Outcomesを実測UTC値で更新し、archiveは編集しない。

## Validation and Acceptance

同一fixture・同一入力に対しv1 linkerとv2 linkerのrelation rows、source provenance、
unresolved/ambiguous semantics、sort/order、status、progress、MCP/CLI outputが一致する。
normal openはv1を変更せずdiagnosticを返し、allowed updateだけがatomicにv2へ置換する。
unchanged/small/fullの性能gate、rollback、interrupted recovery、token/wire invariantsを
すべて満たした場合だけ採用する。

## Idempotence and Recovery

projection生成とdependency selectionは同じ入力から同じnormalized keysとrelation集合を
作る。v1/v2 rebuildはtemporary sibling DBへ隔離し、swap前に公開DBを変更しない。失敗時は
temporary DBを削除しv1を再利用できる。swap中断はmarkerとpathの状態から次のallowed
updateで安全に再開する。candidate gate不合格時は製品コードだけをrevertし、baselineと
findingsを保持する。

## Interfaces and Dependencies

公開MCP/CLI interface、Evidence source fidelity、`focalspan.context.v1`、legacy
`ContextBundle`、`known_handles`は維持する。内部依存はSQLite、既存extractor、既存Go
linker/resolver、既存projectmeta factsだけとし、network、外部LLM、repository code実行、
新runtime serviceは導入しない。

## Progress

- [x] 2026-09-02T13:20Z: v0.18最終PLANをcompleted archiveへ保存し、schema v2設計をactive PLANへ切り替えた。
- [ ] RED schema-safety/equivalence tests。
- [ ] schema v2 projection and guarded open。
- [ ] dependency selection and batch linking。
- [ ] atomic replacement/recovery。
- [ ] static/semantic verification。
- [ ] current-scale benchmark gate。
- [ ] closure and recovery。

## Surprises & Discoveries

- v0.18 SQL batch化は結果同値だったが通常profileのlatency gate未達で棄却された。v0.19はDB更新性能に限定し、token wireと検索rankingを調整しない。
- 現行`Indexer.RunWithProgress`は`writing`前にdurationを確定し、linkerが全relationを走査しているため、progress/duration修正はschema v2と同時に検証する必要がある。

## Decision Log

- 2026-09-02: v0.18棄却後の次候補として、`TASKS.md`承認済みschema v2 relation linkingを開始する。
- 2026-09-02: schema v1はin-place migrationせず、setup/update/明示auto-updateだけがtemporary DBをatomic swapする。
- 2026-09-02: projectionはsuperset filter、最終relation判定は既存Go resolver、成功更新はsingle transactionとする。
- 2026-09-02: public MCP/CLI/Evidence、token wire、ranking、known_handlesは変更しない。

## Outcomes & Retrospective

未完了。v1/v2 relation完全一致、atomic replacement/recovery、current-scale性能gateの実測を
完了時に追記する。
