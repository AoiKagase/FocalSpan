# FocalSpan

FocalSpan is a token-first code context compiler. It turns a repository, a
natural-language question, Git state, and a token budget into a small,
deterministic bundle of ranked source spans with paths, line ranges, reasons,
stable handles, and omission information.

## 日本語

FocalSpanは、トークン数を起点にコードのコンテキストを組み立てるコンパイラです。リポジトリ、自然言語の質問、Gitの状態、トークン予算を入力として、関連度順に並べたソース範囲を、パス・行範囲・選定理由・安定したハンドル・省略情報付きの小さなバンドルとして出力します。

### ビルドと対応プラットフォーム

必要なものはGo 1.26以降です。コアバイナリは`modernc.org/sqlite`を使用するため、`CGO_ENABLED=0`でWindows amd64、Linux amd64、macOS arm64向けにビルドできます。PythonやNode.jsのランタイムは不要で、ネットワーク通信も行いません。

```text
go build ./cmd/focalspan
CGO_ENABLED=0 go build ./cmd/focalspan
```

生成される実行ファイルは、Windowsでは`focalspan.exe`、その他の環境では`focalspan`です。

### クイックスタート

リポジトリ内で次を実行します。

```text
focalspan init
focalspan index
focalspan query --query "where is an expired authentication token rejected?" --budget 1200
focalspan status --json
```

`query`は、インデックスが存在しないか古い場合、既定で差分更新を行います。読み取り専用にする場合は`--no-update`を指定してください。`--root`を指定すると対象ディレクトリを明示的に固定できます。インデックス内のパスはリポジトリ相対かつスラッシュ区切りで保存されます。

よく使うコマンドは次のとおりです。

```text
focalspan update --if-repo --quiet
focalspan query --query "what calls ValidateToken?" --mode outline
focalspan expand --handle chunk_... --relation callers --budget 1200
focalspan impact --budget 2000 --json
focalspan eval --cases testdata/eval/cases.jsonl --json
focalspan doctor --json
focalspan serve --root C:\src\example-project
```

### Codexへの自動登録

リポジトリ内で次を実行すると、既定のproject scopeへFocalSpan MCPを登録できます。

```text
focalspan mcp install codex
focalspan mcp status codex
```

project scopeは既定値で、`<root>/.codex/config.toml`へFocalSpan管理ブロックだけを追加します。Codexはtrusted projectのproject-local設定だけをロードします。rootごとに登録内容が分離され、既存のCodex sessionでは設定の再読込または新しいsessionが必要になる場合があります。

user scopeを使う場合は、公式Codex CLIを経由して次を実行します。

```text
focalspan mcp install codex --scope user
```

user scopeではroot固有のserver名を使い、全projectから見えるユーザー設定へ登録するためCodex CLIが必要です。通常はproject scopeを推奨します。登録前に確認するには次を使えます。

```text
focalspan mcp print codex
focalspan mcp install codex --dry-run
```

削除は次のコマンドです。

```text
focalspan mcp uninstall codex
```

例えばWindowsでパスに空白があっても、rootを指定するだけでshell quotingは不要です。

```text
focalspan mcp install codex --root "C:\Work Spaces\BookStack"
```

user scopeのCodex CLIのoptionや出力はversionによって異なるため、FocalSpanはproject scopeで未確認のproject-scope flagを使用しません。必要に応じて`codex mcp --help`でインストール済みCLIを確認してください。

`impact`は`--base`/`--head`を省略すると、ステージ前後の変更を使用します。関係解析は構文ベースであり、解決できない呼び出しが省略される可能性を明示します。Gitリポジトリ外で`update --if-repo --quiet`を実行しても、正常終了して何も出力しません。

### 設定とインデックス

`focalspan init`は、既存のファイルを上書きせずに`.focalspan.json`を作成し、`.focalspan/`を作成して`.gitignore`へ追加します。既存設定を置き換える意図がある場合だけ`init --force`を使用してください。

インデックスは`.focalspan/index.db`に保存されます。ソース内容が保持されるため、保持してよい場所と権限を選んでください。CLIオプションは設定ファイルより優先され、トークン予算は256〜64000に収められます。

