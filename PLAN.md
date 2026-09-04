# FocalSpan v0.24 MCP text summary 最小化計画

**Goal:** Evidence packetと重複するMCP text summaryを固定短形式へ縮約し、件数、
budget使用量・上限、omitted件数を維持したままmodel-visible wireを削減する。

## Purpose / Big Picture

v0.23時点でsummaryは累積2,324 UTF-8 bytesを占め、structuredContentに既に存在する
説明語を繰り返している。`Summary`の数値情報とsource-free contractは維持し、機械的で
短い一形式だけを返す。

## Baseline and Gates

- v0.23: wire 11,983、bytes 33,414、summary bytes 2,324、useful 5、効率0.4173。
- 全rowのpacket JSONとselected handle/role/fidelity/source/relationは不変。
- 全rowのwire非増加、cumulative wire・UTF-8 bytes・summary bytesを厳密に削減する。
- comparison regression 0、全MCP client/test互換、source/query非露出を維持する。

## Global Constraints

- structuredContent、JSON key、schema、MCP/CLI、known handlesを変更しない。
- `evidence.Summary`だけを変更し、Evidence compilerとbudget selectionへ影響させない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.23をarchiveし、本PLANへ切り替える。
- [x] documentation transition commitを作成する。

### Task 1: RED tests

- [ ] summaryの固定短形式と4つの数値をexact testで固定する。
- [ ] source、symbol、queryがsummaryへ出ないcontractを維持する。
- [ ] MCP context/expand/impactが同じstructuredContentと短形式textを返すことを固定する。

### Task 2: GREEN implementation

- [ ] `internal/evidence/wire.go`の`Summary`だけを固定短形式へ変更する。
- [ ] status/restart summaryとpublic packet schemaは変更しない。

### Task 3: Verification and gate

- [ ] focused/full tests、vet、diff checkを通す。
- [ ] history candidateを1回測定し、v0.23 baselineと比較する。
- [ ] strict gateで採否を決め、findingsとcommit/revertを確定する。
- [ ] 完了後v0.25 relevance-aware abstention planへ遷移する。

## Validation and Acceptance

summaryの数値情報とMCP互換性を維持し、packet byte-identicalのまま全row非回帰、
cumulative estimated wire・UTF-8 bytes・summary bytesがすべて減ること。

## Idempotence and Recovery

summary生成はpure/deterministicとする。不合格時は製品候補だけを通常revertし、
測定結果とfindingは保持する。

## Interfaces and Dependencies

公開interface変更なし。`internal/evidence/wire.go`とそのunit/MCP contract testsだけを
製品対象とする。

## Progress

- [x] 2026-09-04: v0.23 acceptance後、v0.24へ遷移した。

## Surprises & Discoveries

実装中に更新する。

## Decision Log

- 2026-09-04: `FocalSpan evidence:`等の説明語は削減対象とするが、items、used/limit、
  omittedの4数値はclient可観測情報として残す。

## Outcomes & Retrospective

測定後に更新する。
