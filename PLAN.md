# FocalSpan v0.25 relevance-aware abstention 計画

**Goal:** 意味的な支持根拠を持たない初回query候補をEvidenceへ送らず、明示的な
no-result queryだけを小さい空packetへ変換してmodel-visible wireを削減する。

## Purpose / Big Picture

v0.21で追加した`no-relevant-source` fixtureは、存在しない一意identifierに対して
無関係なEvidence 1件を返すことを測定済みである。一方、同じfixture suiteには8件の
positive Evidence、late-hit、relation、known expansionがある。これらを誤って空にせず、
retrieval-fusionだけで意味的一致理由を持たない候補群に限定してabstainする。

## Baseline and Gates

- v0.24: history wire 11,693、bytes 32,494、useful 5、効率0.4276。
- Evidence evalのpositive 8件でexpected coverage、role、late-hitを非回帰とする。
- `no-relevant-source`だけはEvidence 0件、valid empty packet、wire減少とする。
- history comparison regression 0、budget/determinism/relation validity 1、forbidden 0、
  known resend 0を維持する。

## Global Constraints

- 公開schema、MCP/CLI、known handles、ranking weight、packingを変更しない。
- abstentionは初回`QueryEvidence`だけに適用し、expand/impactには適用しない。
- explicit path scopeまたはchanged-onlyで選ばれた候補は自動abstainしない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.24をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED tests

- [x] no-result fixtureのvalid empty packetをREDで固定する。
- [x] positive fixture 8件、explicit paths、changed-onlyの非abstainを固定する。
- [x] trace上のcandidate reasonを調査し、共通する最小境界を記録する。

### Task 2: GREEN implementation

- [x] internal helperでmeaningful candidate reasonの有無だけを判定する。
- [x] 根拠なしの初回queryだけcandidate sliceを空にして既存compilerへ渡す。

### Task 3: Verification and gate

- [x] focused/full tests、vet、diff checkを通す。
- [x] history candidateを1回測定し、v0.24 baselineと比較する。
- [x] strict gateで採否を決め、findingsとcommit/revertを確定する。
- [ ] 完了後v0.26 emitted-long-excerpt planへ遷移する。

## Validation and Acceptance

positive/acceptable queryを一件も誤abstainせず、明示的irrelevant fixtureだけを空packetへ
変え、history品質と全contractを非回帰のまま総wireを削減すること。

## Idempotence and Recovery

判定は候補に既に付与済みの理由だけを使いpure/deterministicとする。不合格時は製品
候補だけを通常revertし、測定結果とfindingは保持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/app`のEvidence query経路と`internal/eval` contract
testsだけを対象とする。

## Progress

- [x] 2026-09-04: v0.24 acceptance後、v0.25へ遷移した。
- [x] 2026-09-04: no-result/positive/bypass REDとcandidate trace調査を完了した。
- [x] 2026-09-04: identifier-aware reason境界を実装し、focused/full検証を通過した。
- [x] 2026-09-04: history候補を1回測定し、target fixture gate合格で採用した。

## Surprises & Discoveries

- 存在しないsnake-case identifierが一般語へ分割され、FTS候補7件すべてにlexical
  reasonが付いたため、lexical有無だけではabstentionできなかった。
- history suiteには今回のabstention対象rowがなく、v0.24とbyte-identicalだった。
- 対象eval rowは230から85 tokensへ減り、delta ratioも0.5521958へ改善した。

## Decision Log

- 2026-09-04: v0.21の`expected` casesをacceptable、`expect_empty`をirrelevant labelとして
  使用し、新しい公開label schemaは追加しない。
- 2026-09-04: identifier queryではlexical/test/relationだけを十分条件にせず、identity
  またはpath/changed supportを要求する。explicit paths/changed-onlyはbypassする。
- 2026-09-04: historyは無変化だが、事前trace済み対象rowで145 tokens削減し全positive
  gateを通過したため、限定効果として採用する。

## Outcomes & Retrospective

v0.25はno-result Evidenceを1件から0件、wireを230から85へ削減し、8 positive cases、
late-hit、known expansion、全history rowを非回帰に保った。履歴suite自体のtoken値は
変わらないため、効果は根拠のないidentifier queryに限定される。次は送信実績のある
長いexcerptが存在するかをtrace-onlyで確認し、存在しなければ製品実装を行わない。