### 検索と開示範囲

Goファイルは標準ライブラリのASTを使った構文解析を行います。PHPは第一級の構文抽出として、namespace、use、class/interface/trait/enum、function/method、property/constant、include/require、PHPUnitテストを扱います。`.inc`は内容依存でPHPタグがある場合だけPHPとして検出し、`.phtml`などの混在HTML/PHPも検索できます。完全な型推論、動的ディスパッチ、サービスコンテナ解決は行わず、ComposerやPHPランタイムも実行しません。その他の対応プロファイルでは、C系、Python系、Markdown、フォールバックのテキストファイルを保守的に構造化抽出します。解決できない呼び出しは、事実と断定せず信頼度付きの字句関係として保持します。

`outline`はメタデータとシグネチャを返し、`source`は制限されたソース本体を追加します。各項目には`expand`で利用できる安定したハンドルがあり、`self`、`parent`、`children`、`callers`、`callees`、`imports`、`references`、`tests`、`neighbors`の関係を辿れます。ヘッダーとメタデータのコストも含めてから結果を詰めるため、最終的なシリアライズ済み推定値は指定予算内に収まります。

### MCP stdioサーバー

Codexでは次のTOML設定でFocalSpanを登録できます。

```toml
[mcp_servers.focalspan]
command = "C:\\Tools\\focalspan.exe"
args = ["serve", "--root", "C:\\src\\example-project"]
startup_timeout_sec = 30
tool_timeout_sec = 60
enabled_tools = ["code_context", "code_expand", "code_impact", "code_status"]
```

サーバーが公開するツールは`code_context`、`code_expand`、`code_impact`、`code_status`だけです。標準出力はMCPプロトコル専用で、ログは標準エラー出力へ送られます。サーバーは起動時のルートに束縛され、ツール入力から任意の絶対パスを受け付けません。

### セキュリティとプライバシー

FocalSpanはネットワーク通信、外部LLMの呼び出し、リポジトリコードの実行、ビルド・テスト・パッケージマネージャーの実行を行いません。シンボリックリンクによる脱出、パストラバーサル、バイナリ、無効なUTF-8、サイズ超過ファイル、選択ルート外のファイルは拒否されます。

既定では`.env`、`.env.*`、`*.pem`、`*.key`、`id_rsa`、`id_ed25519`、`credentials.json`、`secrets.json`など、秘密情報を含みそうなパスを除外します。明示的な`include`パターンで再度含めることはできます。インデックスにはソースが保存されるため、`.focalspan/`の権限と保持期間を確認してください。

### 制限とトラブルシューティング

MVPではWeb UI、HTTP MCPトランスポート、埋め込み、ベクトル検索、ウォッチャー、Tree-sitter、SCIPインポート、ビルド／テストの意味解析、完全な多言語呼び出し解決には対応していません。汎用抽出は意図的に近似的で、`impact`も構文ベースです。

データベースが壊れた場合や未対応スキーマの場合は、対象の`.focalspan/index.db`を削除して`focalspan index`を再実行してください。`focalspan doctor --json`では、ルート検出、Git、SQLite/FTS5、設定、MCP、権限、更新状態を確認できます。結果がない場合はインデックスを作成し、secret/excludeパターンを確認したうえで、`--mode outline --json`を試してください。

### 評価と開発

チェックイン済みのフィクスチャは`testdata/repos/authsample`と`testdata/repos/phpsample`、評価ケースはそれぞれ`testdata/eval/cases.jsonl`と`testdata/eval/php-cases.jsonl`です。評価ではhit@1/3/5、シンボル／パス再現率、禁止パス違反、予算遵守、推定値の中央値、削減率、繰り返し実行時の決定性を確認します。PHP fixtureでは、namespace/use、クラス・メソッド、PHPUnitテスト、`.inc` include、混在HTML/PHPの検索を確認します。

```text
go test ./...
go test -race ./...
go vet ./...
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
```

