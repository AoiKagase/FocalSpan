# FocalSpan v0.26 emitted-long-excerpt replacement 調査計画

**Goal:** 実際に送信されている長いfocused excerptが存在する場合だけ、宣言部と
query-hit windowへ置換してsource fidelityを保ったままwireを削減する。

## Purpose / Big Picture

v0.13の追加adaptive variantは効果がなく再試行しない。今回はv0.25 development metricsで
実送信excerptのrow別Evidence contentを調べ、十分に長い対象が存在するときだけ既存excerpt
表現そのものの置換を検討する。対象がなければ製品コードを変更しない。

## Baseline and Gates

- v0.25 history: wire 11,693、bytes 32,494、excerpt 15、useful 5、効率0.4276。
- 事前traceで長いselected excerpt rowが存在することを製品実装の必要条件とする。
- 対象がある場合はlate-hit、行番号、source fidelity、selected handleを不変とする。
- 対象rowごとのwireを厳密に削減し、全row非回帰とする。

## Global Constraints

- 公開schema、MCP/CLI、known handles、retrieval、ranking、packingを変更しない。
- v0.13と同じ追加variantは導入しない。
- 対象rowがなければRED/GREENや候補ベンチを実行せずno-op findingで閉じる。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.25をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: Trace-only precondition

- [x] v0.25 resultからexcerpt送信rowとEvidence content bytesを列挙する。
- [x] 長文置換に値する対象rowの有無を判定する。
- [x] 対象なしの場合は製品変更なしでfindingを作成する。

### Task 2: Conditional implementation

- [x] 対象がある場合だけREDでlate-hit、行番号、source一致を固定する（対象なし）。
- [x] 既存excerpt表現を宣言部＋query-hit windowへ置換する（対象なし）。

### Task 3: Verification and gate

- [x] 製品変更時だけfocused/full tests、vet、candidate benchmarkを実行する（製品変更なし）。
- [x] findingと採否を確定する。
- [ ] 完了後v0.27 failure-layer micro-retriever planへ遷移する。

## Validation and Acceptance

対象なしなら製品変更ゼロで正直に閉じる。対象ありならそのrowのwireを削減し、
late-hit、行番号、source fidelityと全共通contractを維持する。

## Idempotence and Recovery

trace-only調査は既存結果から再実行可能とする。製品候補が不合格なら通常revertし、
測定結果とfindingは保持する。

## Interfaces and Dependencies

公開interface変更なし。条件成立時のみ`internal/evidence`のfocused segment生成とtestsを
対象とする。

## Progress

- [x] 2026-09-04: v0.25 acceptance後、v0.26へ遷移した。
- [x] 2026-09-04: v0.25結果の15 excerpt rowをtrace-only集計した。
- [x] 2026-09-04: 最大Evidence content 228 bytesで対象なしと判定し、no-opで閉じた。

## Surprises & Discoveries

- excerpt rowは15件あったが、row全Evidence contentでも228/124/41 bytesに収まり、
  1 KiBを超える送信はなかった。
- 現行focused segmentは既に宣言prefixとquery-hit windowを組み合わせており、今回の
  置換案との構造差も存在しなかった。

## Decision Log

- 2026-09-04: 「長い」はrow全体のEvidence contentが1 KiB超を最低条件とし、数百byteの
  excerptは置換対象にしない。
- 2026-09-04: 最大228 bytesのため製品実装、RED/GREEN、候補ベンチを行わず、no-op
  findingとして閉じる。

## Outcomes & Retrospective

v0.26は事前条件を満たさず、製品コードとv0.25 baselineを一切変更しなかった。対象の
ない最適化を避けられた。今後は実送信excerptが1 KiBを超えるtraceが得られた場合だけ
再検討する。次はfailure-layer traceを原因別に集計し、同一形式で3 labels以上を改善
できるmicro-retrieverが存在するかを調査する。
