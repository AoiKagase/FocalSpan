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

- [x] assembly語彙のpositive/negativeとexplicit path suppressionを固定する。
- [x] retrieverが`NewWithConfig`だけを上限8で返すことを固定する。
- [x] trace identityとdeterministic fusionを固定する。

### Task 2: GREEN implementation

- [x] private trigger helperとretriever IDを追加する。
- [x] 既存`SearchExactSymbols`を一回だけ利用するbounded retrieverを追加する。
- [x] abstention helperへprivate strong-support reasonを認識させる。

### Task 3: Verification and gate

- [x] focused/full tests、vet、diff checkを通す。
- [x] history candidateを1回測定しstrict gateで採否を決める。
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
- [x] 2026-09-04: RED/GREENとfocused/full静的検証を完了した。
- [x] 2026-09-04: candidate benchmarkを1回実行し、wire/packing gate不合格を確認した。
- [x] 2026-09-04: candidate `9ff32b2`を通常revert `c6878a6`で除去した。

## Surprises & Discoveries

- path-scope labelsは9から5へ減ったが、C++ 2 labelsはpacking-droppedへ移動し、Rust
  2 labelsだけがpackedになった。
- useful Evidenceは5から13へ増えた一方、wireは232、bytesは676増えた。

## Decision Log

- 2026-09-04: 共通原因は同じ期待path/symbolを持つC++/Rustの4 labelsに限定する。
- 2026-09-04: `packing_dropped 7→9`とwire 11,693→11,925が明示gate違反のため、
  efficiency上昇にかかわらず候補をreject/revertする。

## Outcomes & Retrospective

v0.27は狭いconstructor probeでもv0.11と同じ「retrieval前進がpacking悪化とwire増加へ
移る」問題を再現し、不採用となった。accepted baselineはv0.26/v0.25の11,693 wire、
32,494 bytes、useful 5、効率0.4276のまま。次はEvidence handlesを固定した内部fallback
だけでguidance削減またはfidelity昇格を試す。
