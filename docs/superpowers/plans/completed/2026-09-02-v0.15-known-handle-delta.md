# FocalSpan v0.15 multi-turn known_handles差分最適化 実装計画

> **For agentic workers:** REDテストを先に追加し、known_handlesを含む二段階deltaだけを測定してから最小実装する。

**Goal:** `code_context → code_expand` の既知handle差分を、公開MCP/CLIインターフェースとEvidence品質を変えずに効率化する。既知Evidenceの再送を0件に保ち、fixture評価の二段階delta token ratioを現行`0.5578351609480015`未満へ下げる。

**Architecture:** `known_handles`の正規化、候補選択、relation endpoint、source fidelityは維持する。最終Packetを組み立てる際、known_handlesを含むexpandだけに適用するprivateなdelta-envelope規則を追加し、既知anchorに由来する冗長なlimitations/next guidanceを安全に抑制する。通常の初回query、known_handlesなしのcontrol expand、legacy packetは変更しない。空Packetでも`schema`、`mode`、`budget`、`skipped_known`、必須relation状態を維持し、JSONキー名とsummary契約は固定する。

**Tech Stack:** Go、`internal/evidence` compiler/known tests、`internal/app` expansion tests、`internal/eval` evidence evaluator、`budget.TokenEstimator`、fixture `evidencesample`。

**Spec:** ユーザー承認済み「FocalSpan token効率改善候補・優先順位」rank 5「multi-turn known_handles差分最適化」。

## Purpose / Big Picture

statelessな`known_handles`は、会話の前ターンで返したhandleを次の`code_expand`に渡して同じEvidenceを再送しない契約である。現在は既知anchorの抑制自体は機能し、known resendは0だが、既知anchorが省略されたexpandでも固定のguidanceとPacket envelopeが残り、fixtureのmedian two-step delta ratioは`0.5578351609480015`である。known-only deltaで不要な次アクションや重複制限を出さないことで、初回queryとcontrol expandの意味を保ちながら、known expand側のwireを縮める。

## Global Constraints

- 公開MCP tool、CLI、`focalspan.context.v1`、JSONキー名、`known_handles`入力、SQLite schemaは変更しない。
- source fidelity、relation provenance、relation endpoint validity、budget、deterministic ordering、handle安定性を維持する。
- known handleは常に再送しない。known-onlyでEvidenceが空でも`skipped_known`の意味と整数値を保持する。
- 初回queryとknown_handlesなしcontrol expandは同じPacket内容・選択結果を維持する。変更対象はknown_handlesを含むexpandの冗長guidanceに限定する。
- `source_body_omitted`、未解決relation、budget limitationなど、ユーザーの次操作や安全性に必要なguidanceは削除しない。既知anchorの重複を説明するだけの項目を抑制する。
- 公開summaryはsource-free、通常Packetにranking/token-savings debug fieldを追加しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`は変更・stageしない。
- fixture評価、生成index、レポート、cacheは完了後に明示的に削除する。ベンチマークの同一候補再実行はしない。

## Context and Orientation

- `internal/app/evidence.go` の `ExpandEvidence` は`known_handles`を正規化し、relation候補とanchorを取得して`evidence.Compiler.Compile`へ渡す。
- `internal/evidence/compiler.go` の`preprocess`はknown handleを候補からskipし、`Compile`後半でguidanceとwire使用量を確定する。
- `internal/evidence/next.go` の`BuildGuidance`はsignature/source omission、relation omission、known anchor not repeatedをlimitations/nextへ変換する。
- `internal/benchmark/expand.go` と `internal/eval/evidence.go` は、known resend、relation validity、累積wire、二段階delta ratioを測定する。
- v0.14では最終Packetのmetadata pruningを導入済みだが、known expansion envelopeは未最適化。v0.14 baselineのwire/qualityを下回らないことを確認する。

## Plan of Work

### Task 0: Freeze transition

- [x] 完了したv0.14 root planを `docs/superpowers/plans/completed/2026-09-02-v0.14-metadata-field-pruning.md` へbyte-identicalにアーカイブする。
- [x] root `PLAN.md`をこのv0.15計画へ置き換える。
- [x] archiveとroot planだけを1つのdocumentation-only transition commitにする。ユーザー所有のdirty filesはstageしない。(`ba60039`)

### Task 1: RED tests for known-handle delta

**Files:** `internal/evidence/compiler_test.go`、`internal/app/evidence_test.go`、必要に応じて`internal/eval/evidence_test.go`。

- [x] `TestKnownHandleDeltaKeepsKnownSkipAndRelationInvariants` を追加する。初回Packetのhandlesをknownに渡したexpandでknown Evidenceが0件再送、`SkippedKnown`が正確、dangling edgeがなく、relation candidateのsource/fidelityは保持されることを確認する。
- [x] `TestKnownHandleDeltaSuppressesOnlyRedundantGuidance` を追加する。known anchorを説明する重複guidanceだけを抑制し、unresolved relation、source omission、budget limitation、next relation actionは保持することを確認する。
- [x] `TestKnownHandleDeltaDoesNotChangeControlOrInitialPackets` を追加する。同一query、knownなしcontrol expand、source/outline mode、空knownのPacketをbyte-identicalに比較する。
- [x] `TestKnownHandleDeltaIsDeterministicAndBudgetSafe` を追加する。同一known入力を反復して同じJSONを得て、`Validate`、`Budget.Used == MeasureModelVisible`、hard limitを確認する。
- [x] fixture queryのdelta ratio改善テストを追加し、実装前の基準値以上から実装後に基準未満となることを確認した。

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/compiler.go` と必要最小限の`internal/evidence/next.go` helper、テストfixture。

