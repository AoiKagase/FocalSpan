# FocalSpan v0.31 `focalspan.context.v2` design gate 計画

**Goal:** v1を既定のまま維持し、明示的capability negotiationでのみcompact encodingを
返せる設計が、現行MCP SDKと公開tool契約の範囲で安全に成立するかを判定する。

## Purpose / Big Picture

v1のfield名、location、relationをtableまたはdictionary化すればmodel-visible bytesを
大きく削減できる可能性がある。一方、これは近中期候補のv1互換方針外であり、暗黙の
schema切替は既存client、source fidelity、relation provenanceを破壊し得る。製品実装前に
negotiation経路と意味同値性の検証方法を確定し、前提が不足する場合はno-op findingとして
閉じる。

## Baseline and Gates

- accepted baseline: history wire 11,693、UTF-8 bytes 32,494、packet JSON 31,090、
  summary bytes 1,404、useful Evidence 5、効率0.4276。
- v1は既定かつbyte-identicalのまま維持する。
- v2はclientの明示的opt-in時だけ返す。
- v1/v2で意味、source fidelity、relation provenance、orderingが完全一致する検証方法を
  定義できなければ製品候補を作らない。
- 公開tool入力の追加やSDK initialize capability利用が必要な場合は、その互換性と
  discoverabilityをコードとSDK APIから実証する。

## Global Constraints

- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。
- network、external LLM、repository-code execution、package restoreを行わない。
- MCP stdoutをprotocol以外に使用しない。
- sourceはstructured contentに一度だけ含め、text summaryへ追加しない。
- field/encoding比較は開発用testまたはbenchmark traceだけに限定する。

## Plan of Work

- [x] v0.30をarchiveし、本PLANへ遷移する。
- [ ] documentation transition commitを作成する。
- [ ] 現行MCP SDK、tool registration、call input/output型、initialize capabilityの利用可能性を
  read-onlyで調査する。
- [ ] v1既定維持、v2明示opt-in、意味同値性を満たす最小設計を記録する。
- [ ] 前提が成立する場合だけ、次の独立milestoneで実装可能なacceptance fixtureを定義する。
- [ ] 前提が不足する場合は製品変更なしのdesign-blocked findingを作成する。
- [ ] 完了後、latency-only候補をtoken計画から分離して閉じる。

## Validation and Acceptance

このmilestoneは設計ゲートであり、製品encodingは実装しない。採用可能判定には、既存clientが
入力変更なしでv1を受け取ること、opt-inがtool schemaまたはprotocol capabilityとして明示・
発見可能であること、同一packetをv1/v2へencode/decodeして意味同値性を自動検証できることが
必要。いずれかを現行依存で保証できなければdesign-blockedとする。

文書のみ変更するため、検証はtargeted content inspection、`git diff --check`、明示pathの
status/diffに限定する。

## Idempotence and Recovery

read-only調査と文書化のみを行う。製品変更やbenchmarkは行わないためrevert候補は生じない。
途中再開時は本PLANのProgressと直近commitを照合する。

## Interfaces and Dependencies

調査対象は`internal/mcpserver`、`internal/evidence`、`internal/benchmark`、利用中MCP Go SDK。
公開MCP tools、CLI、`focalspan.context.v1`、`known_handles`は変更しない。

## Progress

- [x] 2026-09-04: v0.30 no-op closureをarchiveし、v0.31 design gateへ遷移した。

## Surprises & Discoveries

- 調査中。

## Decision Log

- 2026-09-04: v0.31はencoding実装ではなく、negotiationと意味同値性の成立性だけを判定する。

## Outcomes & Retrospective

進行中。
