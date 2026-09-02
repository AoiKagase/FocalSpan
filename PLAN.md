# FocalSpan v0.14 v1互換 metadata field pruning 実装計画

> **For agentic workers:** REDテストを先に追加し、最小実装後に静的検証と候補ベンチマークを行う。

**Goal:** `focalspan.context.v1` の公開キーと意味を変更せず、冗長または低価値なEvidenceメタデータを条件付きで省略し、wire tokensを削減する。

**Architecture:** 検索・ranking・候補packingは変更しない。通常の候補選択は現行メタデータで行い、最終Packetを組み立てた後だけ、privateなpruning規則を適用する。必須のhandle、role、location、symbol、fidelity、source/segments、relation、budget、known_handlesの挙動は保持する。JSON `omitempty` で既に任意の `language`、`kind`、`symbol`、`why` を、意味を損なわない範囲で省略する。

**Tech Stack:** Go、`internal/evidence` compiler/model/wire tests、既存の `budget.TokenEstimator`、source-free historical benchmark。

**Spec:** ユーザー承認済み「FocalSpan token効率改善候補・優先順位」rank 4「v1互換のmetadata field pruning」。

## Purpose / Big Picture

現在のEvidence itemは、sourceやsegmentsに加えてlanguage/kind/symbol/whyを持つ。これらは全て任意JSONフィールドで、特にrelation候補の繰り返しlanguage、補助的なkind、`path_match`/`lexical_match`/`same_file`などのwhyは、同一Packet内での有用性が低い一方、metadata overheadの中央値が約0.924ある。最終出力だけをpruneすることで、候補の選択順・relation構築・fidelity判定を変えず、後方互換な省略としてwireを削減する。

## Global Constraints

- `focalspan.context.v1`、公開MCP tool、CLI、`known_handles`、JSONキー名、SQLite schemaは変更しない。
- 検索、RRF/ranking、候補packing、Evidenceのsource fidelity、relation provenance、budget、deterministic orderingを変更しない。
- target/changeの`symbol`と、全itemのhandle/role/location/fidelityを必ず保持する。
- source/segments/outline/signature本文は変更せず、relation endpointは省略しない。
- `language`はtarget/change、およびPacket内で推論不能なitemでは保持する。Packet内で同一言語が既に明示され、itemが補助roleの場合だけ省略する。
- `kind`はtarget/change、symbolが空のitem、signature/outlineだけで種別判定に必要なitemでは保持する。補助roleでsymbolとsource/segmentsがあり、kindが役割と重複する場合だけ省略する。
- `symbol`は省略しない。`why`はidentity/relation/changedを示す高価値コードだけ保持し、path/lexical/same-fileなど低価値コードは条件付きで削る。
- guidance（limitations/next）は公開契約と上限を維持する。内容を削る場合も、source omission・relation expansion・budget limitationを示す必須項目は保持する。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`は変更・stageしない。
- ベンチマークは候補ごとに1回だけ実行し、source text、絶対パス、ユーザー名、秘密情報を開発レポートへ書かない。

## Context and Orientation

- `internal/evidence/compiler.go` の `buildPacket` がItem/Relationを組み立て、`Compile` が最終Packetへguidanceを付与する。
- `internal/evidence/model.go` の `Item.Language`、`Item.Kind`、`Item.Symbol`、`Item.Why` とPacketの`Limitations`/`Next`はJSON `omitempty` を使用する。
- `internal/evidence/validate.go` はoptional metadataの存在を要求せず、必須のhandle/role/location/fidelity/source/segmentsとrelation endpoint、budgetを検証する。
- `internal/evidence/wire.go` の `MeasureModelVisible` がJSONとsummaryを含む実wire tokenを測定する。
- active v0.10 baselineはfocused/2048で `packing_dropped=7`、packed labels `5`、累積wire tokens `12,740`、useful evidence efficiency `0.3925`、median metadata overhead `0.924`。

## Plan of Work

### Task 0: Freeze transition

- [x] 完了したv0.13 root planを `docs/superpowers/plans/completed/2026-09-02-v0.13-adaptive-focused-excerpt.md` へbyte-identicalにアーカイブする。
- [x] root `PLAN.md`をこのv0.14計画へ置き換える。
- [x] archiveとroot planだけを1つのdocumentation-only transition commitにする。ユーザー所有のdirty filesはstageしない。

### Task 1: RED tests for pruning

**Files:** `internal/evidence/compiler_test.go`、必要に応じて`wire_test.go`/`validate_test.go`。

- [x] `TestPruneMetadataPreservesV1RequiredFields` を追加する。target/changeのsymbol、全itemのhandle/role/location/fidelity、source/segments、relation endpoints、known-handle skip、budgetをprune前後で保持し、`Validate`とdeterministic marshalが通ることを確認する。
- [x] `TestPruneMetadataDropsOnlyRedundantOptionalFields` を追加する。同一languageの補助role、重複kind、低価値why、非必須guidance候補を省略し、高価値identity/relation/changed whyとtarget language/kind/symbolは保持することを確認する。
- [x] `TestPruneMetadataReducesMeasuredWireWithoutChangingSelection` を追加する。同一CompileRequestの選択handle/role/relationをprune前後で比較し、prune後の`MeasureModelVisible`が小さく、`Budget.Used <= Budget.Limit`であることを確認する。
- [x] `TestPruneMetadataIsIdempotentAndSchemaCompatible` を追加する。二重適用で変化せず、schemaが`focalspan.context.v1`のまま、公開キー以外のdebug/token-savingsフィールドが出ないことを確認する。
- [x] 新テストだけを実行し、private pruning関数が未定義でcompile failureになるRED結果を記録する。

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/compiler.go` と必要最小限の新規private helper/test fixture。