- [x] `CompileRequest`の公開型を変えず、`len(req.KnownHandles)>0`をprivateなdelta-envelope判定に使う。
- [x] known expandで、既知anchorを再説明するだけの`known_anchor_not_repeated` limitationや同一handleのself `NextAction`を、relation/actionが別途存在しない場合だけ抑制する。unresolved relation、omitted relation、source omission、budget limitationは残す。known-only empty packetの`no_relevant_source_found`も意図的known skipの場合だけ抑制した。
- [x] known handles自体、`SkippedKnown`、selected relation candidates、`selectedEdges`、source/segments/signature、local IDs、guidance上限は変更しない。
- [x] control expandと初回queryにはpruneを適用せず、既存JSONを保持する。
- [x] 最終Packetのwire使用量と`CompileResult.Stats`を再計算し、known-only empty packetも`Validate`を通す。
- [x] private規則を二重適用してもno-opにし、既存のmetadata pruningと決定順を壊さない。

### Task 3: Static verification

- [x] gofmtと`git diff --check`を実行する。
- [x] `go test ./... -count=1`、`go vet ./...`をrepository-local cacheで実行する。
- [x] nativeおよび`CGO_ENABLED=0`のWindows amd64、Linux amd64、Darwin arm64 buildを一時出力へ実行し、生成物を削除する。（この時点では削除前）
- [x] `go test -race ./...`を実行する。MinGWの64-bit compiler制限が再発したため`UNVERIFIED`として記録する。
- [x] `cmd/focalspan-eval --root testdata/repos/evidencesample --cases testdata/eval/evidence-cases.jsonl --contract compare --json` を実行し、known resend 0、relation/fidelity/budget/deterministic 1、delta ratio `0.5550053059780686`を記録する。

### Task 4: Candidate benchmark gate

- [x] historical `focalspan-history-v0.5`をdefault profile、repeat 1、attribution/diagnosis有効で候補ごとに1回だけ実行する。v0.14 baseline reportと比較する。
- [x] focused/2048の`packing_dropped`非増加、packed labels 5以上、累積wireがv0.14の12,304以下、useful efficiencyが0.4064以上、quality/fidelity/relation/budget/deterministic/known-handle/MCP契約非回帰を要求する。
- [x] fixture delta ratioが`0.5578351609480015`未満、known resend 0、relation validity 1を確認した。strict gateを通過した。
- [x] compatible=true、regressions=0、privacy scan、artifact hashes、実測値を`docs/benchmarks/findings-v0.15.md`へsource-freeに記録する。

### Task 5: Closure and recovery

