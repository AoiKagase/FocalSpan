# FocalSpan v0.29 known-handles delta phase 2 計画

**Goal:** known-handle expansionに残るrelation action/limitationの重複だけを除去し、
初回queryを変えずにmulti-turn delta wireを削減する。

## Purpose / Big Picture

v0.15はknown Evidence resendを0にし、v0.24/v0.25後のmedian delta ratioは0.5521958。
現行`pruneKnownDeltaGuidance`はknown-only empty packetを扱うが、relation edgeを含む場合は
早期returnするため、structured relationと重複するguidanceが残る可能性がある。

## Baseline and Gates

- delta ratio 0.5521958未満、known resend 0、relation validity 1。
- 初回query packetはbyte-identical。
- relation endpoint/actionabilityを壊さず、with-known packet wireを厳密に削減する。
- history baseline wire 11,693、bytes 32,494を非回帰とする。

## Global Constraints

- 公開schema、MCP/CLI、known handles、selection、rankingを変更しない。
- structured relation edgeと同じhandle/relationを示すguidanceだけを重複扱いする。
- dangling edge/actionを作らず、通常初回queryには適用しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.28をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED and trace

- [ ] with-known relation packetの残存guidanceをfixtureで固定する。
- [ ] initial byte identity、known resend 0、relation validityを固定する。
- [ ] edgeと同一のnext actionだけが削除対象になることを固定する。

### Task 2: GREEN implementation

- [ ] `pruneKnownDeltaGuidance`をrelation-edge重複へ限定拡張する。
- [ ] safety/actionable limitationは維持する。

### Task 3: Verification and gate

- [ ] focused/full tests、vet、diff checkを通す。
- [ ] Evidence evalとhistory candidateを各1回測定し採否を確定する。
- [ ] 完了後v0.30 TokenEstimator oracle gateへ遷移する。

## Validation and Acceptance

初回byte identityとrelation validityを保ち、known delta ratioを0.5521958未満へ下げる。

## Idempotence and Recovery

pruningはpure/idempotent。不合格時は通常revertし、findingを保持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/evidence/next.go`とdelta contract testsだけを対象とする。

## Progress

- [x] 2026-09-04: v0.28 rejection後、v0.29へ遷移した。

## Surprises & Discoveries

実装中に更新する。

## Decision Log

- 2026-09-04: relation edgeが同じ情報を保持する場合だけguidanceを重複とみなす。

## Outcomes & Retrospective

測定後に更新する。
