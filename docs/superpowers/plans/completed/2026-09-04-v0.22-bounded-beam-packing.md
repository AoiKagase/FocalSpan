# FocalSpan v0.22 bounded beam Evidence packing 実装計画

**Goal:** 現行greedy utility-per-wire selectionをcontrolとして保持し、幅8・最大深度6の
決定的beam探索が、同じ候補数以下かつ完成wire以下でより高いutilityのEvidence組合せを
選べる場合だけ採用する。

## Purpose / Big Picture

v0.12はpacking dropsを7から4へ改善したが、全anchor予約でwireを12,740から27,667へ
増やした。v0.22は予約を行わず、既存variantと完成packet測定を用いたbounded combination
searchに限定し、request単位のlegacy ceilingでwire悪化を構造的に防ぐ。

## Baseline and Gates

- controlは現行v0.21 greedy output。
- beam width 8、depthは`selectionLimit`上限6、anchor規則とtie orderingを維持する。
- beam採用時はselected count `<=control`、final wire `<=control`、utility `>control`。
- historical focused/2048でpacking droppedを3件以上改善し、packed labelsを維持する。
- cumulative estimated wire `<=12,304`、UTF-8 wire bytes `<=34,280`、efficiency `>0.4064`。

## Global Constraints

- retrieval、ranking、variant生成、guidance規則、公開MCP/CLI/v1 schemaを変更しない。
- source fidelity、relation provenance、budget、determinism、known handlesを維持する。
- 通常responseにbeam/control/debug fieldを出さない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.21をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED tests

- [x] greedy局所解より高utilityの組合せをbeamが選ぶtestを追加する。
- [x] beamがwire、count、utility条件を超える場合controlへ戻るtestを追加する。
- [x] tie orderingとMCP非露出を既存determinism/contract testsで固定する。

### Task 2: GREEN implementation

- [x] 現行greedy selectionをprivate control helperへ抽出する。
- [x] 幅8のbeam state展開、完成packet評価、deterministic pruningを追加する。
- [x] control/beamを同じguidance・metadata finalizationへ通し、strict ceilingで選択する。

### Task 3: Verification and gate

- [x] candidateのfocused Evidence/App/MCP testsを通す。全体test/vetはgate不合格後の追加実行を避け、post-revert closureで実行する。
- [x] history candidate benchmarkを1回実行し、v0.21 byte baselineとv0.15 qualityを比較する。
- [x] gate不合格のため製品変更を通常revertし、negative findingsを保持する。
- [x] gate不合格をnegative findingとしてcommitし、v0.23 metadata planへ遷移する。

## Validation and Acceptance

request単位の完成wire/count ceiling、全contract invariant、historical gateを同時に満たすこと。
packing改善が3件未満なら、unit testが通っても不採用とする。

## Idempotence and Recovery

beamは固定幅、固定順序、pure in-memory stateで、同一入力から同一packetを生成する。不合格時は
製品commitだけをrevertし、v0.21 controlと測定所見を維持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/evidence/compiler.go`と同package testsだけを製品対象とする。

## Progress

- [x] 2026-09-04: v0.21完了後、v0.22 planへ遷移した。
- [x] 2026-09-04: RED/GREENとfocused tests後、history 48-row candidateを1回測定した。
- [x] 2026-09-04: packing/quality/wire/bytesがcontrolと同値でgate不合格となり、`016fe5f`を`22813d6`でrevertした。
- [x] 2026-09-04: post-revert `go test ./... -count=1`、`go vet ./...`、diff/privacy checksがpassした。

## Surprises & Discoveries

- strict control wire ceilingは回帰を防いだが、history suiteで採用可能なbeam stateを1件も作らなかった。

## Decision Log

- 2026-09-04: exact-cost導入ではなく、既存完成packet測定を使うbounded combination searchとする。

## Outcomes & Retrospective

candidateは`packing_dropped=7`、packed=1、useful Evidence=5、wire=12,304、
UTF-8 bytes=34,280でcontrolと完全同値だった。3件改善と効率向上を満たさないため棄却し、
v0.21 product baselineへ復帰した。