- [x] 全ゲート合格時だけ製品commit、findings、`results-v0.15.{json,md}` baselineを確定する。（製品`b07203a`）
- [x] gate不合格時のrevertは不要。v0.14 product baselineを下回らないことを確認した。
- [x] generated reports、fixture indexes、binaries、caches、temporary workspacesを削除し、ユーザー所有dirty filesを保持する。
- [x] このplanのProgress、Discoveries、Decision Log、Outcomesを実測結果で更新し、archive済みv0.14 planは編集しない。

## Validation and Acceptance

known expansionで既知handleの再送が0、`skipped_known`が正確、relation endpointが有効、source fidelityと必須identityが不変、`Validate`が成功し、wire使用量がbudget内で`MeasureModelVisible`と一致すること。初回およびknownなしcontrolのPacketは既存JSONと同一で、同一入力は決定的であること。fixtureのmedian two-step delta ratioが`0.5578351609480015`未満になり、historical gateもv0.14 qualityを下回らないこと。

## Idempotence and Recovery

known delta pruningはin-memory Packetへのprivate変換で、index/database/永続stateを変更しない。二重適用はno-opで、known handlesの順序・重複除去は既存NormalizeKnownHandlesに委ねる。候補が棄却された場合は製品commitだけをordinary `git revert`し、v0.14 baselineを保持する。

## Interfaces and Dependencies

公開interfaceは変更しない。`code_context`、`code_expand`、`code_impact`、CLI evidence、`focalspan.context.v1`、`known_handles`、legacy `ContextBundle`、MCP summaryを維持する。private helperは`internal/evidence`に限定し、development trace以外へranking/token-savings debug情報を出さない。

## Progress

- [x] 2026-09-02: v0.14 metadata pruningを採用し、v0.14 baseline reportを保存済み。
- [x] 2026-09-02: v0.14 planをbyte-identicalにアーカイブし、v0.15のknown_handles差分設計を作成（`ba60039`）。
- [x] 2026-09-02T06:35Z: REDテストを追加し、既存delta ratio `0.5578351609480015`を下回らない失敗を確認した。
- [x] 2026-09-02T06:35Z: private guidance pruningとfixture regressionを実装（`b07203a`）。
- [x] 2026-09-02T06:35Z: 全体test/vet/build、fixture evaluator、race（MinGW制約でUNVERIFIED）を実行した。
- [x] 2026-09-02T06:35Z: historical benchmarkを1回実行し、v0.14 baseline比較でcompatible=true/regressions=0を確認した。
- [x] 2026-09-02T06:35Z: generated reports、binaries、caches、temporary workspaceを削除した。

## Surprises & Discoveries

- self expansionではknown anchorが全てskipされ、`known_anchor_not_repeated`ではなく`no_relevant_source_found`がknown-only empty envelopeとしてwireを占めていた。`budget_limited`と`skipped_known`は保持したまま、この誤解を招く項目だけをknown-only時に抑制した。
- historical suiteのdefault profilesは有効なexpand anchorを取得しないケースが多く、quality/cumulative-wire gateはv0.14と同値だった。一方、evidence fixtureはself deltaを実測でき、ratio改善を確認できた。

## Decision Log

- 2026-09-02: 次順位としてmulti-turn known_handles差分を選択し、retrieval/ranking/packingの変更を避ける。
- 2026-09-02: 初回queryとknownなしcontrolを不変にし、known handlesを含むexpandの冗長guidanceだけを対象にする。
- 2026-09-02: `skipped_known`、relation validity、source fidelity、public schemaを保持し、stateless契約を壊さない。
- 2026-09-02: known-only empty packetでは`no_relevant_source_found`を冗長と判定するが、`budget_limited`、source omission、unresolved/omitted relation guidanceは削除しない。

## Outcomes & Retrospective

v0.15はstrict gateを通過し採用した。private guidance pruningでfixture delta ratioを`0.5550053059780686`へ改善し、known resend 0、relation/fidelity/budget/deterministic 1を維持した。historical candidateは累積wire `12,304`、useful evidence `5`、efficiency `0.4064`でv0.14 baseline非回帰、comparisonはcompatible=true/regressions=0だった。raceはMinGW 64-bit compiler制約により未検証である。生成物削除まで完了した。