- [x] 最終Packetにだけ適用する `prunePacketMetadata` を追加する。候補選択中のtrial packetには適用せず、selection/ranking/packingを不変にする。
- [x] Packet内の明示language集合を決定し、target/changeと推論不能なitemはlanguageを保持する。同一languageが既に明示された補助roleだけ`Language`を空にする。
- [x] target/change、symbol空、signature/outlineのみ、またはsourceなしのitemは`Kind`を保持する。補助roleでsource/segmentsとsymbolがあり役割と重複するkindだけ空にする。
- [x] `Symbol`、handle、role、location、fidelity、本文、relations、budget、known-handle統計は常に保持する。
- [x] `Why`は既存順序を保ったまま、identity（exact/qualified/same-symbol）、relation、changedを優先し、path/lexical/same-fileだけを必要時に省略する。空になったsliceはnilにして`omitempty`を効かせる。
- [x] guidanceは原則そのまま保持し、wire budgetを超える場合の既存`applyGuidanceWithinBudget`だけを利用する。新しい上限・キー・理由文字列は導入しない。
- [x] prune後に`settleWireUsage`を再計算し、`CompileResult.Stats`を最終Packetと一致させる。

### Task 3: Static verification

- [x] 変更Goファイルをgofmtし、`git diff --check`を実行する。
- [x] `go test ./... -count=1`、`go vet ./...`をrepository-local cacheで実行する。
- [x] nativeおよび`CGO_ENABLED=0`のWindows amd64、Linux amd64、Darwin arm64 buildをtemporary directoryへ出力し、生成物を削除する。
- [x] `go test -race ./...`を実行する。MinGWの既知の64-bit compiler制限が再発した場合は`UNVERIFIED`として記録し、成功とは呼ばない。

### Task 4: Candidate benchmark gate

