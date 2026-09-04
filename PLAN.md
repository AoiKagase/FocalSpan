# FocalSpan v0.32 negotiated compact context 実装計画

**Goal:** 明示的に`focalspan.context.v2`を受理するMCP clientだけへ、意味同値でv1より小さい
compact Evidence Packetを返し、既定v1 responseをbyte-identicalに維持する。

## Purpose / Big Picture

accepted baselineのmodel-visible bytesは32,494で、packet JSONが31,090を占める。v2では
repeated field名、location object、local relation IDを固定位置tableへ変換し、sourceや
provenanceを失わずmetadata bytesを削減する。v2が個別responseで小さくならない場合はv1へ
戻し、opt-in自体がwire回帰を生まないようにする。

## Baseline and Gates

- v1 history wire 11,693、UTF-8 bytes 32,494、useful 5、効率0.4276。
- v1 default structured contentとsummaryはbyte-identical。
- negotiated profileはcomparison regression 0、relation validity 1、forbidden violation 0、
  known resend 0、cumulative wire 11,693以下。
- negotiated profileのUTF-8 model-visible bytesを少なくとも1 rowかつ累積で厳密に削減する。
- hard budget、determinism、source fidelityを維持する。

## Global Constraints

- RED testを製品変更より先に作成する。
- candidate benchmarkは静的検証後に一度だけ実行する。
- `AGENTS.md`、`.focalspan.json`、`TASKS.md`を変更・stageしない。
- network、external LLM、repository-code execution、package restoreを行わない。
- MCP stdoutはprotocol only、sourceはstructured contentに一度だけとする。
- 不合格ならcandidate commit後に通常の`git revert`で製品変更だけを戻し、findingを保持する。

## Plan of Work

- [x] v0.31をarchiveし、本PLANへ遷移する。
- [ ] documentation transition commitを作成する。
- [ ] v2 codec、invalid table、determinism、canonical equivalenceのRED testsを作成する。
- [ ] capability discovery、v1 default、malformed fallback、3 tool negotiationのRED testsを作成する。
- [ ] compact codecとstrict decoderを実装する。
- [ ] server extension advertisement、request negotiation、per-response smaller-only fallbackを実装する。
- [ ] targeted tests、`go test ./...`、`go vet ./...`、cross-build、`git diff --check`を実行する。
- [ ] candidate commitを作成し、history benchmarkを一度だけ実行する。
- [ ] gate判定をfindingとPLANへ記録し、採用または通常revertする。
- [ ] 完了後、latency-only候補をtoken計画から分離して閉じる。

## Validation and Acceptance

REDでは既存v1 testを残したまま、`internal/evidence`へ全fidelityとrelation/guidanceを含む
round-trip fixtureを追加し、`internal/mcpserver`へcapability有無・malformed・discovery・3 tools
のintegration fixtureを追加する。GREEN後はlocal cacheを使い、targeted tests、全test、vet、
Windows amd64/Linux amd64/Darwin arm64の`CGO_ENABLED=0` cross-buildを行う。race testは既知の
local MinGW制約によりunavailableとして記録する。

benchmarkは既存history suiteにv2 capabilityを明示するprofile/runner経路を追加し、v1 baseline
と同一caseを比較する。測定は一度だけとし、結果で採否を確定する。

## Idempotence and Recovery

codecはpure conversion、negotiationはrequest-localでstateを持たない。benchmark生成物と
`.tmp-go-cache`はmilestone後に削除する。不合格時はexplicit candidate commitをrevertし、
測定文書だけを別commitで保持する。

## Interfaces and Dependencies

- 新schema: `focalspan.context.v2`（明示opt-inのみ）。
- MCP extension: `io.focalspan/context-encoding`。
- 既存tools、CLI、v1 schema、`known_handles`入力は不変。
- 実装対象: `internal/evidence`、`internal/mcpserver`、必要最小限の`internal/benchmark`。

## Progress

- [x] 2026-09-04: v0.31 design gateをarchiveし、v0.32実装milestoneへ遷移した。

## Surprises & Discoveries

- 調査・実装中。

## Decision Log

- 2026-09-04: `docs/benchmarks/findings-v0.31.md`のnegotiationとequivalence契約を採用する。

## Outcomes & Retrospective

進行中。
