# FocalSpan v0.27 structural constructor micro-retriever 計画

**Goal:** registry assembly/registration queryに限定してGoの`NewWithConfig` constructorを
exact検索し、同一原因のpath/symbol labelsを3件以上改善する。

## Purpose / Big Picture

focused/2048のpath-scope miss 9件中、C++ registry assemblyとRust service registrationは
同じ`internal/app/service.go::NewWithConfig`を期待し、path/symbol合計4 labelsを占める。
v0.11の広いpath/identity bridgeは再利用せず、明示的assembly語彙から一つのconstructor
symbolだけをprobeする。

## Baseline and Gates

- v0.26/v0.25: wire 11,693、bytes 32,494、useful 5、効率0.4276。
- 同一原因4 labels中3件以上をpath/symbol/packedへ前進させる。
- packing_dropped、project-metadata、全row wireを非回帰とする。
- cumulative wire `<=11,693`、効率`>0.4276`、全共通contractを維持する。

## Global Constraints

- 公開schema、MCP/CLI、known handles、ranking weight、packingを変更しない。
- triggerはregistry/assembled/wired/registered/登録などassembly語彙へ限定する。
- probe symbolは`NewWithConfig`一つ、候補上限8、決定順序を維持する。
- v0.11のpath bridge、lexical seed、store schemaは再利用しない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.26をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED tests

- [ ] assembly語彙のpositive/negativeとexplicit path suppressionを固定する。
- [ ] retrieverが`NewWithConfig`だけを上限8で返すことを固定する。
- [ ] trace identityとdeterministic fusionを固定する。

### Task 2: GREEN implementation

- [ ] private trigger helperとretriever IDを追加する。
- [ ] 既存`SearchExactSymbols`を一回だけ利用するbounded retrieverを追加する。
- [ ] abstention helperへprivate strong-support reasonを認識させる。

### Task 3: Verification and gate

- [ ] focused/full tests、vet、diff checkを通す。
- [ ] history candidateを1回測定しstrict gateで採否を決める。
- [ ] 完了後v0.28 guidance/fidelity joint-budget planへ遷移する。

## Validation and Acceptance

同一原因4 labels中3件以上を前進し、packing/wire/project-metadataと全contractを非回帰に
する。不合格なら製品候補だけを通常revertする。

## Idempotence and Recovery

trigger/probeはstatelessで再実行可能とする。不合格時は通常revertし、negative findingと
測定結果を保持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/search`のprivate retrieverと`internal/app` abstention
supportだけを対象とする。

## Progress

- [x] 2026-09-04: v0.26 no-op closure後、v0.27へ遷移した。

## Surprises & Discoveries

実装中に更新する。

## Decision Log

- 2026-09-04: 共通原因は同じ期待path/symbolを持つC++/Rustの4 labelsに限定する。

## Outcomes & Retrospective

測定後に更新する。