- [ ] historical `focalspan-history-v0.5`をdefault profile、repeat 1、attribution/diagnosis有効で1回だけ実行する。
- [ ] v0.10 baselineに対して、focused/2048の`packing_dropped`非増加、packed labels 5以上、累積wire tokensの`12,740`未満、useful evidence efficiencyの`0.3925`超、quality/fidelity/relation/budget/deterministic/known-handle/MCP契約全通過を要求する。
- [ ] compatible=true、regressions=0、privacy scan結果、artifact hashes、実測値を`docs/benchmarks/findings-v0.14.md`へsource-freeに記録する。strict gate不合格なら新baselineを記録しない。

### Task 5: Closure and recovery

- [ ] strict gate合格時だけ製品commitとbaselineを確定する。
- [ ] gate不合格時は製品commitだけを通常のreverse-order `git revert`で戻し、RED/GREENテストとfindingsを歴史証拠として残し、v0.10 product baselineを維持する。
- [ ] generated reports、indexes、binaries、caches、temporary workspacesを削除し、ユーザー所有dirty filesを保持する。
- [ ] このplanのProgress、Discoveries、Decision Log、Outcomesを実測結果で更新する。archive済みv0.13 planは編集しない。

## Validation and Acceptance

全Packetで、必須identity/location/fidelity/source/segmentsとrelation endpointが不変で、`Validate`が通り、`MeasureModelVisible(packet)`が`Budget.Used`と一致しclamped limit以下であること。pruneは二重適用で不変、同一入力で同一JSONを返すこと。strict candidate gateはwire-token削減とquality/invariant非回帰の両方を要求し、wireまたはefficiencyが改善しない場合は候補を棄却する。

## Idempotence and Recovery

pruningはin-memory Packetへのprivate変換で、index/database/永続stateを変更しない。同一Packetへの再適用はno-opで、Compileのselectionはprune前と同一である。候補が棄却された場合は製品commitのみをordinary `git revert`し、計画とsource-free findingsを保持する。

## Interfaces and Dependencies

公開interfaceは変更しない。`model.PackRequest`、`model.ContextBundle`、Evidence schema `focalspan.context.v1`、MCP methods、CLI output、`known_handles`を維持する。private helperは`internal/evidence`内だけで使用し、通常のMCP/CLI出力にranking/token-savings debug fieldを追加しない。

## Progress

- [x] 2026-09-02: v0.13 adaptive focused excerpt planをbyte-identicalにアーカイブし、v0.10 product baselineへ復帰済み。
- [x] 2026-09-02: v0.14 designとして、最終Packet限定のoptional metadata pruning、必須identity/fidelity/relation保持、公開schema固定を決定。
- [x] Documentation-only transition commit（`5760016`）。
- [x] RED tests（未定義`prunePacketMetadata`によるcompile failureを確認）。
- [x] GREEN implementation（pruning回帰テスト36件を通過）。
- [x] Static verification（全691件、vet、native/cross-build成功。raceはMinGW `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`でUNVERIFIED）。
- [ ] Candidate benchmark gate。
- [ ] Closure and recovery。

## Surprises & Discoveries

- v0.13のadaptive excerptは単体で候補を縮約できてもhistorical suiteでは選択されずwireとefficiencyが変わらなかったため、v0.14ではselectionを変えず最終Packetのmetadataだけを直接削減する。
- metadata pruning後もselection/ranking/packingを変えない設計により、全691テスト、vet、native/cross-buildは通過した。raceだけは既知のMinGW toolchain制約で実行不能だった。

## Decision Log

- 2026-09-02: v0.13がstrict gate不合格だったため、優先順位4のmetadata pruningへ遷移する。
- 2026-09-02: 候補選択中のtrial packetをpruneせず、最終出力だけをpruneしてranking/packingの回帰を防ぐ。
- 2026-09-02: `symbol`は常時保持し、target/changeのlanguage/kindとrelation provenanceを保持する。optional field omissionだけでv1互換を保つ。
- 2026-09-02: guidanceの公開上限と理由文字列は変更せず、rank 6の共同budget化と分離する。

## Outcomes & Retrospective

未完了。候補ベンチマークの実測値と、採用または棄却の根拠を完了時に追記する。
