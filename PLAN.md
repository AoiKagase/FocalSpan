# FocalSpan v0.28 guidance-funded fidelity fallback 計画

**Goal:** legacy Evidence handlesを固定し、metadata pruning後の余剰wireでfidelityを昇格し、
再計算されたguidanceを含む完成packetがlegacy wire以下の場合だけ採用する。

## Purpose / Big Picture

v0.16はguidanceをselection trialへ無条件加算してrow regressionを生んだ。今回はlegacy
selectionを先に完成させ、そのhandle集合を変えないbounded fallbackだけを評価する。

## Baseline and Gates

- accepted baseline: wire 11,693、bytes 32,494、useful 5、効率0.4276。
- 全rowのselected handlesをbyte-identicalに保つ。
- candidateはlegacy完成packet wire以下の場合だけ採用する。
- per-row wire regression 0。fidelity昇格またはwire削減がなければno-opとする。

## Global Constraints

- 公開schema、MCP/CLI、known handles、retrieval、ranking、initial selectionを変更しない。
- 最大selected件数×variant数だけを決定的に評価し、探索をboundedにする。
- relation/source fidelity/budget/determinism/forbidden contractを維持する。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.27をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED tests

- [ ] handles固定、wire ceiling、決定性を固定する。
- [ ] affordable upgradeとno-op fallbackを固定する。
- [ ] guidanceが昇格後fidelityから再計算されることを固定する。

### Task 2: GREEN implementation

- [ ] 完成packet生成をprivate helperへ抽出する。
- [ ] selected variantだけをboundedに昇格し、strict fallbackを適用する。

### Task 3: Verification and gate

- [ ] focused/full tests、vet、diff checkを通す。
- [ ] history candidateを1回測定し、採否を確定する。
- [ ] 完了後v0.29 known-handles delta phase 2へ遷移する。

## Validation and Acceptance

handlesと全contractを維持し、candidate完成packetがlegacy wire以下の場合だけfidelityを
上げる。全history row wire非回帰を必須とする。

## Idempotence and Recovery

fallbackはpure/deterministic。不合格時は製品候補を通常revertし、findingを保持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/evidence/compiler.go`とtestsだけを製品対象とする。

## Progress

- [x] 2026-09-04: v0.27 rejection後、v0.28へ遷移した。

## Surprises & Discoveries

実装中に更新する。

## Decision Log

- 2026-09-04: v0.16と異なりlegacy selectionを変更しないpost-selection fallbackに限定する。

## Outcomes & Retrospective

測定後に更新する。
