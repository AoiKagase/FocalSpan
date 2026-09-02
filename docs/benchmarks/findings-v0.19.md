# schema v2 relation linking v0.19 findings

## Scope and execution

v0.19は、derived SQLite indexへschema v2 lookup projectionを追加し、relation
candidate selection、batch linking、atomic replacementを更新性能の範囲で実装した。
公開MCP tool、CLIの公開引数、`focalspan.context.v1`、Evidence source fidelity、
ranking、packing、`known_handles`は変更していない。schema v1は通常open/status/search/
MCP startupで変更せず、setup/update/明示auto-updateだけがtemporary sibling DBを構築し、
検証後にswapする。

## Static and semantic verification

- schema safety、projection equivalence、scoped linking、batch rollback、atomic
  replacement、progress timingのRED/GREEN testsが成功した。
- store/linker/indexer/app/cli/extractor targeted testsと全体`go test ./... -count=1`が成功した。
- `go vet ./...`、gofmt、`git diff --check`が成功した。`go test -race ./...`は
  ローカルMinGWの`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`により
  **UNVERIFIED**（終了コードにかかわらず診断を優先）とした。
- Windows amd64 native/CGO-free、Linux amd64 CGO-free、Darwin arm64 CGO-free buildが成功した。
- relation rows、unresolved/ambiguous semantics、lookup projection、normal v1 diagnostic、
  future schema拒否、temporary rebuild、atomic swap、公開出力の回帰は検出されなかった。

## Current-scale benchmark gate

専用fixture（450 files、5,000 symbols、28,000 relations、21,676 unresolved relations）を
直近のPython module lookup修正後に1回だけ測定した。

| Metric | Measured | Gate | Result |
|---|---:|---:|---|
| unchanged update linking | `0s` | `<=250ms` | pass |
| small related file change | `322.2249ms` | `<=1s` | pass |
| full relation linking | `101.2515ms` | `<=5s` | pass |
| relation candidate scope | projection superset + resolver equivalence | no hidden candidates | pass |
| batch writes | one transaction for successful links | one transaction | pass |

この専用benchmarkは候補実装とv1のwall-clockを同一runで比較するcomparatorを持たない。
そのためTASKS.mdの10x/5x/2x比率は独立測定せず、絶対時間gate、構造的candidate比較、
relation完全一致を採用判断の根拠とした。比率を厳密に受入条件へ戻す場合は、v1 comparatorを
別の開発用benchmarkとして追加する。

## Compatibility and privacy

公開JSON/MCPのschema、Evidence wire、source provenance、deterministic ordering、
CLI stdout protocolに変更はない。findingsと生成artifactをscanし、絶対ローカルパス、
ユーザー名由来パス、秘密情報、private-key markerは検出されなかった。

生成物はhash記録後に削除した。

| Artifact | SHA-256 |
|---|---|
| current-scale benchmark log | `41ef6c1583ad2dc9f7035b8096e3c6d2ad01556cf023a5e018f36dd1f0225eb` |
| Windows amd64 CGO-free build | `474bd19d41741da42df1099f7061a6142400c383639a6878ae055ba4f4a4cf1f` |
| Windows amd64 native build | `b1aeab3bde8558405b60c1bbb9d6d1a2906533c89fbc8d3a5543f44c8c6ee2a7` |
| Linux amd64 CGO-free build | `eae2e0bddb32c0f2bc5b657d603667bfb92fce326d6d9dcc76ca01cc04fba0` |
| Darwin arm64 CGO-free build | `51dcc628496c17843d44ef401f5b4be3d4fe7fa582b3467e22dbfb1a5d326942` |

## Outcome and retrospective

schema v2 relation linkingは、同値性・非破壊upgrade・rollback・cross-build・絶対時間gateを
満たしたため採用候補として確定する。full linkingのprojection miss fallbackを除去したことが
最大の性能寄与であり、Python qualified moduleのraw-case判定が互換性上の重要な修正だった。
v1対v2のwall-clock比率を直接測るcomparatorが未実装な点は残課題であり、次の性能候補で
独立benchmarkとして扱う。