プロジェクト規則とパッケージ境界は`AGENTS.md`、設計・ロードマップ・評価結果の解釈は`docs/design.md`と`docs/evaluation.md`に記載しています。

## MVP architecture

```text
repository -> scanner -> extractor -> SQLite/FTS5 -> retrieval -> ranking
           -> token packer -> compact CLI output or MCP structured output
```

The index stores source content locally in `.focalspan/index.db`. Do not place
the index in a location where that content should not be retained.

## Build and supported platforms

Requirements: Go 1.26 or newer. The core binary uses `modernc.org/sqlite`, so
it builds with `CGO_ENABLED=0` for Windows amd64, Linux amd64, and macOS arm64.

```text
go build ./cmd/focalspan
CGO_ENABLED=0 go build ./cmd/focalspan
```

The resulting executable is `focalspan` (or `focalspan.exe` on Windows). It
uses no Python or Node.js runtime and makes no network requests.

## Quick start

Run these commands from a repository:

```text
focalspan init
focalspan index
focalspan query --query "where is an expired authentication token rejected?" --budget 1200
focalspan status --json
```

`query` performs an incremental update by default when the index is stale or
missing. Use `--no-update` to make it read-only. `--root` binds a command to
the explicitly supplied directory; paths in the index remain repository-
relative and slash-normalized.

Useful commands:

```text
focalspan update --if-repo --quiet
focalspan query --query "what calls ValidateToken?" --mode outline
focalspan expand --handle chunk_... --relation callers --budget 1200
focalspan impact --budget 2000 --json
focalspan eval --cases testdata/eval/cases.jsonl --json
focalspan doctor --json
focalspan serve --root C:\src\example-project
```

`impact` uses unstaged and staged changes when `--base`/`--head` are omitted.
Its relationship analysis is syntax-only and explicitly reports that
unresolved calls may be omitted. `update --if-repo --quiet` exits successfully
and prints nothing when run outside Git, which makes it suitable for a hook.

### Automatic Codex registration

Run these commands from a repository to register the FocalSpan MCP server in
the project-local Codex configuration:

```text
focalspan mcp install codex
focalspan mcp status codex
```

Project scope is the default and adds only a FocalSpan-managed block to
`<root>/.codex/config.toml`. Codex loads project-local configuration only for
trusted projects. Each root has a separate registration, and an existing Codex
session may need to reload configuration or use a new session.

User scope delegates registration to the official Codex CLI:

```text
focalspan mcp install codex --scope user
```

It uses a root-specific server name and is visible from every project, so the
Codex CLI is required and project scope is generally preferred. Preview or
remove a registration with:

```text
focalspan mcp print codex
focalspan mcp install codex --dry-run
focalspan mcp uninstall codex
```

On Windows, a path containing spaces needs no shell quoting when passed as a
root value to FocalSpan:

```text
focalspan mcp install codex --root "C:\Work Spaces\BookStack"
```

Codex CLI options and output can differ by installed version. FocalSpan does
not assume an unverified project-scope flag; use `codex mcp --help` to inspect
the installed CLI when troubleshooting.

## Configuration

`focalspan init` creates `.focalspan.json` without overwriting an existing
file, creates `.focalspan/`, and adds `.focalspan/` to `.gitignore`. Use
`init --force` only when replacing an existing configuration is intended.

Default configuration:

```json
{
  "index_directory": ".focalspan",
  "default_token_budget": 4000,
  "max_file_bytes": 2097152,
  "workers": 0,
  "auto_update_before_query": true,
  "include": [],
  "exclude": [],
  "secret_excludes_enabled": true,
  "generic_chunk_lines": 80,
  "generic_chunk_overlap": 10,
  "max_candidates": 200
}
```

Unknown keys produce warnings; invalid types and out-of-range values are
errors. Command-line options take precedence over configuration. Token budgets
are clamped to 256..64000.

## Search and disclosure

