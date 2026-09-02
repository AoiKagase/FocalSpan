# FocalSpan v0.16 guidance共同budget化 実装計画

> **For agentic workers:** 候補選択がguidance wireを予算へ含めることをREDテストで固定し、最小のselection-envelope実装後に品質・wireゲートを測定する。

**Goal:** Evidence候補の選択時にlimitations/next guidanceのwireコストも同じbudgetへ予約し、不要なguidanceを抑えながら有用Evidenceを保持する。初回・known expansion・MCP/CLI契約、anchor recall、source fidelity、determinismを維持し、v0.15 baselineの累積wire `12,304`以下、useful efficiency `0.4064`以上を満たす。

**Architecture:** retrieval/rankingと公開型は変更しない。compilerのselection trialだけにprivateなguidance-aware packet builderを使い、候補ごとのomitted relationとguidanceを含むmodel-visible wireを比較する。最終Packetは既存のguidance budget trimming、v0.15 known-delta pruning、metadata pruningを同じ順序で適用する。guidance-aware候補が一つもfitしない場合は既存selectionへ安全にフォールバックする。

**Spec:** ユーザー承認済み「FocalSpan token効率改善候補・優先順位」rank 6「guidanceと候補選択の共同budget化」。

## Purpose / Big Picture

現在のcompilerは候補をEvidenceだけのwireで選択し、選択完了後にguidanceを付加する。予算超過時はnext/limitationsだけが後から削られるため、guidanceの多い候補集合を選んだ結果、必要なEvidenceと次操作案が競合する。selection trial時点で同じguidanceを合成し、予算内の候補をutility/wire比で選ぶことで、同一hard budget内のEvidence密度を改善する。

## Global Constraints

- 公開MCP tool、CLI、`focalspan.context.v1`、JSON key、`known_handles`、SQLite schemaを変更しない。
- retrieval、RRF/ranking、relation resolution、source fidelity、handle、local ID、deterministic orderingを変更しない。
- `source_body_omitted`、unresolved/omitted relation、`budget_limited`、known skip、relation endpointは保持する。
- guidanceの上限（limitations 8、next 4）と既存理由コードを変更しない。
- guidance-aware試行はprivate in-memory評価のみ。通常Packetにranking/token-savings debug fieldを出さない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`は変更・stageしない。
- fixture/historical reports、generated indexes、binaries、cacheはゲート後に削除する。benchmarkは候補につき1回だけ。

## Context and Orientation

- `internal/evidence/compiler.go` のselection loopは`buildPacket`でEvidence wireだけを試し、Compile末尾で`BuildGuidance`と`applyGuidanceWithinBudget`を実行する。
- `internal/evidence/next.go` はintent、omitted relation、source omission、known-deltaのguidanceを決定順に構築する。
- `internal/evidence/wire.go` の`MeasureModelVisible`がJSONとsummaryを含む実wireを測定し、metadata/known pruning後に`Budget.Used`を確定する。
- `internal/benchmark/efficiency.go` と`internal/eval/evidence.go` はuseful evidence、累積wire、known resend、delta ratioを集計する。

## Plan of Work

### Task 0: Freeze transition

- [x] v0.15 root planを`docs/superpowers/plans/completed/2026-09-02-v0.15-known-handle-delta.md`へbyte-identicalにアーカイブする。
- [x] root `PLAN.md`をこのv0.16計画へ置き換える。
- [x] archiveとroot planだけをdocumentation-only transition commitにする。ユーザー所有dirty filesはstageしない。

### Task 1: RED tests for shared budget

**Files:** `internal/evidence/compiler_test.go`、必要に応じて`internal/app/evidence_test.go`。

- [x] guidance-aware trialがEvidence wireだけでなくlimitations/nextを含めて測定することを固定する。
- [x] `budget_limited`、source omission、unresolved/omitted relation、known skipを削らず、guidance上限と`Validate`を保持することを固定する。
- [x] initial、knownなしcontrol、known expansionのselection/JSON determinismとhard budgetを固定する。
- [x] guidance-aware候補がfitしない場合の既存selection fallbackを固定する。

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/compiler.go` と必要最小限のprivate helper/tests。

- [x] `CompileRequest`や公開型を変えず、selection trial用`buildPacketWithGuidance`を追加する。
- [x] trial packetは`buildPacket`→`BuildGuidance`→known-delta pruning→metadata pruning→`settleWireUsage`の順で作り、omitted relationとguidanceコストを同じwire予算へ含める。
- [x] anchor/trial/currentの候補比較だけをguidance-awareにし、utility、tie-break、candidate order、fallbackを維持する。
- [x] Compile末尾の最終guidance trimmingとStats再計算は既存順序を保ち、二重適用をno-opにする。
- [x] 候補実装をfocused/full testで検証したが、歴史ベンチのwire回帰により `f940b84` から `7b24f55` でrevertした。既存selectionとの品質比較fallbackは次候補の前提として残す。

### Task 3: Static verification

