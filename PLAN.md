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
- [ ] root `PLAN.md`をこのv0.16計画へ置き換える。
- [ ] archiveとroot planだけをdocumentation-only transition commitにする。ユーザー所有dirty filesはstageしない。

### Task 1: RED tests for shared budget

**Files:** `internal/evidence/compiler_test.go`、必要に応じて`internal/app/evidence_test.go`。

- [ ] guidance-aware trialがEvidence wireだけでなくlimitations/nextを含めて測定することを固定する。
- [ ] `budget_limited`、source omission、unresolved/omitted relation、known skipを削らず、guidance上限と`Validate`を保持することを固定する。
- [ ] initial、knownなしcontrol、known expansionのselection/JSON determinismとhard budgetを固定する。
- [ ] guidance-aware候補がfitしない場合の既存selection fallbackを固定する。

### Task 2: Minimal GREEN implementation

**Files:** `internal/evidence/compiler.go` と必要最小限のprivate helper/tests。

- [ ] `CompileRequest`や公開型を変えず、selection trial用`buildPacketWithGuidance`を追加する。
- [ ] trial packetは`buildPacket`→`BuildGuidance`→known-delta pruning→metadata pruning→`settleWireUsage`の順で作り、omitted relationとguidanceコストを同じwire予算へ含める。
- [ ] anchor/trial/currentの候補比較だけをguidance-awareにし、utility、tie-break、candidate order、fallbackを維持する。
- [ ] Compile末尾の最終guidance trimmingとStats再計算は既存順序を保ち、二重適用をno-opにする。
- [ ] guidance-aware結果が既存selectionより品質・契約を悪化させる候補では既存selectionへフォールバックする。

### Task 3: Static verification

- [ ] gofmt、`git diff --check`。
- [ ] repository-local cacheで`go test ./... -count=1`、`go vet ./...`。
- [ ] native、CGO-free Windows amd64/Linux amd64/Darwin arm64 buildを一時出力し削除する。
- [ ] `go test -race ./...`を実行し、MinGW制約ならUNVERIFIEDと記録する。
- [ ] fixture evaluatorでcoverage/role/fidelity/relation/budget/determinism、known resend、delta ratioを再測定する。

### Task 4: Candidate benchmark gate

- [ ] historical `focalspan-history-v0.5`をdefault profile、repeat 1、attribution/diagnosis有効で1回実行し、v0.15 baselineと比較する。
- [ ] focused/2048の`packing_dropped`非増加、packed labels 5以上、累積wire `<=12,304`、useful efficiency `>=0.4064`を要求する。
- [ ] anchor recall、source fidelity、relation validity、budget、determinism、known resend、MCP契約に非回帰を要求する。
- [ ] `compatible=true`、`regressions=0`、privacy scan、artifact hashes、実測値を`docs/benchmarks/findings-v0.16.md`へsource-freeに記録する。未達ならbaselineを作らない。

### Task 5: Closure and recovery

- [ ] 全ゲート合格時だけ製品commit、findings、`results-v0.16.{json,md}`を確定する。
- [ ] gate不合格時は製品commitだけをreverse-order `git revert`し、RED/GREENテストとfindingsを残し、v0.15 baselineを維持する。
- [ ] generated reports/indexes/binaries/caches/workspacesを削除し、ユーザー所有dirty filesを保持する。
- [ ] Progress、Discoveries、Decision Log、Outcomesを実測値で更新し、v0.15 archiveは編集しない。

## Validation and Acceptance

同一入力は決定的で、`Validate`が成功し、model-visible wireと`Budget.Used`/Statsが一致すること。initialとknownなしcontrolは既存JSONと同一、known expansionはknown resend 0とrelation validity 1を維持すること。historical gateはv0.15のquality、packed labels、wire、efficiencyを下回らず、比較はcompatible=true/regressions=0であること。

## Idempotence and Recovery

guidance-aware builderはin-memory trialだけでindex/databaseを変更しない。known-delta/metadata pruningの二重適用はno-op。候補棄却時は製品commitのみordinary revertし、v0.15 baselineとsource-free findingsを維持する。

## Interfaces and Dependencies

公開interfaceは変更しない。`code_context`、`code_expand`、`code_impact`、CLI evidence、`focalspan.context.v1`、`known_handles`、legacy `ContextBundle`を維持する。private helperは`internal/evidence`内に限定する。

## Progress

- [x] 2026-09-02: v0.15 strict gate通過、product `b07203a`、docs/baseline `20b78c9`を確定。
- [x] 2026-09-02: v0.15 planをbyte-identical archiveへコピーした。
- [ ] v0.16 documentation-only transition commit。
- [ ] RED tests。
- [ ] GREEN implementation。
- [ ] Static verification。
- [ ] Candidate benchmark gate。
- [ ] Closure and recovery。

## Surprises & Discoveries

- 未測定。現行のguidanceは候補選択後にwireへ加算され、budget超過時にguidanceだけをtrimする。候補trialへ合成した場合のselection・quality・wire影響をfixtureとhistorical gateで確認する。

## Decision Log

- 2026-09-02: v0.15の次順位としてguidanceと候補選択の共同budget化を開始する。
- 2026-09-02: retrieval/ranking/public schemaを変更せず、compiler selection trialだけへ適用する。
- 2026-09-02: essential guidanceを削除せず、fitしない場合は既存selectionへfallbackする。

## Outcomes & Retrospective

未完了。共同budget化のwire/qualityゲート、採用または棄却の根拠を完了時に追記する。
