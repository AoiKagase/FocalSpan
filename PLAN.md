# FocalSpan MCP利用ガイダンス 実装計画

## Purpose / Big Picture

MCP登録後の新しい接続で、FocalSpanがコード作業の早期ナビゲーション手段としてモデルに案内されるようにする。MCP標準のサーバーInstructionsを利用し、ツール呼び出しを強制せずにセッションごとの手入力を不要にする。

## Progress

- [x] 2026-09-04: 現行SDKのInstructions経路、MCP登録、dirty stateを確認した。
- [x] 2026-09-04: InstructionsのREDテストを追加し、現状で`instructions=""`の失敗を確認した。
- [x] 2026-09-04: サーバー初期化Instructionsを実装し、focused MCP tests 7件を通過した。
- [x] 2026-09-04: READMEと設計文書を更新した。
- [x] 2026-09-04: 全726テスト、vet、通常build、3 OS cross-build、diff checkを完了した。
- [ ] 新規Codexセッションで実際のツール選択を受入確認する。

## Surprises & Discoveries

- MCP登録はツールを接続可能にするだけで、モデルのツール選択を強制しない。
- 現行のgo-sdk v1.7.0はlegacy initializeとmodern discoverの両方でServerOptions.Instructionsを返す。
- Instructionsはサーバー実行時に返るため、登録ファイルの独自拡張は不要である。
- 既定Go build cacheはアクセス拒否になったため、検証では`.build/go-cache`を使用した。

## Decision Log

- 2026-09-04: 適用範囲はコード調査、変更、レビュー、デバッグとし、非コード作業では利用しない。
- 2026-09-04: `code_context` focusedを早期利用し、`code_expand`、`code_impact`、必要時の`code_status`を案内する。
- 2026-09-04: Instructionsは強い推奨に留め、ユーザー・上位指示と直接証拠を優先する。
- 2026-09-04: `AGENTS.md`、`.focalspan.json`、`TASKS.md`、グローバルCodex設定、フックは変更しない。
- 2026-09-04: Instructionsの自動選択はプロトコルテストと手動Codex UATを分け、後者を未確認のまま完了扱いにしない。

## Outcomes & Retrospective

実装と自動検証を完了し、既存のMCPツールとEvidence wireは非回帰である。手入力なしの新規Codexセッションでのcode_context早期選択は、現在の実装環境でFocalSpan MCPツールが公開されていないため未確認である。

## Context and Orientation

`internal/mcpserver/server.go` の `New` がSDKサーバーと5つのtoolを登録している。MCP SDKの `mcp.ServerOptions.Instructions` は initialize/discover応答の `instructions` フィールドへ転送される。既存のMCP登録は `command`、`args`、timeout、enabled_toolsだけを保持するため変更しない。

## Plan of Work

1. `internal/mcpserver`へ接続時Instructionsを検証するテストを、製品変更より先に追加する。FocalSpan、code_context、focused、code_expand、handles、code_impact、非コード作業の扱い、エラー時の継続方針を検証し、現状でREDを確認する。
2. `server.go`に不変のInstructions文を定義し、`mcp.NewServer`の`ServerOptions`へ設定する。サーバー名、バージョン、capability、5ツール、tool schema、Evidence出力は変更しない。
3. READMEと`docs/design.md`へInstructionsの目的、MCP登録との違い、更新済みバイナリへの差し替え、MCP再接続または新規セッションの必要性、モデル呼び出しを強制できないことを記録する。
4. REDテストをGREEN化し、既存MCP contract testsで5ツール、v1/v2 output、source fidelity、summary非漏洩を再確認する。
5. 実装検証後、dirtyなユーザーファイルを除く変更だけをレビューし、受入用の新規Codexセッションを確認する。

## Validation and Acceptance

- `go test ./internal/mcpserver -count=1`
- `go test ./...`
- `go vet ./...`
- `go build ./cmd/focalspan`
- `CGO_ENABLED=0`でWindows amd64、Linux amd64、Darwin arm64をcross-buildする。
- `git diff --check`を通す。
- 更新済みバイナリへMCP登録を向け、新規セッションでFocalSpanを明示せずコード質問を行い、広範な読み取りより前に`code_context`が呼ばれることを確認する。
- 非コード質問ではFocalSpanを呼ばないことを確認する。

## Idempotence and Recovery

Instructionsはサーバー起動時の定数で、セッション状態やDB状態を変更しない。既存登録はそのまま利用できる。旧バイナリや既存接続にはInstructionsがないため、バイナリ更新後にMCP再接続する。失敗時は製品変更と文書変更を分離して戻せる。

## Interfaces and Dependencies

- MCP wireのinitialize/discover応答に非空の`instructions`が追加される。
- MCP tool名、入力・出力schema、Evidence Packet v1/v2、登録形式、enabled_toolsは不変。
- 依存関係は既存の`github.com/modelcontextprotocol/go-sdk v1.7.0`のみで、追加依存はない。
