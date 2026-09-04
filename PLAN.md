# FocalSpan v0.21 token benchmark hardening 実装計画

**Goal:** token改善候補を実装する前に、現行history suiteが十分に測れていないpositive
initial hit、selected long excerpt、known-handle expansion、no-resultを決定的fixtureで覆い、
packetのfield・variant・guidance・summary別のUTF-8 byte寄与を開発専用reportへ追加する。

**Architecture:** 公開MCP/Evidence packetは変更しない。`internal/benchmark`のquality resultへ
estimator非依存のbyte metricsとprivate packet breakdownを追加し、`testdata/benchmark`に
合成acceptance repository/caseを追加する。通常MCP応答にdebug fieldを露出しない。

## Purpose / Big Picture

v0.15 history suiteは初回required recallがほぼゼロで、長いsourceや有効なdelta expansionを
十分に評価できない。Estimator係数変更による見かけ上の改善を防ぎ、次のpacking候補が実際に
影響するrowを実装前に判定できる測定基盤を作る。

## Baseline and Gates

- 既存v0.15の48 quality rowsと集計値はbyte-identicalに保持する。
- 新fixtureはpositive initial hit、long excerpt、known expansion、no-resultを各1件以上含む。
- packet bytesはcompact JSONとcanonical summaryのUTF-8 bytesを直接測る。
- breakdown合計はpacket bytesと一致し、通常MCP JSONには新fieldを出さない。

## Global Constraints

- benchmark/fixtureだけを変更し、retrieval、ranking、packing、Evidence compilerを変更しない。
- source本文、絶対path、user名、secretをreportへ出さない。
- network、外部LLM、repository code実行を行わない。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。

## Plan of Work

### Task 0: Transition

- [x] v0.20をcompleted archiveへ移動し、本PLANをactiveにする。
- [ ] documentation transition commitを作成する。

### Task 1: RED metrics and privacy tests

- [ ] UTF-8 packet bytes、JSON bytes、summary bytes、Evidence content bytes、metadata bytesの整合testを追加する。
- [ ] selected fidelity、guidance bytesをsource-free breakdownとして集計するtestを追加する。
- [ ] normal MCP responseにbreakdown/debug fieldが出ないtestを固定する。

### Task 2: GREEN metrics implementation

- [ ] `QualityResult`へstable aggregate byte fieldsを追加する。
- [ ] development-only detail reportへfidelity countとfield contributionを追加する。
- [ ] 既存Estimator metricsはbaseline比較互換のため維持する。

### Task 3: Coverage fixtures

- [ ] positive initial targetがfocused packetへ入るfixtureを追加する。
- [ ] 40行以上でlate hitを持ち、excerptが実際にselectedされるfixtureを追加する。
- [ ] valid anchorからknown_handles付きexpandが成功するfixtureを追加する。
- [ ] relevant candidateがなく空packetになるno-result fixtureを追加する。

### Task 4: Verification and closure

- [ ] focused benchmark/evidence/MCP tests、全体test、vet、diff checkを通す。
- [ ] fixture contract evaluationとprivacy scanを通す。
- [ ] findingsに新しいcoverageとbyte baselineを記録する。
- [ ] 合格後にatomic commitし、v0.22 bounded beam planへ遷移する。

## Validation and Acceptance

既存report互換性を壊さず、新fixtureが4つの盲点を実際に通ること。byte breakdownの総計が
model-visible payload bytesと一致し、debug情報が公開MCPへ出ないこと。

## Idempotence and Recovery

fixtureはtestdata内の固定sourceとlabelsだけを使用し、runごとにtemporary workspaceへ展開する。
不合格ならbenchmark変更だけを通常revertし、v0.15 baselineを維持する。

## Interfaces and Dependencies

公開interface変更なし。変更は`internal/benchmark`、`internal/benchcli`、`testdata/benchmark`
および開発用reportに限定する。

## Progress

- [x] 2026-09-04: v0.20完了後、v0.21 planへ遷移した。

## Surprises & Discoveries

- 現行wire metricはEstimator値のみで、Estimator変更から独立したserialized byte指標がない。

## Decision Log

- 2026-09-04: tokenizer oracleを導入せず、まずUTF-8 bytesをEstimator非依存の防止線とする。

## Outcomes & Retrospective

実装・測定後に更新する。
