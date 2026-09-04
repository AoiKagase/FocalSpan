# FocalSpan v0.20 relation-linker comparator 実装計画

**Goal:** v0.19で未測定だったschema v1対schema v2のwall-clock比率を、同一規模・
同一内容のfixtureを同一processで直接測定し、`TASKS.md`の10x/5x/2x受入条件を
事実に基づいて閉じる。

**Architecture:** 製品linkerと公開interfaceは変更しない。`internal/linker`のopt-in
開発benchmarkに、v0.19直前の全relation・全symbol走査とrelationごとのwriteを再現する
test-only comparatorを置く。各scenarioは独立した同一fixtureでv1/v2を測定し、最終relation
rowsの完全一致を確認してから比率を判定する。

## Purpose / Big Picture

v0.19はunchanged `0s`、small `322.2249ms`、full `101.2515ms`で絶対時間gateを
満たしたが、v1 comparatorがなく10x/5x/2x比率は未検証だった。本マイルストーンは
token改善候補へ進む前にその測定負債だけを解消する。

## Baseline and Gates

- unchangedはv1比`>=10x`かつv2 `<=250ms`。
- small related changeはv1比`>=5x`かつv2 `<=1s`。
- full linkingはv1比`>=2x`かつv2 `<=5s`。
- 各scenarioのv1/v2 relation rowsはserialized value単位で完全一致する。
- v0.15 token baseline、MCP/CLI、Evidence、ranking、packingは変更しない。

## Global Constraints

- comparatorはopt-in test-onlyとし、通常`go test ./...`ではskipする。
- network、外部LLM、repository code実行、新runtime依存を導入しない。
- timing値はordinary unit testの合否へ混ぜない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Context and Orientation

- `internal/linker/schema_v2_benchmark_test.go`に現行scale fixtureとv2測定がある。
- v1 comparatorはv0.19直前の`Linker.Link`動作をtest-only helperとして再現する。
- `internal/store`の公開read/write APIだけを使い、製品codeへlegacy pathを戻さない。

## Plan of Work

### Task 0: ExecPlan transition

- [x] 完了済みv0.19 PLANをcompleted archiveへbyte-identicalに移動する。
- [ ] 本PLANだけをactive root PLANとしてdocumentation transition commitにする。

### Task 1: RED comparator tests

- [ ] v1/v2のrelation row不一致、gate未達、ゼロdurationを検出するtest helperを先に追加する。
- [ ] ratio計算をduration zeroでも決定的に扱う純粋helper testを追加する。

### Task 2: GREEN comparator implementation

- [ ] v1全走査・per-relation write comparatorをtest-onlyで実装する。
- [ ] unchanged、small、fullを独立した同一fixture pairで測定する。
- [ ] duration、ratio、candidate relation数、最終relation同値性を一つのsource-free logへ出す。

### Task 3: Verification and closure

- [ ] focused linker/store tests、`go test ./... -count=1`、`go vet ./...`を実行する。
- [ ] opt-in comparatorを1回実行し、全ratio・絶対時間gateを判定する。
- [ ] 結果を新しいfindingsへ記録し、v0.19 archiveは変更しない。
- [ ] comparatorとfindingsだけをatomic commitし、次のbenchmark-hardening planへ遷移する。

## Validation and Acceptance

v1/v2が同じfixtureから同じrelation rowsを生成し、unchanged/small/fullの全ratioと絶対
時間gateを満たすこと。未達ならv0.19を遡及的に成功扱いせず、未達値をそのまま記録する。

## Idempotence and Recovery

各scenarioは`t.TempDir()`配下の独立DBを作るため再実行可能で、checkoutや利用中indexを
変更しない。中断時は一時DBがtest cleanupで破棄される。

## Interfaces and Dependencies

公開interface変更なし。test-only comparatorは既存`store.Store`、`resolveCandidates`、
`projectmeta.Fact`だけに依存する。

## Progress

- [x] 2026-09-04: v0.19 archiveとv0.20 active planを作成した。

## Surprises & Discoveries

- 現行benchmarkはv2だけを測り、ratioを計算するv1 pathを持たない。

## Decision Log

- 2026-09-04: token候補より先にP0 comparatorを独立マイルストーンとして実施する。
- 2026-09-04: legacy linkerは製品codeへ戻さず、test-only comparatorへ限定する。

## Outcomes & Retrospective

実測後に更新する。