Go files use syntax-only standard-library AST extraction. PHP has a first-class
structural extractor for namespaces, imports, class-like declarations,
functions/methods, properties/constants, include/require, and PHPUnit tests.
`.inc` is detected as PHP only when its content contains a PHP opening tag;
mixed HTML/PHP such as `.phtml` remains searchable. Complete type inference,
dynamic dispatch, and service-container resolution are not provided, and
Composer or the PHP runtime is never executed. Other supported profiles use
conservative structural extraction for C-like, Python-like, Markdown, and
fallback text files. Calls that cannot be resolved are retained as lexical
relations with confidence rather than being asserted as facts.

`outline` returns metadata and signatures. `source` adds bounded source body.
Every item has a stable handle suitable for `expand`; supported relations are
`self`, `parent`, `children`, `callers`, `callees`, `imports`,
`references`, `tests`, and `neighbors`. Results are deterministic and are
packed after the header/metadata cost is included, so the final serialized
estimate remains within the requested budget.

## MCP stdio server

Configure FocalSpan in Codex using the current TOML shape:

```toml
[mcp_servers.focalspan]
command = "C:\\Tools\\focalspan.exe"
args = ["serve", "--root", "C:\\src\\example-project"]
startup_timeout_sec = 30
tool_timeout_sec = 60
enabled_tools = ["code_context", "code_expand", "code_impact", "code_status"]
```

On Linux or macOS:

```toml
[mcp_servers.focalspan]
command = "/usr/local/bin/focalspan"
args = ["serve", "--root", "/src/example-project"]
startup_timeout_sec = 30
tool_timeout_sec = 60
enabled_tools = ["code_context", "code_expand", "code_impact", "code_status"]
```

The server exposes exactly `code_context`, `code_expand`, `code_impact`, and
`code_status`. stdout is reserved for MCP protocol messages; logs go to
stderr. The server is bound to its startup root and does not accept arbitrary
absolute paths from tool input.

A hook can update an index without producing normal output:

```json
{
  "command": "focalspan update --if-repo --quiet"
}
```

## Security and privacy

FocalSpan does not contact a network, invoke an external LLM, execute
repository code, run a build/test/package manager, or use a shell command to
analyze source. Git is called with separated arguments only for repository
listing and diff metadata. Symlink escapes, path traversal, binary files,
invalid UTF-8, oversized files, and files outside the selected root are
rejected.

Secret-shaped paths are skipped by default: `.env`, `.env.*`, `*.pem`,
`*.key`, `id_rsa`, `id_ed25519`, `credentials.json`, and `secrets.json`.
An explicit `include` pattern can opt a path back in. Review `.focalspan/`
permissions and retention because indexed source is present in the SQLite DB.

## Limitations and troubleshooting

The MVP does not provide a Web UI, HTTP MCP transport, embeddings, vector
search, watcher, Tree-sitter, SCIP import, build/test semantic analysis, or
complete multi-language call resolution. Generic extraction is intentionally
approximate, and `impact` is syntax-only.

If the database is corrupt or its schema is unsupported, remove the selected
`.focalspan/index.db` and run `focalspan index` again. Use `focalspan doctor
--json` to check root detection, Git, SQLite/FTS5, configuration, MCP setup,
permissions, and freshness. If no results appear, run `focalspan index`, check
secret/exclude patterns, and retry with `--mode outline --json`.

## Evaluation and development

The checked-in fixtures are `testdata/repos/authsample` and
`testdata/repos/phpsample`; cases are in `testdata/eval/cases.jsonl` and
`testdata/eval/php-cases.jsonl`. Evaluation reports hit@1/3/5, symbol/path recall,
forbidden-path violations, budget compliance, median estimate, reduction ratio,
and repeated-run determinism.

```text
go test ./...
go test -race ./...
go vet ./...
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
```

Project rules and package boundaries are in `AGENTS.md`; design, roadmap, and
evaluation interpretation are in `docs/design.md` and `docs/evaluation.md`.

## Future extension points

The design leaves room for SCIP, Tree-sitter, C/C++/C#/Rust/Python/TypeScript
semantic providers, model-specific tokenizers, MMR or learned reranking,
streamable HTTP, a filesystem watcher, and optional remote embeddings. These
are roadmap items, not incomplete production stubs.