- [x] gofmt、`git diff --check`。
- [x] repository-local cacheで`go test ./... -count=1`、`go vet ./...`。
- [x] native、CGO-free Windows amd64/Linux amd64/Darwin arm64 buildを一時出力し削除する。
- [x] `go test -race ./...`を実行し、MinGW制約（`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`）のためUNVERIFIEDと記録する。
- [x] fixture evaluatorでcoverage/role/fidelity/relation/budget/determinism、known resend、delta ratioを再測定する。

### Task 4: Candidate benchmark gate

- [x] historical `focalspan-history-v0.5`をdefault profile、repeat 1、attribution/diagnosis有効で1回実行し、v0.15 baselineと比較する。
- [x] focused/2048の`packing_dropped`非増加、packed labels 5以上、累積wire `<=12,304`、useful efficiency `>=0.4064`を要求する。
- [x] anchor recall、source fidelity、relation validity、budget、determinism、known resend、MCP契約に非回帰を要求する。
- [x] `compatible=true`、privacy scan、artifact hashes、実測値を`docs/benchmarks/findings-v0.16.md`へsource-freeに記録した。`regressions=5`のためbaselineは作成しない。

### Task 5: Closure and recovery

- [x] 全ゲート合格時だけ製品commit、findings、`results-v0.16.{json,md}`を確定する（不合格のためresultsは作成しない）。
- [x] gate不合格時は製品commit `f940b84` を reverse-order `git revert` `7b24f55` で取り消し、RED/GREENテストとfindingsを履歴へ残し、v0.15 baselineを維持する。
- [x] v0.16 generated reportsとrepository-local cacheを削除し、既存のindex/worktree/binary artifactsおよびユーザー所有dirty filesを保持する。
- [x] Progress、Discoveries、Decision Log、Outcomesを実測値で更新し、v0.15 archiveは編集しない。

## Validation and Acceptance

同一入力は決定的で、`Validate`が成功し、model-visible wireと`Budget.Used`/Statsが一致すること。initialとknownなしcontrolは既存JSONと同一、known expansionはknown resend 0とrelation validity 1を維持すること。historical gateはv0.15のquality、packed labels、wire、efficiencyを下回らず、比較はcompatible=true/regressions=0であること。

## Idempotence and Recovery

guidance-aware builderはin-memory trialだけでindex/databaseを変更しない。known-delta/metadata pruningの二重適用はno-op。候補棄却時は製品commitのみordinary revertし、v0.15 baselineとsource-free findingsを維持する。

## Interfaces and Dependencies

公開interfaceは変更しない。`code_context`、`code_expand`、`code_impact`、CLI evidence、`focalspan.context.v1`、`known_handles`、legacy `ContextBundle`を維持する。private helperは`internal/evidence`内に限定する。

## Progress

- [x] 2026-09-02: v0.15 strict gate通過、product `b07203a`、docs/baseline `20b78c9`を確定。
- [x] 2026-09-02: v0.15 planをbyte-identical archiveへコピーした。
- [x] 2026-09-02T06:53Z: v0.16 documentation-only transition commit `b746cc3` を確定した。
- [x] 2026-09-02T07:20Z: guidance wire共同選択のRED/GREEN実装とcandidate commit `f940b84` を検証した。
- [x] 2026-09-02T07:20Z: historical benchmarkを1回実行し、48 quality/40 attribution/40 diagnosisを生成。比較はcompatible=true、regressions=5。
- [x] 2026-09-02T07:20Z: strict gate不合格を確定し、product commitを `7b24f55` でrevertした。findings-v0.16を作成し、v0.15 baselineを維持する。
- [x] 2026-09-02T07:24Z: revert後の `go test ./... -count=1`（696 tests/46 packages）、`go vet ./...`、gofmt、diff checkを再確認。raceはMinGW制約でUNVERIFIED。

## Surprises & Discoveries

- guidance-aware trialはaggregate wire（12,304→12,240）と効率（0.4064→0.4085）を改善したが、MCPケースのvariant選択がverbatimへ寄り、5行でwire 251→286の回帰を生んだ。
- attribution/diagnosis出力はv0.15と同一hashで、packing_dropped 7、packed labels 5、recall/fidelity/relation/budget/determinismは非回帰だった。
- 単純な「fitしない場合だけlegacy fallback」では、fitするがwireが悪化する候補を防げない。per-row legacy-selection比較が必要である。

## Decision Log

- 2026-09-02: v0.15の次順位としてguidanceと候補選択の共同budget化を開始する。
- 2026-09-02: retrieval/ranking/public schemaを変更せず、compiler selection trialだけへ適用する。
- 2026-09-02: essential guidanceを削除せず、fitしない場合は既存selectionへfallbackする。
- 2026-09-02: strict historical gateはper-row wire回帰5件を理由に不合格とし、v0.15 baselineを維持する。候補は `f940b84` から `7b24f55` でrevertし、同じ候補を再実行しない。

## Outcomes & Retrospective

v0.16はnegative candidateとして終了した。共同budget化は局所テストとaggregate効率では有望だったが、MCPの5 quality rowsでwire回帰が発生し、strict non-regression gateを満たさなかった。製品変更はrevertされ、v0.15（wire 12,304、efficiency 0.4064）が現行baselineである。将来再検討する場合は、guidance-aware trialとlegacy selectionのper-row比較fallbackを先に実装し、別マイルストーンとして新しい単一ベンチを実行する。
