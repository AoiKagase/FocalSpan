# FocalSpan v0.23 metadata pruning phase 2 実装計画

**Goal:** v0.14のfinal-packet-only pruningを限定的に拡張し、path拡張子から一意に
復元できるlanguageと、target/changeのsymbol identityから冗長なexact/qualified `why`
だけを省略して、Evidence selectionを変えずにmodel-visible wireを削減する。

## Purpose / Big Picture

v0.21では34,280 UTF-8 bytes中、Evidence contentは3,459 bytesに対し、残る
envelope/metadataは24,487 bytesだった。公開v1 schemaと必須identityを維持したまま、
既存`omitempty`で安全に除去できる重複値を削る。

## Baseline and Gates

- v0.15/v0.21: wire 12,304、bytes 34,280、useful 5、効率0.4064。
- 全rowのselected handle/role/fidelity/source/relationは不変。
- cumulative wireとUTF-8 bytesを両方厳密に削減し、comparison regression 0。
- ambiguous extension、mixed source、unknown languageは保持する。

## Global Constraints

- JSON key、schema、MCP/CLI、known handles、guidanceを変更しない。
- final-packet pruningだけを変更し、selection/rankingへ影響させない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.22をarchiveし、本PLANへ切り替える。
- [ ] documentation transition commitを作成する。

### Task 1: RED tests

- [ ] unambiguous extensionのlanguage省略とambiguous/mixed保持を固定する。
- [ ] target/changeのexact/qualified whyだけを省略し、relation whyを保持する。
- [ ] selection、wire、idempotence、MCP非露出の既存contractを拡張する。

### Task 2: GREEN implementation

- [ ] private extension-language allowlistを追加する。
- [ ] `prunePacketMetadata`で上記2規則だけを適用する。

### Task 3: Verification and gate

- [ ] focused/full tests、vet、diff checkを通す。
- [ ] history candidateを1回測定し、v0.15 qualityとv0.21 bytesを比較する。
- [ ] strict gateで採否を決め、findingsとcommit/revertを確定する。
- [ ] 完了後v0.24 summary planへ遷移する。

## Validation and Acceptance

公開意味を維持し、全row非回帰のままestimated wireとUTF-8 bytesがともに減ること。

## Idempotence and Recovery

pruningはpure/idempotentとする。不合格時は製品候補だけを通常revertする。

## Interfaces and Dependencies

公開interface変更なし。`internal/evidence/compiler.go`とtestsだけを製品対象とする。

## Progress

- [x] 2026-09-04: v0.22 negative closure後、v0.23へ遷移した。

## Surprises & Discoveries

実装中に更新する。

## Decision Log

- 2026-09-04: language allowlistは拡張子とparser languageが一意な形式だけに限定する。

## Outcomes & Retrospective

測定後に更新する。
