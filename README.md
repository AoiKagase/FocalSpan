# FocalSpan

FocalSpan is a token-first code context compiler. It turns a repository, a
natural-language question, Git state, and a token budget into a small,
deterministic bundle of ranked source spans with paths, line ranges, reasons,
stable handles, and omission information.

FocalSpan is an independent implementation. It is not Code Review Graph (CRG),
does not copy CRG code or schemas, and does not claim CRG compatibility.

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

Go files use syntax-only standard-library AST extraction. Other supported
profiles use conservative structural extraction for C-like, Python-like,
Markdown, and fallback text files. Calls that cannot be resolved are retained
as lexical relations with confidence rather than being asserted as facts.

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

The checked-in fixture is `testdata/repos/authsample`; cases are in
`testdata/eval/cases.jsonl`. Evaluation reports hit@1/3/5, symbol/path recall,
forbidden-path violations, budget compliance, median estimate, reduction ratio,
and repeated-run determinism.

```text
go test ./...
go test -race ./...
go vet ./...
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
```

Project rules and package boundaries are in `AGENTS.md`; design, roadmap, and
evaluation interpretation are in `docs/design.md` and `docs/evaluation.md`.

## Future extension points

The design leaves room for SCIP, Tree-sitter, C/C++/C#/Rust/Python/TypeScript
semantic providers, model-specific tokenizers, MMR or learned reranking,
streamable HTTP, a filesystem watcher, and optional remote embeddings. These
are roadmap items, not incomplete production stubs.
